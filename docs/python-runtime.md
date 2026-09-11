# Python runtime

Package: `mica`. Concurrency: asyncio. Handlers run on the application event loop.

```python
from mica import App
from camera.v1.camera_pb2 import PoseChanged
from mica_tokens import CameraControl

app = App("tracker")

@app.subscribe(PoseChanged)
async def on_pose(event: PoseChanged) -> None:
    ...

@app.serve(CameraControl.SetPose)
async def set_pose(request):
    ...

response = await app.call(CameraControl.SetPose, request, timeout=1.0)
app.run(main)
```

`App(name)` reads `MICA_NATS_URL` and `MICA_COMPONENT_NAME` when present.

Raise `mica.RpcError(RpcCode.INVALID_ARGUMENT, "reason")` from a handler to return a structured RPC status. Other exceptions become `INTERNAL` with message `internal error`.

Event handler exceptions are logged; the process keeps running.

`run(main)` installs SIGINT/SIGTERM handlers, starts the app, runs `main`, then shuts down. Drain uses nats-py `drain()`.

Default RPC timeout: 5 seconds. No retries.

`mica generate` writes RPC method tokens and generated messages for the workspace. Import generated messages as `camera.v1.camera_pb2`, not `contracts.camera.v1`, and import workspace tokens from `mica_tokens`. The protobuf package is the wire identity.
