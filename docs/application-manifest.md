# Application manifests

MICA-owned configuration is TOML. A workspace is one application.

The root `app.toml` identifies the workspace. `mica` finds it in the current directory or walks upward. Paths are resolved relative to the directory that contains `app.toml`.

## app.toml

```toml
[app]
name = "demo"
shutdown_timeout_ms = 5000

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "worker"
component = "./components/worker/component.toml"
restart = "never"
env = {}
```

Fields:

- `app.name`, `app.shutdown_timeout_ms`
- `transport.kind` (`nats`), `transport.url`
- `process.name`: unique process instance name
- `process.component`: path to `component.toml`
- `process.args`, `process.env`, `process.working_directory`
- `process.restart`: `never` or `on-failure`
- `process.shutdown_timeout_ms`

Optional tables:

```toml
[generated]
go_runtime = "mica/runtime/go/mica"   # optional Go import path used by generated tokens

[test]
command = "python3"
args = ["-m", "pytest", "tests"]
```

`command` is never part of an application process. A process references a component; the CLI resolves the component artifact and launch information through the component's native build adapter.

## component.toml

A component declares its contract surface, native build, artifact, launch information, tests, and target prerequisites.

```toml
[component]
name = "worker"
language = "cpp"

publishes = ["jobs.v1.JobCompleted"]
subscribes = []
calls = []
provides = ["jobs.v1.Worker.Run"]

[build]
adapter = "cmake"

[build.cmake]
source_dir = "."
target = "worker"
configure_args = []
build_args = []

[artifact]
kind = "executable"
path = "worker"

[requirements]
executables = []
libraries = ["libstdc++.so.6"]
```

`language` is `python`, `cpp`, or `go`. Contract identifiers must exist in the workspace descriptor image. Subjects are never listed here.

### build adapters

`adapter = "cmake"`:

```toml
[build.cmake]
source_dir = "."
target = "worker"
configure_args = []
build_args = []
```

The adapter configures and builds the target into `.mica/build/<component>`. The component CMake project may use `MICA_SDK_DIR`, `MICA_GENERATED_DIR`, and `MICA_WORKSPACE_DIR`.

When building outside the MICA framework repository, set `MICA_SDK_ROOT` to the installed or checked-out MICA framework root so the adapter can provide `MICA_SDK_DIR`.

`adapter = "go"`:

```toml
[build.go]
module_dir = "../../../.."
package = "./examples/demo/components/client"
output = "client"
build_args = []
```

`adapter = "python"`:

```toml
[build.python]
source = "."
module = "recorder"
```

The Python adapter copies the source into the build output and validates syntax.

### artifact

Every component must declare a standard artifact so deploy does not guess build paths.

- `kind = "executable"`: `path` is relative to the adapter output directory.
- `kind = "python"`: `path` is the copied source tree; `[run]` is required.

### run

Optional for executables, required for Python artifacts. Describes how the artifact starts.

```toml
[run]
command = "python3"
args = ["-m", "recorder"]
working_directory = "."
```

### test

Optional component test command. Without it the adapter runs its native default when tests exist (`ctest`, `go test`, `pytest`).

```toml
[test]
command = "python3"
args = ["-m", "pytest", "."]
```

### requirements

Target prerequisites are checked by the launcher before any process starts.

```toml
[requirements]
executables = ["python3"]
libraries = ["libstdc++.so.6"]

[requirements.python]
version = ">=3.12"
packages = ["nats-py>=2.9.0", "protobuf>=4.21.12,<6"]
```

## deployment.toml

`deployment.toml` is generated inside a deployment artifact. It is not written by users and contains resolved launch information for the launcher: application name, transport, platform, process commands, arguments, environment, and merged prerequisites.

## Commands

```
mica init [directory]
mica generate [app.toml]
mica build [app.toml]
mica test [app.toml]
mica graph [app.toml] [--format ascii|dot]
mica deploy [app.toml] [--output PATH] [--local] [--host HOST]
```

- `build` performs contract generation and manifest validation before building.
- Contract errors are fatal. Topology mismatches (publisher without subscriber, call without provider) are warnings.
- `graph` is static; it does not build or start processes.
