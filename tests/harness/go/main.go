package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	auditv1 "mica/examples/demo/generated/go/audit/v1"
	jobsv1 "mica/examples/demo/generated/go/jobs/v1"
	"mica/examples/demo/generated/go/mica/tokens"
	"mica/runtime/go/mica"
)

const warmup = 100

type loadReport struct {
	Role              string `json:"role"`
	Received          int    `json:"received"`
	Expected          int    `json:"expected"`
	Sent              int    `json:"sent"`
	Ok                int    `json:"ok"`
	Errors            int    `json:"errors"`
	Timeouts          int    `json:"timeouts"`
	Unavailable       int    `json:"unavailable"`
	P50Ns             uint64 `json:"p50_ns"`
	P99Ns             uint64 `json:"p99_ns"`
	MaxNs             uint64 `json:"max_ns"`
	PercentileSamples int    `json:"percentile_samples"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mica-go-harness <command>")
		os.Exit(2)
	}
	cmd := os.Args[1]
	if cmd == "proto-roundtrip" {
		if len(os.Args) < 4 {
			os.Exit(2)
		}
		in, err := os.ReadFile(os.Args[2])
		if err != nil {
			panic(err)
		}
		event := &jobsv1.JobCompleted{}
		if err := proto.Unmarshal(in, event); err != nil {
			fmt.Fprintln(os.Stderr, "parse failed")
			os.Exit(1)
		}
		out, err := proto.Marshal(event)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(os.Args[3], out, 0o644); err != nil {
			panic(err)
		}
		return
	}

	app := mica.NewApp("go-harness")
	ctx := context.Background()
	switch cmd {
	case "publish-completed":
		if err := app.Start(); err != nil {
			panic(err)
		}
		event := &jobsv1.JobCompleted{
			JobId:       &jobsv1.JobId{Value: "job-1"},
			Output:      "done:task-1",
			TimestampNs: 42,
		}
		if err := app.Publish(ctx, event); err != nil {
			panic(err)
		}
		time.Sleep(200 * time.Millisecond)
		_ = app.Shutdown()
	case "subscribe-completed":
		got := make(chan struct{}, 1)
		app.Subscribe((*jobsv1.JobCompleted)(nil), func(ctx context.Context, msg proto.Message) error {
			event := msg.(*jobsv1.JobCompleted)
			fmt.Printf("got output=%v\n", event.Output)
			got <- struct{}{}
			return nil
		})
		if err := app.Start(); err != nil {
			panic(err)
		}
		select {
		case <-got:
			_ = app.Shutdown()
		case <-time.After(5 * time.Second):
			os.Exit(1)
		}
	case "subscribe-recorded":
		got := make(chan struct{}, 1)
		app.Subscribe((*auditv1.JobRecorded)(nil), func(ctx context.Context, msg proto.Message) error {
			event := msg.(*auditv1.JobRecorded)
			fmt.Printf("got job=%v\n", event.JobId)
			got <- struct{}{}
			return nil
		})
		if err := app.Start(); err != nil {
			panic(err)
		}
		select {
		case <-got:
			_ = app.Shutdown()
		case <-time.After(5 * time.Second):
			os.Exit(1)
		}
	case "serve-run":
		behavior := "ok"
		if len(os.Args) > 2 {
			behavior = os.Args[2]
		}
		app.Serve(tokens.WorkerRun, func(ctx context.Context, msg proto.Message) (proto.Message, error) {
			switch behavior {
			case "invalid":
				return nil, mica.NewRpcError(mica.RpcCodeInvalidArgument, "bad job")
			case "internal":
				return nil, fmt.Errorf("boom")
			case "sleep":
				time.Sleep(3 * time.Second)
			}
			return &jobsv1.RunResponse{Accepted: true}, nil
		})
		if err := app.Run(context.Background()); err != nil {
			panic(err)
		}
	case "call-run":
		if err := app.Start(); err != nil {
			panic(err)
		}
		req := &jobsv1.RunRequest{
			JobId: &jobsv1.JobId{Value: "job-1"},
			Input: "task-1",
		}
		callCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := app.Call(callCtx, tokens.WorkerRun, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Printf("accepted=%v\n", resp.(*jobsv1.RunResponse).Accepted)
		_ = app.Shutdown()
	case "publish-load":
		count, rate := mustPositive(os.Args, 4)
		publishLoad(app, count, rate)
	case "subscribe-load":
		if len(os.Args) < 4 {
			os.Exit(2)
		}
		expect := mustAtoi(os.Args[2])
		timeoutS, err := strconv.ParseFloat(os.Args[3], 64)
		if err != nil || timeoutS <= 0 || expect <= 0 {
			os.Exit(2)
		}
		subscribeLoad(app, expect, timeoutS)
	case "call-load":
		if len(os.Args) < 5 {
			os.Exit(2)
		}
		count := mustAtoi(os.Args[2])
		rate := mustAtoi(os.Args[3])
		concurrency := mustAtoi(os.Args[4])
		if concurrency <= 0 {
			os.Exit(2)
		}
		callLoad(app, count, rate, concurrency)
	case "serve-pipeline":
		servePipeline(app)
	default:
		os.Exit(2)
	}
}

func mustAtoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		os.Exit(2)
	}
	return n
}

func mustPositive(args []string, need int) (int, int) {
	if len(args) < need {
		os.Exit(2)
	}
	return mustAtoi(args[2]), mustAtoi(args[3])
}

func usedSamples(samples []uint64) []uint64 {
	if len(samples) > warmup {
		out := make([]uint64, len(samples)-warmup)
		copy(out, samples[warmup:])
		return out
	}
	out := make([]uint64, len(samples))
	copy(out, samples)
	return out
}

func percentileNs(samples []uint64, p float64) uint64 {
	used := usedSamples(samples)
	if len(used) == 0 {
		return 0
	}
	sort.Slice(used, func(i, j int) bool { return used[i] < used[j] })
	k := int(math.Ceil(p/100*float64(len(used)))) - 1
	if k < 0 {
		k = 0
	}
	if k >= len(used) {
		k = len(used) - 1
	}
	return used[k]
}

func maxNs(samples []uint64) uint64 {
	used := usedSamples(samples)
	var max uint64
	for _, v := range used {
		if v > max {
			max = v
		}
	}
	return max
}

func printLoad(report loadReport) {
	body, err := json.Marshal(report)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MICA_LOAD %s\n", body)
}

func reportFromSamples(role string, received, expected, sent, ok, errs, timeouts, unavailable int, samples []uint64) loadReport {
	return loadReport{
		Role:              role,
		Received:          received,
		Expected:          expected,
		Sent:              sent,
		Ok:                ok,
		Errors:            errs,
		Timeouts:          timeouts,
		Unavailable:       unavailable,
		P50Ns:             percentileNs(samples, 50),
		P99Ns:             percentileNs(samples, 99),
		MaxNs:             maxNs(samples),
		PercentileSamples: len(usedSamples(samples)),
	}
}

func publishLoad(app *mica.App, count, rate int) {
	if err := app.Start(); err != nil {
		panic(err)
	}
	ctx := context.Background()
	interval := time.Second / time.Duration(rate)
	next := time.Now()
	sent := 0
	errs := 0
	for i := 0; i < count; i++ {
		event := &jobsv1.JobCompleted{
			JobId:       &jobsv1.JobId{Value: strconv.Itoa(i)},
			Output:      "done:" + strconv.Itoa(i),
			TimestampNs: uint64(time.Now().UnixNano()),
		}
		if err := app.Publish(ctx, event); err != nil {
			errs++
		} else {
			sent++
		}
		next = next.Add(interval)
		time.Sleep(time.Until(next))
	}
	time.Sleep(200 * time.Millisecond)
	printLoad(reportFromSamples("publish", 0, 0, sent, sent, errs, 0, 0, nil))
	_ = app.Shutdown()
}

func subscribeLoad(app *mica.App, expect int, timeoutS float64) {
	var mu sync.Mutex
	samples := make([]uint64, 0, expect)
	done := make(chan struct{})
	var once sync.Once
	app.Subscribe((*jobsv1.JobCompleted)(nil), func(ctx context.Context, msg proto.Message) error {
		event := msg.(*jobsv1.JobCompleted)
		now := uint64(time.Now().UnixNano())
		ts := event.TimestampNs
		var lat uint64
		if now >= ts {
			lat = now - ts
		}
		mu.Lock()
		samples = append(samples, lat)
		n := len(samples)
		mu.Unlock()
		if n >= expect {
			once.Do(func() { close(done) })
		}
		return nil
	})
	if err := app.Start(); err != nil {
		panic(err)
	}
	select {
	case <-done:
	case <-time.After(time.Duration(timeoutS * float64(time.Second))):
	}
	mu.Lock()
	copySamples := append([]uint64(nil), samples...)
	mu.Unlock()
	printLoad(reportFromSamples("subscribe", len(copySamples), expect, 0, 0, 0, 0, 0, copySamples))
	_ = app.Shutdown()
}

func callLoad(app *mica.App, count, rate, concurrency int) {
	if err := app.Start(); err != nil {
		panic(err)
	}
	var mu sync.Mutex
	samples := make([]uint64, 0, count)
	var ok atomic.Int64
	var errs atomic.Int64
	var timeouts atomic.Int64
	var unavailable atomic.Int64
	interval := time.Second / time.Duration(rate)
	origin := time.Now()
	var next atomic.Int64
	var wg sync.WaitGroup
	for t := 0; t < concurrency; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= count {
					return
				}
				time.Sleep(time.Until(origin.Add(interval * time.Duration(i))))
				req := &jobsv1.RunRequest{
					JobId: &jobsv1.JobId{Value: strconv.Itoa(i)},
					Input: "task",
				}
				callCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				t0 := time.Now()
				_, err := app.Call(callCtx, tokens.WorkerRun, req)
				lat := time.Since(t0)
				cancel()
				if err != nil {
					errs.Add(1)
					var rpcErr *mica.RpcError
					if errors.As(err, &rpcErr) {
						switch rpcErr.Code {
						case mica.RpcCodeTimeout:
							timeouts.Add(1)
						case mica.RpcCodeUnavailable:
							unavailable.Add(1)
						}
					}
					continue
				}
				ok.Add(1)
				mu.Lock()
				samples = append(samples, uint64(lat.Nanoseconds()))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	copySamples := append([]uint64(nil), samples...)
	mu.Unlock()
	okN := int(ok.Load())
	printLoad(reportFromSamples("call", okN, count, count, okN, int(errs.Load()), int(timeouts.Load()),
		int(unavailable.Load()), copySamples))
	_ = app.Shutdown()
}

func servePipeline(app *mica.App) {
	app.Serve(tokens.WorkerRun, func(ctx context.Context, msg proto.Message) (proto.Message, error) {
		req := msg.(*jobsv1.RunRequest)
		event := &jobsv1.JobCompleted{
			JobId:       req.JobId,
			Output:      "done:" + req.Input,
			TimestampNs: uint64(time.Now().UnixNano()),
		}
		if err := app.Publish(context.Background(), event); err != nil {
			return nil, err
		}
		return &jobsv1.RunResponse{Accepted: true}, nil
	})
	if err := app.Run(context.Background()); err != nil {
		panic(err)
	}
}
