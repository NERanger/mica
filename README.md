# MICA

**MICA = Multi-language Interprocess Contract Architecture**

Contract-first runtime for polyglot, multi-process applications.

> Processes communicate through versioned contracts. They do not depend on each other's implementation.

> NATS is MICA's V0 transport implementation, not MICA's application programming model.

V0 languages: Python, C++, Go. Transport: NATS Core.

## What MICA is not

Not ROS2, Kubernetes, a service mesh, an actor framework, a schema registry, or a generic microservice platform.

V0 does not include JetStream, retries, streaming RPC, gRPC transport, or YAML configuration except Buf's own files.

## Quick start

Developer setup, demo, and daily loop: [docs/quick-start.md](docs/quick-start.md).

```
./scripts/bootstrap
./scripts/build
./scripts/test
mica validate examples/demo/app.toml
mica inspect examples/demo/app.toml
mica graph examples/demo/app.toml
./scripts/run-demo
```

`scripts/run-demo` starts a local `nats-server` and the three-language demo. Ctrl+C stops it.

## Demo

```
Go planner
  -- RPC camera.v1.CameraControl.SetPose -->
C++ camera-control
  -- event camera.v1.PoseChanged -->
Python tracker
  -- event tracking.v1.PersonTracked -->
Go planner
```

The loop is bounded to three `SetPose` calls.

## Repository

```
contracts/     protobuf source of truth
generated/     Buf output (not committed; run ./scripts/generate)
runtime/       Python, C++, Go App APIs
cli/           mica validate|inspect|graph|run
examples/demo  three-language application
tests/         contract, compatibility, integration, failure
docs/          current-state documentation
scripts/       bootstrap, generate, build, test, run-demo
```

## Configuration

MICA-owned files are TOML (`app.toml`, `component.toml`, `pyproject.toml`).

`buf.yaml` and `buf.gen.yaml` are YAML because Buf requires YAML.
