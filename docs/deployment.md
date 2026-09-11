# Deployment

`mica deploy` is the delivery command for an application workspace. It builds components, assembles a deployment artifact, and can execute the artifact locally or over SSH.

```
mica deploy app.toml --output dist/demo.run
mica deploy app.toml --local --start-nats
mica deploy app.toml --host user@machine
```

`--local` and `--host` are mutually exclusive. Without either flag, deploy only writes the artifact.

## Pipeline

```
manifest/contract checks + generate
        |
        v
build components (native adapters)
        |
        v
stage deployment directory
        |
        v
makeself .run
        |
        v
optional local or SSH execution
```

`package` is an internal phase, not a user command. The artifact is always created.

## Artifact contents

```
components/<process>/...   built component artifacts
contracts/image.binpb      contract descriptor image
deployment.toml            resolved launch manifest
generated/python/...       generated Python packages
python/mica/...            bundled Python runtime when available
mica-launcher              process supervisor for the target
startup.sh                 makeself entry point
```

The artifact is self-contained for execution. The target does not need the `mica` CLI, Go, or a build toolchain.

## The .run file

`mica deploy` produces an executable makeself archive. Copy it to a target and run it:

```
./demo.run
```

makeself verifies checksums, extracts to a temporary directory, runs the launcher, and removes the temporary directory when the launcher exits. Ctrl+C stops the application.

The first version only executes the application. There is no `install`, `start`, `stop`, `restart`, `upgrade`, `rollback`, or service registration.

## Launcher

The launcher is the process supervisor inside the artifact. It:

- reads `deployment.toml`
- checks target prerequisites before starting any process
- starts processes in declared order
- injects `MICA_NATS_URL`, `MICA_APP_NAME`, and `MICA_COMPONENT_NAME`
- prefixes and forwards process output
- handles SIGINT/SIGTERM and process groups
- applies `on-failure` restarts
- waits for the application shutdown timeout
- returns the application exit code

## Target prerequisites

An artifact declares what the target must provide. The launcher reports every failure and refuses to start processes when requirements are missing.

- `executables`: looked up on `PATH`
- `libraries`: checked with `ldconfig` or standard library directories
- `requirements.python.version`: checked against `python3`
- `requirements.python.packages`: checked with `importlib.metadata`

MICA does not install dependencies on the target. External NATS is a service requirement, not a bundled dependency; the launcher checks its TCP endpoint before starting processes. `--start-nats` is a development convenience for `--local` only.

Python components are delivered as source bundles. Whether to bundle an interpreter or virtual environment is a future adapter/profile capability.

## Platform

The artifact records the build platform (`os`, `arch`). The launcher rejects a mismatch. Cross-compilation is not supported in V0; build on the target platform or use `--host` so the CLI detects the remote platform before uploading.

## SSH

`--host user@machine` uses the system OpenSSH client:

```
probe remote platform
create remote temporary directory
upload the .run
execute it
forward output and exit code
remove the remote temporary directory
```

SSH configuration, keys, and `known_hosts` come from the user's OpenSSH setup. Credentials are never stored in MICA manifests. `--ssh-port` selects a non-default port.

## Removed commands

`mica run` no longer exists. Local development is `mica deploy --local`. `mica validate` and `mica inspect` no longer exist; validation is part of `build`, and `graph` covers static topology output.
