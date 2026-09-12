# MICA

**MICA = Multi-language Interprocess Contract Architecture**

Contract-first application platform for polyglot, multi-process applications.

> Processes communicate through versioned contracts. They do not depend on each other's implementation.

> NATS is MICA's V0 transport implementation, not MICA's application programming model.

One application is one workspace, described by `app.toml`. The `mica` CLI owns the workspace lifecycle: initialize, generate, build, test, graph, and deploy.

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
mica graph examples/demo/app.toml
./scripts/run-demo
```

`scripts/run-demo` starts a local `nats-server` and runs the three-language demo through `mica deploy --local`. Ctrl+C stops it.

## Lifecycle

```
mica init        create an application workspace
mica generate    contracts -> generated code, descriptor image, tokens
mica build       generate + validate + build components
mica test        component and application tests
mica graph       static contract communication graph
mica deploy      build, package as a makeself .run, execute locally or over SSH
```

`mica deploy` produces a self-contained `.run` artifact. The target machine executes it without installing MICA, Go, or a build toolchain. See [docs/deployment.md](docs/deployment.md).

## Demo

```
Go client
  -- RPC jobs.v1.Worker.Run -->
C++ worker
  -- event jobs.v1.JobCompleted -->
Python recorder
  -- event audit.v1.JobRecorded -->
Go client
```

The client submits jobs over RPC; the worker announces completions as
events; the recorder turns them into an audit trail that drives the next
iteration. The loop is bounded to three `Run` calls.

`examples/demo/README.md` is a guided walkthrough of the workspace and
the concepts it demonstrates.

## Repository

```
contracts/     framework mica.v1 contract source
generated/     framework Buf output (not committed; run ./scripts/generate)
runtime/       Python, C++, Go App APIs
cli/           Go module providing the mica workspace CLI
examples/demo  quick-start demo workspace (see examples/demo/README.md)
tests/         contract, compatibility, integration, failure
docs/          current-state documentation
scripts/       bootstrap, generate, build, test, run-demo
```

## Configuration

MICA-owned files are TOML (`app.toml`, `component.toml`, `pyproject.toml`).

`buf.yaml` and `buf.gen.yaml` are YAML because Buf requires YAML.
