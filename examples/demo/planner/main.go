package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	camerav1 "mica/examples/demo/generated/go/camera/v1"
	"mica/examples/demo/generated/go/mica/tokens"
	trackingv1 "mica/examples/demo/generated/go/tracking/v1"
	"mica/runtime/go/mica"
)

const maxCalls = 3

func main() {
	app := mica.NewApp("planner")
	var mu sync.Mutex
	calls := 0

	callSetPose := func(pan float32) {
		mu.Lock()
		if calls >= maxCalls {
			mu.Unlock()
			return
		}
		calls++
		n := calls
		mu.Unlock()
		req := &camerav1.SetPoseRequest{
			CameraId: &camerav1.CameraId{Value: "cam-1"},
			Pose:     &camerav1.Pose{Pan: pan, Tilt: 5, Zoom: 1},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		resp, err := app.Call(ctx, tokens.CameraControlSetPose, req)
		if err != nil {
			log.Printf("SetPose failed: %v", err)
			return
		}
		out := resp.(*camerav1.SetPoseResponse)
		fmt.Printf("SetPose RPC completed accepted=%v iteration=%d\n", out.Accepted, n)
	}

	app.Subscribe((*trackingv1.PersonTracked)(nil), func(ctx context.Context, msg proto.Message) error {
		event := msg.(*trackingv1.PersonTracked)
		fmt.Printf("received PersonTracked x=%.1f y=%.1f\n", event.X, event.Y)
		callSetPose(event.X + 8)
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	callSetPose(0)
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
