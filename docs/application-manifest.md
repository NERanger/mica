# Application manifests

MICA-owned configuration is TOML.

## app.toml

```toml
[app]
name = "demo"
shutdown_timeout_ms = 5000

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "tracker"
command = "python3"
args = ["-m", "tracker"]
working_directory = "."
component = "./tracker/component.toml"
restart = "never"
env = { PYTHONUNBUFFERED = "1" }
```

Supported fields:

- application name, shutdown timeout
- transport kind (`nats`) and URL
- process name, command, args, working directory, env, component path
- restart: `never` or `on-failure`

Paths are resolved relative to the directory containing `app.toml`.

The launcher injects `MICA_NATS_URL`, `MICA_APP_NAME`, and `MICA_COMPONENT_NAME`.

## component.toml

```toml
[component]
name = "tracker"
language = "python"

publishes = ["tracking.v1.PersonTracked"]
subscribes = ["camera.v1.PoseChanged"]
calls = []
provides = []
```

`language` is `python`, `cpp`, or `go`.

Contract identifiers must exist in `generated/image.binpb`. Subjects are never listed here.

## Commands

```
mica validate examples/demo/app.toml
mica inspect examples/demo/app.toml
mica inspect examples/demo/app.toml --format json
mica graph examples/demo/app.toml
mica graph examples/demo/app.toml --format dot
mica run examples/demo/app.toml
mica run examples/demo/app.toml --start-nats
```

Validate errors are fatal. Topology mismatches (publisher without subscriber, call without provider) are warnings.
