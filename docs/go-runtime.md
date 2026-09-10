# Go runtime

Handlers run in nats.go goroutines. `Call` uses `context.Context` for deadline and local cancellation.

```go
app := mica.NewApp("planner")

app.Subscribe((*trackingv1.PersonTracked)(nil),
    func(ctx context.Context, event proto.Message) error {
        return nil
    })

app.Serve(tokens.CameraControlSetPose,
    func(ctx context.Context, req proto.Message) (proto.Message, error) {
        return &camerav1.SetPoseResponse{Accepted: true}, nil
    })

resp, err := app.Call(ctx, tokens.CameraControlSetPose, request)
```

Return `mica.NewRpcError(code, message)` for a structured status. Other errors become `INTERNAL`.

Event handler errors are logged.

`Run(ctx)` starts the app and waits for SIGINT/SIGTERM or `ctx` cancellation, then shuts down.

Default RPC timeout: 5 seconds unless the context deadline is sooner. No retries.

V0 does not install a custom goroutine scheduler.
