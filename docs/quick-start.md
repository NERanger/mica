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
- `makeself`
- the `mica` CLI

Repo scripts prepend `.tools/bin` to `PATH`. After bootstrap, `mica` is that CLI. The Python runtime is installed editable as the `mica` package.

## First run

From the repository root:

```
./scripts/bootstrap
./scripts/build
./scripts/test
mica graph examples/demo/app.toml
./scripts/run-demo
```

`./scripts/run-demo` builds, starts a local `nats-server`, and runs `mica deploy examples/demo/app.toml --local --start-nats`. Stop with Ctrl+C.

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
[planner] SetPose RPC completed accepted=true iteration=1
[tracker] received PoseChanged ...
[tracker] published PersonTracked
[planner] received PersonTracked ...
... iteration=3 ...
```

A `transport error: nats: unexpected EOF` on shutdown is the Python client noticing NATS going away. It is not a demo failure.

`examples/demo` is a workspace: `app.toml`, `contracts/`, `components`, and generated code all live under it. `mica build examples/demo/app.toml` builds it; `mica graph examples/demo/app.toml` shows its topology.

## Your own application

```
mkdir hello
cd hello
mica init .
mica build
mica deploy --local --start-nats
```

`mica init` scaffolds `app.toml`, `buf.yaml`, `buf.gen.yaml`, `contracts/`, a Hello component, and `tests/`.

## Daily loop

MICA framework repository:

| Change | Then |
| --- | --- |
| `contracts/mica/*.proto` | `./scripts/generate` |
| runtime, CLI, or demo source | `./scripts/build` |
| Behavior or contracts | `./scripts/test` |

Inside an application workspace:

| Change | Then |
| --- | --- |
| `contracts/*.proto` | `mica generate` |
| component source or build metadata | `mica build` |
| behavior | `mica test` |
| run locally | `mica deploy --local` |
| deliver | `mica deploy --output dist/app.run` |

Default verification is `./scripts/test`. Integration tests need `nats-server`.

Buf breaking checks use `contracts/baseline.binpb` for the framework contracts and the workspace image for application contracts. Update a baseline only when you intentionally freeze a new contract snapshot.

## Where things live

| Path | Role |
| --- | --- |
| `contracts/` | Framework `mica.v1` contract source |
| `generated/` | Framework Buf output; gitignored; do not edit |
| `runtime/python` | `from mica import App` |
| `runtime/cpp` | `mica::App` |
| `runtime/go` | `mica.NewApp` |
| `cli/` | Go module providing `mica` |
| `examples/demo/` | Demo workspace: camera-control (C++), tracker (Python), planner (Go) |

Language APIs: `docs/python-runtime.md`, `docs/cpp-runtime.md`, `docs/go-runtime.md`.
Manifests: `docs/application-manifest.md`.
Deployment: `docs/deployment.md`.
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

Declare the same contracts in that component's `component.toml`. `mica build` checks identifiers against the workspace descriptor image.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| `buf: command not found` | `./scripts/bootstrap`; use `./scripts/*` so `.tools/bin` is on `PATH` |
| `missing descriptor image` | `mica generate` |
| `makeself is required` | `./scripts/bootstrap` or set `MICA_MAKESELF` |
| `failed to connect to transport` | start NATS (`--start-nats` or `nats-server`) |
| `mica: command not found` | `./scripts/bootstrap` or add `.tools/bin` to `PATH` |
| C++ link / missing `nats.h` | `./scripts/build` from a clean tree after bootstrap |
| target prerequisite failure | install the missing executable/library/Python package on the target |
