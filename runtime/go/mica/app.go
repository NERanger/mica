package mica

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	micav1 "mica/generated/go/mica/v1"
)

type EventHandler func(context.Context, proto.Message) error
type RPCHandler func(context.Context, proto.Message) (proto.Message, error)

type App struct {
	cfg    AppConfig
	nc     *nats.Conn
	mu     sync.Mutex
	state  State
	start  time.Time
	subs   []*nats.Subscription
	events []queuedEvent
	rpcs   []queuedRPC
}

type queuedEvent struct {
	sample  proto.Message
	handler EventHandler
}

type queuedRPC struct {
	method  RPCMethod
	handler RPCHandler
}

func NewApp(name string, cfg ...AppConfig) *App {
	config := ConfigFromEnv(name)
	if len(cfg) > 0 {
		config = cfg[0]
		if config.Name == "" {
			config.Name = name
		}
		if config.TransportURL == "" {
			config.TransportURL = DefaultTransportURL
		}
		if config.RPCTimeout == 0 {
			config.RPCTimeout = DefaultRPCTimeout
		}
	}
	return &App{cfg: config, state: StateStopped}
}

func (a *App) Subscribe(sample proto.Message, handler EventHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, queuedEvent{sample: sample, handler: handler})
}

func (a *App) Serve(method RPCMethod, handler RPCHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rpcs = append(a.rpcs, queuedRPC{method: method, handler: handler})
}

func (a *App) Start() error {
	a.mu.Lock()
	if a.state == StateRunning {
		a.mu.Unlock()
		return nil
	}
	a.state = StateStarting
	a.mu.Unlock()
	nc, err := nats.Connect(a.cfg.TransportURL, nats.Timeout(2*time.Second), nats.RetryOnFailedConnect(false))
	if err != nil {
		a.setState(StateFailed)
		return &TransportError{Message: "failed to connect to transport at " + a.cfg.TransportURL, Err: err}
	}
	a.mu.Lock()
	a.nc = nc
	events := append([]queuedEvent(nil), a.events...)
	rpcs := append([]queuedRPC(nil), a.rpcs...)
	a.mu.Unlock()
	for _, item := range events {
		if err := a.bindEvent(item.sample, item.handler); err != nil {
			a.setState(StateFailed)
			return err
		}
	}
	for _, item := range rpcs {
		if err := a.bindRPC(item.method, item.handler); err != nil {
			a.setState(StateFailed)
			return err
		}
	}
	a.mu.Lock()
	a.start = time.Now()
	a.state = StateRunning
	a.mu.Unlock()
	return nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.Start(); err != nil {
		return err
	}
	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-runCtx.Done()
	return a.Shutdown()
}

func (a *App) Shutdown() error {
	a.mu.Lock()
	if a.state == StateStopped || a.state == StateStopping {
		a.state = StateStopped
		a.mu.Unlock()
		return nil
	}
	a.state = StateStopping
	nc := a.nc
	subs := a.subs
	a.mu.Unlock()
	for _, sub := range subs {
		_ = sub.Drain()
	}
	if nc != nil {
		_ = nc.Drain()
	}
	a.mu.Lock()
	a.nc = nil
	a.subs = nil
	a.state = StateStopped
	a.mu.Unlock()
	return nil
}

func (a *App) Publish(_ context.Context, event proto.Message) error {
	nc, err := a.requireRunning()
	if err != nil {
		return err
	}
	contractID := string(event.ProtoReflect().Descriptor().FullName())
	payload, err := proto.Marshal(event)
	if err != nil {
		return &ProtocolError{Message: "failed to serialize event"}
	}
	body, err := marshalEnvelope(a.cfg.Name, contractID, payload, RpcCodeOK, "", false)
	if err != nil {
		return err
	}
	return nc.Publish(EventSubject(contractID), body)
}

func (a *App) Call(ctx context.Context, method RPCMethod, request proto.Message) (proto.Message, error) {
	nc, err := a.requireRunning()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, NewRpcError(RpcCodeCancelled, "rpc cancelled")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.RPCTimeout)
		defer cancel()
	}
	payload, err := proto.Marshal(request)
	if err != nil {
		return nil, &ProtocolError{Message: "failed to serialize request"}
	}
	body, err := marshalEnvelope(a.cfg.Name, method.ContractID(), payload, RpcCodeOK, "", false)
	if err != nil {
		return nil, err
	}
	msg, err := nc.RequestWithContext(ctx, RPCSubject(method.Service, method.Method), body)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, NewRpcError(RpcCodeCancelled, "rpc cancelled")
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
			return nil, NewRpcError(RpcCodeTimeout, "rpc timed out")
		}
		if errors.Is(err, nats.ErrNoResponders) {
			return nil, NewRpcError(RpcCodeUnavailable, "no rpc provider")
		}
		return nil, &TransportError{Message: "rpc transport failure", Err: err}
	}
	env, err := unmarshalEnvelope(msg.Data)
	if err != nil {
		return nil, err
	}
	if env.Status != nil {
		code := RpcCode(env.Status.Code)
		if code != RpcCodeUnspecified && code != RpcCodeOK {
			return nil, NewRpcError(code, env.Status.Message)
		}
	}
	response := method.NewResponse()
	if err := proto.Unmarshal(env.Payload, response); err != nil {
		return nil, &ProtocolError{Message: "malformed rpc response payload"}
	}
	return response, nil
}

