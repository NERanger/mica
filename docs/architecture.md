# Architecture

MICA is a contract-first application platform for one application made of independent processes.

```
Application process
        |
        v
     Contract
        |
        v
  MICA runtime
        |
        v
     Transport
        |
        v
       NATS Core
```

Processes depend on versioned Protobuf contracts and a language runtime. They do not depend on each other.

NATS is the V0 transport implementation, not the application programming model. Application code never writes NATS subject strings.

## Modules

| Module | Purpose | May depend on | Must not depend on |
| --- | --- | --- | --- |
| `contracts/` | Framework contract source (`mica.v1`) | nothing | runtimes, CLI |
| `generated/` | Framework Buf output for runtimes | contracts via Buf | handwritten logic |
| `runtime/python` | Python App API | framework contracts, nats-py | other processes, CLI |
| `runtime/cpp` | C++ App API | framework contracts, nats.c | other processes, CLI |
| `runtime/go` | Go App API | framework contracts, nats.go | other processes, CLI |
| `cli/` | `mica` command: workspace lifecycle | manifests, descriptor image, native build tools | process implementations |
| `examples/demo/*` | Demo workspace | own contracts + language runtimes | other process source |
| `scripts/*` | MICA framework repository tooling | all modules | nothing |

The MICA framework repository produces runtimes and the CLI. It is not itself a workspace.

## Workspace model

A user project is a workspace and an application at the same time.

```
my-app/
├── app.toml            workspace and application manifest
├── buf.yaml            Buf module configuration
├── buf.gen.yaml        Buf generation configuration
├── contracts/          application .proto source
├── components/         buildable components
├── tests/              application tests
├── generated/          contract output; never edit
└── .mica/              build and deployment staging
```

`mica` is the only entry point for the workspace lifecycle. The CLI reads manifests and the descriptor image, builds components through native adapters, and produces deployment artifacts.

## State ownership

| State | Owner | Canonical form | Mutation path |
| --- | --- | --- | --- |
| Topology | files | `app.toml` + `component.toml` + protos | edit files; CLI only reads |
| Generated code | `mica generate` | `<workspace>/generated` + `image.binpb` | deterministic derivation from contracts |
| Build artifacts | `mica build` | `<workspace>/.mica/build/<component>` | native adapters |
| Deployment artifact | `mica deploy` | `<workspace>/dist/<app>.run` | staging + makeself |
| Child processes | `mica` launcher | OS processes | start, signal, kill |
| Runtime lifecycle | each `App` | STARTING/RUNNING/STOPPING/STOPPED/FAILED | `start` / `run` / `shutdown` |
| In-flight RPC | calling `App` | local deadline + envelope status | `call`; no retry |
| NATS server | external or optional `--start-nats` child | nats-server | not owned by runtimes |

## Boundaries

- Application code talks to `App`, never to NATS APIs.
- Subject names are derived from protobuf descriptors.
- RPC errors cross the wire as `mica.v1.RpcStatus`, never as language exceptions. Caller deadline expiry and caller cancellation are local `CallTimeout` / `CallCancelled` outcomes, not `RpcStatus`.
- Static architecture is computed from manifests and the descriptor image, not from runtime observation.
- Components own their native build declaration. MICA owns lifecycle, artifact flow, and deployment.
- The CLI does not import language runtimes. Language runtimes do not import the CLI.
- The launcher inside a deployment artifact does not depend on the CLI being installed on the target.
