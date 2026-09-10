# Developer quick start

Get a working MICA checkout, run the three-language demo, and know what to edit next.

> Processes communicate through versioned contracts. They do not depend on each other's implementation.
>
> NATS is the V0 transport, not the application API. Do not write NATS subject strings in application code.

## Prerequisites

Install these yourself:

| Tool | Version |
| --- | --- |
| Python | 3.12+ |
| C++ compiler + CMake | C++20, CMake 3.20+ |
| Go | 1.22+ |
| `protoc` | 3.21+ (system protobuf is fine) |
| `curl` | any recent |

`./scripts/bootstrap` then installs into `.tools/bin`:

- `buf`
- `nats-server`
- `protoc-gen-go`

Repo scripts prepend `.tools/bin` to `PATH`. After bootstrap, `pip install -e .` provides the `mica` command.

## First run

From the repository root:

```
./scripts/bootstrap
./scripts/build
./scripts/test
mica validate examples/demo/app.toml
mica inspect examples/demo/app.toml
mica graph examples/demo/app.toml
./scripts/run-demo
```

If `mica` is not on `PATH`:

```
python3 -m mica.cli validate examples/demo/app.toml
```

`./scripts/run-demo` builds if needed, starts a local `nats-server`, and runs `examples/demo/app.toml`. Stop with Ctrl+C.

## What the demo does

Three processes, one bounded loop (three `SetPose` calls):

```
Go planner
  -- RPC camera.v1.CameraControl.SetPose -->
C++ camera-control
  -- event camera.v1.PoseChanged -->
Python tracker
  -- event tracking.v1.PersonTracked -->
Go planner
```

You should see log lines like:

```
[camera-control] started
[tracker] started
[planner] started
[planner] SetPose RPC completed accepted=true iteration=1
[tracker] received PoseChanged ...
[tracker] published PersonTracked
[planner] received PersonTracked ...
... iteration=3 ...
[camera-control] stopped
[tracker] stopped
[planner] stopped
```

A `transport error: nats: unexpected EOF` on shutdown is the Python client noticing NATS going away. It is not a demo failure.

Without `--start-nats`, `mica run` expects an external server at the URL in `app.toml` (default `nats://127.0.0.1:4222`).

## Daily loop

| Change | Then |
| --- | --- |
| `contracts/*.proto` | `./scripts/generate` (never edit `generated/`) |
| Python / C++ / Go runtime or demo | `./scripts/build` |
| Behavior or contracts | `./scripts/test` |

Default verification is `./scripts/test`. Integration tests need `nats-server`.

Buf breaking checks use `contracts/baseline.binpb`. Update that file only when you intentionally freeze a new contract snapshot.

## Where things live

| Path | Role |
| --- | --- |
| `contracts/` | Protobuf source of truth |
| `generated/` | Buf output; gitignored; do not edit |
| `runtime/python` | `from mica import App` |
| `runtime/cpp` | `mica::App` |
| `runtime/go` | `mica.NewApp` |
| `cli/` | `mica validate\|inspect\|graph\|run` |
| `examples/demo/` | camera-control (C++), tracker (Python), planner (Go) |

Language APIs: `docs/python-runtime.md`, `docs/cpp-runtime.md`, `docs/go-runtime.md`.  
Manifests: `docs/application-manifest.md`.  
Wire semantics: `docs/runtime-semantics.md`.

## Minimal process

```python
from mica import App
from camera.v1.camera_pb2 import PoseChanged

app = App("tracker")

@app.subscribe(PoseChanged)
async def on_pose(event: PoseChanged) -> None:
    print(event)

async def main():
    ...

app.run(main)
```

`App("tracker")` reads `MICA_NATS_URL` and `MICA_COMPONENT_NAME` when the launcher injects them.

Declare the same contracts in that process's `component.toml`. `mica validate` checks identifiers against `generated/image.binpb`.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| `buf: command not found` | `./scripts/bootstrap`; use `./scripts/*` so `.tools/bin` is on `PATH` |
| `missing generated/image.binpb` | `./scripts/generate` |
| `failed to connect to transport` | start NATS (`--start-nats` or `nats-server`) |
| `mica: command not found` | `./scripts/bootstrap` or `python3 -m mica.cli` |
| C++ link / missing `nats.h` | `./scripts/build` from a clean tree after bootstrap |
| Demo RPC never completes | another process already bound to port 4222; stop leftover `nats-server` |