func (a *App) Health() Health {
	a.mu.Lock()
	defer a.mu.Unlock()
	h := Health{State: a.state}
	if a.state == StateRunning {
		h.UptimeMS = uint64(time.Since(a.start).Milliseconds())
	}
	return h
}

func (a *App) requireRunning() (*nats.Conn, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.state != StateRunning || a.nc == nil {
		return nil, &TransportError{Message: "app is not running"}
	}
	return a.nc, nil
}

func (a *App) setState(state State) {
	a.mu.Lock()
	a.state = state
	a.mu.Unlock()
}

func (a *App) bindEvent(sample proto.Message, handler EventHandler) error {
	contractID := string(sample.ProtoReflect().Descriptor().FullName())
	sub, err := a.nc.Subscribe(EventSubject(contractID), func(msg *nats.Msg) {
		env, err := unmarshalEnvelope(msg.Data)
		if err != nil {
			log.Printf("mica: dropping malformed event: %v", err)
			return
		}
		event := sample.ProtoReflect().New().Interface()
		if err := proto.Unmarshal(env.Payload, event); err != nil {
			log.Printf("mica: dropping malformed event payload")
			return
		}
		if err := handler(context.Background(), event); err != nil {
			log.Printf("mica: event handler failed for %s: %v", contractID, err)
		}
	})
	if err != nil {
		return &TransportError{Message: "subscribe failed", Err: err}
	}
	a.mu.Lock()
	a.subs = append(a.subs, sub)
	a.mu.Unlock()
	return nil
}

func (a *App) bindRPC(method RPCMethod, handler RPCHandler) error {
	sub, err := a.nc.Subscribe(RPCSubject(method.Service, method.Method), func(msg *nats.Msg) {
		code := RpcCodeOK
		message := ""
		var payload []byte
		env, err := unmarshalEnvelope(msg.Data)
		if err != nil {
			code = RpcCodeInvalidArgument
			message = "malformed request"
		} else {
			request := method.NewRequest()
			if err := proto.Unmarshal(env.Payload, request); err != nil {
				code = RpcCodeInvalidArgument
				message = "malformed request"
			} else {
				resp, herr := handler(context.Background(), request)
				if herr != nil {
					var rpcErr *RpcError
					if errors.As(herr, &rpcErr) {
						code = rpcErr.Code
						message = rpcErr.Message
					} else {
						log.Printf("mica: rpc handler failed for %s: %v", method.ContractID(), herr)
						code = RpcCodeInternal
						message = "internal error"
					}
				} else if resp != nil {
					payload, _ = proto.Marshal(resp)
				}
			}
		}
		if msg.Reply == "" {
			return
		}
		body, err := marshalEnvelope(a.cfg.Name, method.ContractID(), payload, code, message, true)
		if err != nil {
			return
		}
		_ = a.nc.Publish(msg.Reply, body)
	})
	if err != nil {
		return &TransportError{Message: "serve failed", Err: err}
	}
	a.mu.Lock()
	a.subs = append(a.subs, sub)
	a.mu.Unlock()
	return nil
}

func marshalEnvelope(sender, contractID string, payload []byte, code RpcCode, message string, withStatus bool) ([]byte, error) {
	env := &micav1.Envelope{
		ProtocolVersion: ProtocolVersion,
		MessageId:       time.Now().Format("20060102150405.000000000"),
		SenderId:        sender,
		TimestampNs:     uint64(time.Now().UnixNano()),
		ContractId:      contractID,
		Payload:         payload,
	}
	if withStatus {
		env.Status = &micav1.RpcStatus{
			Code:    micav1.RpcCode(code),
			Message: message,
		}
	}
	out, err := proto.Marshal(env)
	if err != nil {
		return nil, &ProtocolError{Message: "failed to serialize envelope"}
	}
	return out, nil
}

func unmarshalEnvelope(data []byte) (*micav1.Envelope, error) {
	env := &micav1.Envelope{}
	if err := proto.Unmarshal(data, env); err != nil {
		return nil, &ProtocolError{Message: "malformed envelope"}
	}
	if env.ProtocolVersion != ProtocolVersion {
		return nil, &ProtocolError{Message: "unsupported mica protocol version"}
	}
	return env, nil
}
