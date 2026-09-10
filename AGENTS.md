# MICA

MICA is a contract-first runtime for polyglot, multi-process applications.
V0 supports Python, C++, and Go over NATS Core.

Processes communicate through versioned contracts. They do not depend on each
other's implementation. NATS is the V0 transport implementation, not the
application programming model.

## Engineering Principles

- Backward compatibility is not a default goal. Once a code path or contract is
  explicitly superseded and approved for removal, delete it instead of retaining
  a compatibility layer, legacy fallback, dual path, or migration path. An
  active documented contract remains a current requirement until an approved
  design change replaces it.
- Use the simplest implementation that fully satisfies current requirements.
  Do not add speculative abstractions, configuration, or indirection.
- Deliver the smallest end-to-end working version first, then add capability in
  layers on top of stable behavior.
- Keep components modular, with explicit responsibilities and separation of
  concerns, while preserving the documented ownership and dependency rules.
- Before implementing functionality or adding a dependency, inspect the
  documentation, API, and type definitions of existing dependencies for the
  required capability.
- Prefer a mature, maintained library when it reduces total complexity or
  improves reliability; do not reimplement common functionality without a
  concrete reason.
- Make architecture decisions for long-term evolution. Do not adopt a knowingly
  temporary design that is expected to require replacement.
- Before designing an unfamiliar solution, study how mature products,
  standards, or established implementations solve the same problem and prefer
  proven patterns and conventions.

## Read First

| Document | Authority |
| --- | --- |
| `docs/index.md` | Documentation index |
| `docs/quick-start.md` | Developer setup, first run, daily loop |
| `docs/architecture.md` | Modules, boundaries, dependency direction |
| `docs/runtime-semantics.md` | Language-neutral runtime behavior |
| `docs/contracts.md` | Protobuf identity, versioning, subjects |
| `docs/application-manifest.md` | `app.toml` and `component.toml` |
| `docs/python-runtime.md` | Python API and concurrency |
| `docs/cpp-runtime.md` | C++ API and threading |
| `docs/go-runtime.md` | Go API and goroutines |

## Design Changes

Changes to module decomposition, state ownership, architectural boundaries, or
public/internal contracts are design changes and require an explicit user
decision. Document the current design and evidence, present options and a
recommendation, obtain the decision, and record it before implementation.

## Commands

```
./scripts/bootstrap
./scripts/generate
./scripts/build
./scripts/test
mica validate examples/demo/app.toml
mica inspect examples/demo/app.toml
mica graph examples/demo/app.toml
mica run examples/demo/app.toml
```

Default verification: `./scripts/test`.
Integration tests require `nats-server`.

## Scope

V0 languages: Python, C++, Go. Transport: NATS Core only.
Do not add JetStream, retries, streaming RPC, gRPC transport, YAML (except Buf),
or process implementation dependencies.

## Generated Code

Never edit `generated/`. Run `./scripts/generate` after contract changes.
`buf.yaml` and `buf.gen.yaml` are YAML because Buf requires it.
