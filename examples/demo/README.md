# MICA demo: a polyglot job pipeline

This workspace is the MICA quick start. It is a complete application with
three processes in three languages, wired together only by versioned
contracts. Read this file top to bottom, run the demo, then poke at it.

## What MICA is

MICA (Multi-language Interprocess Contract Architecture) is a
contract-first platform for polyglot, multi-process applications:

- Processes communicate through **versioned protobuf contracts**. They do
  not depend on each other's implementation, language, or build system.
- The **`mica` CLI** owns the workspace lifecycle: `init`, `generate`,
  `build`, `test`, `graph`, `deploy`.
- The transport (NATS Core in V0) is an implementation detail. Application
  code uses generated tokens, never subject strings.

## The demo story

A Go client submits jobs to a C++ worker over RPC. The worker announces
each finished job as an event. A Python recorder turns those events into
an audit trail, and the client uses the audit record to submit the next
job. The loop is bounded to three jobs so the demo finishes on its own.

```
Go client
  -- RPC jobs.v1.Worker.Run -->
C++ worker
  -- event jobs.v1.JobCompleted -->
Python recorder
  -- event audit.v1.JobRecorded -->
Go client
```

Every interaction kind MICA V0 supports appears once per direction:
`provides`/`calls` (RPC) and `publishes`/`subscribes` (events).

## Workspace anatomy

One application is one workspace rooted at `app.toml`:

| Path | Role |
| --- | --- |
| `app.toml` | Application manifest: processes, transport, shutdown |
| `contracts/` | Protobuf contract source, the single source of truth |
| `components/<name>/` | One directory per component, any of the three languages |
| `components/<name>/component.toml` | Component manifest: language, build, contract usage |
| `generated/` | Output of `mica generate`; gitignored; never edited |
| `.mica/`, `dist/` | Build and deploy artifacts; gitignored |

## File-by-file walkthrough

| File | What it teaches |
| --- | --- |
| `app.toml` | Declaring processes and pointing them at components |
| `contracts/jobs/v1/jobs.proto` | A contract package: messages, an event, a service; `v1` versioning |
| `contracts/audit/v1/audit.proto` | A second, independent contract package owned by the recorder |
| `components/client/` (Go) | `app.Call` with generated tokens; `app.Subscribe`; bounding a loop |
| `components/worker/` (C++) | `app.serve` with a generated token; `app.publish`; typed `RpcError` |
| `components/recorder/` (Python) | `@app.subscribe`; `await app.publish`; async `main` |
| `components/*/component.toml` | Declaring `publishes`/`subscribes`/`calls`/`provides`, language builds |

The same four contract verbs across languages:

| Verb | Go | C++ | Python |
| --- | --- | --- | --- |
| serve RPC | `app.Serve(tokens.WorkerRun, ...)` | `app.serve(mica::tokens::Worker_Run, ...)` | `@app.serve(Worker.Run)` |
| call RPC | `app.Call(ctx, tokens.WorkerRun, req)` | `app.call(token, req, 1s)` | `await app.call(Worker.Run, req, timeout=1.0)` |
| publish | `app.Publish(ctx, event)` | `app.publish(event)` | `await app.publish(event)` |
| subscribe | `app.Subscribe((*T)(nil), ...)` | `app.subscribe<T>(...)` | `@app.subscribe(T)` |

Tokens (`Worker_Run`, `WorkerRun`, `Worker.Run`) are generated per
language from the contract package. They are the only way application
code refers to a method; subjects like `rpc.jobs.v1.Worker.Run` exist
only on the wire.

## Run it

From a MICA framework checkout (first time: `./scripts/bootstrap`):

```
./scripts/run-demo
```

That builds the workspace, starts a local `nats-server`, and runs
`mica deploy examples/demo/app.toml --local --start-nats`. Stop with
Ctrl+C. The loop ends after three iterations; the processes then keep
running until you stop them.

Or drive the workspace lifecycle yourself, as you would in your own
application:

```
mica graph examples/demo/app.toml   # static contract communication graph
mica build examples/demo/app.toml   # generate + validate + build components
mica test examples/demo/app.toml    # component and application tests
mica deploy examples/demo/app.toml --local --start-nats
mica deploy examples/demo/app.toml --output dist/demo.run  # self-contained artifact
```

## What you should see

```
[client] Run RPC completed accepted=true job=job-1
[worker] Run accepted input=task-1
[recorder] received JobCompleted job=job-1 output=done:task-1
[recorder] published JobRecorded
[client] received JobRecorded job=job-1 summary=recorded done:task-1
... job=job-2 ... job=job-3 ...
```

A `transport error: nats: unexpected EOF` on shutdown is the Python
client noticing NATS going away. It is not a failure.

## Things to try

1. **See the topology**: run `mica graph examples/demo/app.toml`. The
   graph is derived from the `component.toml` declarations, not from
   source inspection.
2. **Break the contract, watch the build catch it**: add
   `"jobs.v1.JobMisspelled"` to the recorder's `subscribes` and run
   `mica build`. Validation fails because the identifier is not in the
   workspace descriptor image.
3. **Evolve a contract**: add a field to `JobCompleted`, run
   `mica generate`, and see every language's generated code update.
   Adding a field is backward compatible; renaming or renumbering one
   is not (`mica generate` runs Buf breaking checks against the last
   descriptor image).
4. **Add a fourth component**: copy `components/recorder` to a Python
   or Go `metrics` component that also subscribes to
   `jobs.v1.JobCompleted`, add it to `app.toml`. Nothing in the worker
   changes; that is the point of events.
5. **Ship it**: `mica deploy examples/demo/app.toml --output dist/demo.run`
   produces a self-contained `.run` artifact that a target machine can
   execute without installing MICA, Go, or a build toolchain.

## Where to go next

- `docs/quick-start.md` -- developer setup and the daily loop
- `docs/contracts.md` -- protobuf identity, versioning, subjects
- `docs/application-manifest.md` -- `app.toml` and `component.toml` reference
- `docs/python-runtime.md`, `docs/cpp-runtime.md`, `docs/go-runtime.md`
- `docs/runtime-semantics.md` -- language-neutral wire behavior
- `docs/deployment.md` -- `.run` artifacts, the launcher, SSH deploy

To start your own application instead: `mica init hello`.
