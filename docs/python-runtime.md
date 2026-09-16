# Python runtime

Package: `mica`. Concurrency: asyncio. Handlers run on the application event loop.

```python
from mica import App
from jobs.v1.jobs_pb2 import JobCompleted
from mica_tokens import Worker

app = App("recorder")

@app.subscribe(JobCompleted)
async def on_completed(event: JobCompleted) -> None:
    ...

@app.serve(Worker.Run)
async def run_job(request):
    ...

response = await app.call(Worker.Run, request, timeout=1.0)
app.run(main)
```

`App(name)` reads `MICA_NATS_URL`, `MICA_COMPONENT_NAME`, and `MICA_SURFACE_FILE` when present. `MICA_SURFACE_FILE` enables contract surface reporting (see `docs/runtime-semantics.md`).

Raise `mica.RpcError(RpcCode.INVALID_ARGUMENT, "reason")` from a handler to return a structured RPC status. Other exceptions become `INTERNAL` with message `internal error`.

`call` raises `CallTimeout` when the caller deadline expires and `CallCancelled` when the caller task is cancelled. Those are local outcomes, not wire status.

Event handler exceptions are logged; the process keeps running.

`run(main)` installs SIGINT/SIGTERM handlers, starts the app, runs `main`, then shuts down. Drain uses nats-py `drain()`.

Default RPC timeout: 5 seconds. No retries.

`mica generate` writes RPC method tokens and generated messages for the workspace. Import generated messages as `jobs.v1.jobs_pb2`, not `contracts.jobs.v1`, and import workspace tokens from `mica_tokens`. The protobuf package is the wire identity.
