# Architecture

MICA is a contract-first runtime for one application made of independent processes.

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
| `contracts/` | Versioned `.proto` source | nothing | runtimes, examples |
| `generated/` | Buf output, descriptor image, method tokens | contracts via Buf | handwritten logic |
| `runtime/python` | Python App API | generated contracts, nats-py | other processes, CLI |
| `runtime/cpp` | C++ App API | generated contracts, nats.c | other processes, CLI |
| `runtime/go` | Go App API | generated contracts, nats.go | other processes, CLI |
| `cli/` and `mica.cli` | validate, inspect, graph, run | manifests, descriptor image | process implementations |
| `examples/demo/*` | Demo processes | own language runtime + contracts | other process source |

## State ownership

| State | Owner | Canonical form | Mutation path |
| --- | --- | --- | --- |
| Topology | files | `app.toml` + `component.toml` + protos | edit files; CLI only reads |
| Child processes | `mica run` | OS processes | start, signal, kill |
| Runtime lifecycle | each `App` | STARTING/RUNNING/STOPPING/STOPPED/FAILED | `start` / `run` / `shutdown` |
| In-flight RPC | calling `App` | local deadline + envelope status | `call`; no retry |
| NATS server | external or optional `--start-nats` child | nats-server | not owned by runtimes |

## Boundaries

- Application code talks to `App`, never to NATS APIs.
- Subject names are derived from protobuf descriptors.
- RPC errors cross the wire as `mica.v1.RpcStatus`, never as language exceptions.
- Static architecture is computed from manifests and `generated/image.binpb`, not from runtime observation.
