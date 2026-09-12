package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/protobuf/proto"

	auditv1 "mica/examples/demo/generated/go/audit/v1"
	"mica/examples/demo/generated/go/mica/tokens"
	jobsv1 "mica/examples/demo/generated/go/jobs/v1"
	"mica/runtime/go/mica"
)

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
	default:
		os.Exit(2)
	}
}
