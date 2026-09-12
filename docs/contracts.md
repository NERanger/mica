# Contracts

An application workspace keeps its contracts under `contracts/`. The MICA
framework keeps the `mica.v1` protocol contract under `contracts/` in this
repository.

## Package naming

Use `{domain}.v{N}`, for example `jobs.v1`. The protobuf package is part of wire identity.

`jobs.v1.JobCompleted` and `jobs.v2.JobCompleted` are different contracts and different subjects.

## File layout

Application workspace:

```
my-app/contracts/jobs/v1/jobs.proto
my-app/contracts/audit/v1/audit.proto
```

MICA framework:

```
contracts/mica/v1/runtime.proto
```

The directory matches the package.

## Naming

- Messages: PascalCase
- Fields: lower_snake_case
- Services: PascalCase (V0 example: `Worker`)
- RPCs: PascalCase
- Events: past-tense or fact names (`JobCompleted`, `JobRecorded`)

## Evolution

- Never reuse field numbers.
- Removed fields must be `reserved`.
- Prefer additive changes.
- Breaking API changes require a new package version.
- v1 and v2 may coexist.

## Identifiers

Event identifier: `{package}.{Message}`  
RPC identifier: `{package}.{Service}.{Method}`

These identifiers appear in `component.toml`. They are not NATS subjects.

## Buf

`buf.yaml` and `buf.gen.yaml` are YAML because Buf v2 requires YAML. This is a third-party constraint. MICA-owned configuration is TOML.

Lint: `STANDARD` except `SERVICE_SUFFIX` (V0 services follow the documented `Worker` example).

Breaking checks: `FILE` category against the framework `contracts/baseline.binpb`. Update that baseline only when an intentional contract freeze is accepted.

## Generated code

Never edit `generated/`. Run `./scripts/generate` for framework contracts and `mica generate` for a workspace. Generated language sources are not committed. `contracts/baseline.binpb` is the framework breaking-change snapshot.
