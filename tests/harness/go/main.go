package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"google.golang.org/protobuf/proto"

	camerav1 "mica/generated/go/camera/v1"
	"mica/generated/go/mica/tokens"
	trackingv1 "mica/generated/go/tracking/v1"
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
		event := &camerav1.PoseChanged{}
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
	case "publish-pose":
		if err := app.Start(); err != nil {
			panic(err)
		}
		event := &camerav1.PoseChanged{
			CameraId:    &camerav1.CameraId{Value: "cam-1"},
			Pose:        &camerav1.Pose{Pan: 1.5, Tilt: 2.5, Zoom: 3.5},
			TimestampNs: 42,
		}
		if err := app.Publish(ctx, event); err != nil {
			panic(err)
		}
		time.Sleep(200 * time.Millisecond)
		_ = app.Shutdown()
	case "subscribe-pose":
		got := make(chan struct{}, 1)
		app.Subscribe((*camerav1.PoseChanged)(nil), func(ctx context.Context, msg proto.Message) error {
			event := msg.(*camerav1.PoseChanged)
			fmt.Printf("got pan=%v\n", event.Pose.Pan)
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
	case "subscribe-person":
		got := make(chan struct{}, 1)
		app.Subscribe((*trackingv1.PersonTracked)(nil), func(ctx context.Context, msg proto.Message) error {
			event := msg.(*trackingv1.PersonTracked)
			fmt.Printf("got x=%v\n", event.X)
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
	case "serve-setpose":
		behavior := "ok"
		if len(os.Args) > 2 {
			behavior = os.Args[2]
		}
		app.Serve(tokens.CameraControlSetPose, func(ctx context.Context, msg proto.Message) (proto.Message, error) {
			switch behavior {
			case "invalid":
				return nil, mica.NewRpcError(mica.RpcCodeInvalidArgument, "bad pose")
			case "internal":
				return nil, fmt.Errorf("boom")
			case "sleep":
				time.Sleep(3 * time.Second)
			}
			return &camerav1.SetPoseResponse{Accepted: true}, nil
		})
		if err := app.Run(context.Background()); err != nil {
			panic(err)
		}
	case "call-setpose":
		if err := app.Start(); err != nil {
			panic(err)
		}
		req := &camerav1.SetPoseRequest{
			CameraId: &camerav1.CameraId{Value: "cam-1"},
			Pose:     &camerav1.Pose{Pan: 1},
		}
		callCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		resp, err := app.Call(callCtx, tokens.CameraControlSetPose, req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Printf("accepted=%v\n", resp.(*camerav1.SetPoseResponse).Accepted)
		_ = app.Shutdown()
	default:
		os.Exit(2)
	}
}
