# Go runtime

Handlers run in nats.go goroutines. `Call` uses `context.Context` for deadline and local cancellation.

```go
app := mica.NewApp("client")

app.Subscribe((*auditv1.JobRecorded)(nil),
    func(ctx context.Context, event proto.Message) error {
        return nil
    })

app.Serve(tokens.WorkerRun,
    func(ctx context.Context, req proto.Message) (proto.Message, error) {
        return &jobsv1.RunResponse{Accepted: true}, nil
    })

resp, err := app.Call(ctx, tokens.WorkerRun, request)
```

Return `mica.NewRpcError(code, message)` for a structured status. Other errors become `INTERNAL`.

`Call` returns `CallTimeout` when the caller deadline expires and `CallCancelled` when the caller context is cancelled. Those are local outcomes, not wire status. They unwrap to `context.DeadlineExceeded` / `context.Canceled`.

Event handler errors are logged.

`Run(ctx)` starts the app and waits for SIGINT/SIGTERM or `ctx` cancellation, then shuts down.

`NewApp` reads `MICA_NATS_URL`, `MICA_COMPONENT_NAME`, and `MICA_SURFACE_FILE` from the environment. `MICA_SURFACE_FILE` enables contract surface reporting (see `docs/runtime-semantics.md`).

Default RPC timeout: 5 seconds unless the context deadline is sooner. No retries.

V0 does not install a custom goroutine scheduler.
