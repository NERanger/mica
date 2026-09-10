from __future__ import annotations

import os
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path

from mica.cli.contracts import find_repo_root
from mica.cli.manifest import ProcessSpec
from mica.cli.validate import LoadedApp, ValidationResult


class LaunchError(Exception):
    pass


class Child:
    def __init__(self, spec: ProcessSpec, proc: subprocess.Popen[str]) -> None:
        self.spec = spec
        self.proc = proc


def _prefix_stream(name: str, stream, dest) -> None:
    for line in iter(stream.readline, ""):
        dest.write(f"[{name}] {line}")
        dest.flush()
    stream.close()


def _resolve_cwd(app_dir: Path, spec: ProcessSpec) -> Path:
    if spec.working_directory:
        path = Path(spec.working_directory)
        return path if path.is_absolute() else (app_dir / path).resolve()
    return app_dir


def _command(app_dir: Path, spec: ProcessSpec) -> list[str]:
    command = Path(spec.command)
    resolved = command if command.is_absolute() else (app_dir / command)
    if resolved.exists():
        argv0 = str(resolved.resolve())
    else:
        argv0 = spec.command
    return [argv0, *spec.args]


def _child_env(loaded: LoadedApp, spec: ProcessSpec, repo: Path | None) -> dict[str, str]:
    env = os.environ.copy()
    env.update(spec.env)
    env["MICA_NATS_URL"] = loaded.app.transport_url
    env["MICA_APP_NAME"] = loaded.app.name
    env["MICA_COMPONENT_NAME"] = spec.name
    env.setdefault("PYTHONUNBUFFERED", "1")
    if repo is not None:
        extra = [
            str(repo / "generated" / "python"),
            str(repo / "runtime" / "python" / "src"),
            str(loaded.app.path.parent),
        ]
        existing = env.get("PYTHONPATH", "")
        env["PYTHONPATH"] = os.pathsep.join([p for p in extra if p] + ([existing] if existing else []))
        env.setdefault("PATH", os.environ.get("PATH", ""))
        tools = repo / ".tools" / "bin"
        if tools.is_dir():
            env["PATH"] = str(tools) + os.pathsep + env["PATH"]
    return env


def _start_nats(url: str) -> subprocess.Popen[str] | None:
    if not url.startswith("nats://"):
        raise LaunchError(f"unsupported transport url: {url}")
    hostport = url[len("nats://") :]
    if "@" in hostport:
        hostport = hostport.split("@", 1)[1]
    host, _, port = hostport.partition(":")
    port = port or "4222"
    host = host or "127.0.0.1"
    nats_server = os.environ.get("NATS_SERVER", "nats-server")
    proc = subprocess.Popen(
        [nats_server, "-a", host, "-p", port],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    for _ in range(50):
        if proc.poll() is not None:
            raise LaunchError("nats-server exited before becoming ready")
        time.sleep(0.05)
    return proc


def run_app(result: ValidationResult, *, start_nats: bool = False) -> int:
    if result.loaded is None:
        for error in result.errors:
            print(error, file=sys.stderr)
        return 1
    loaded = result.loaded
    for warning in result.warnings:
        print(f"warning: {warning}", file=sys.stderr)
    app_dir = loaded.app.path.parent
    repo = find_repo_root(app_dir)
    nats_proc = None
    if start_nats:
        nats_proc = _start_nats(loaded.app.transport_url)
        print("[mica] started nats-server")
    children: list[Child] = []
    stop = threading.Event()
    exit_code = 0

    def spawn(spec: ProcessSpec) -> Child:
        cwd = _resolve_cwd(app_dir, spec)
        argv = _command(app_dir, spec)
        proc = subprocess.Popen(
            argv,
            cwd=str(cwd),
            env=_child_env(loaded, spec, repo),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            start_new_session=True,
        )
        print(f"[{spec.name}] started")
        if proc.stdout is not None:
            threading.Thread(
                target=_prefix_stream,
                args=(spec.name, proc.stdout, sys.stdout),
                daemon=True,
            ).start()
        return Child(spec, proc)

    try:
        for spec in loaded.app.processes:
            children.append(spawn(spec))
            time.sleep(0.2)
    except OSError as exc:
        print(f"failed to start process: {exc}", file=sys.stderr)
        exit_code = 1
        stop.set()

    def handle_signal(_signum, _frame) -> None:
        stop.set()

    signal.signal(signal.SIGINT, handle_signal)
    signal.signal(signal.SIGTERM, handle_signal)

    try:
        while not stop.is_set():
            for child in list(children):
                code = child.proc.poll()
                if code is None:
                    continue
                print(f"[{child.spec.name}] exited with {code}")
                if child.spec.restart == "on-failure" and code != 0 and not stop.is_set():
                    print(f"[{child.spec.name}] restarting")
                    children = [c for c in children if c is not child]
                    children.append(spawn(child.spec))
                    continue
                if code != 0:
                    exit_code = 1
                stop.set()
                break
            else:
                time.sleep(0.1)
                continue
            break
    finally:
        timeout_s = loaded.app.shutdown_timeout_ms / 1000.0
        for child in children:
            if child.proc.poll() is None:
                try:
                    os.killpg(child.proc.pid, signal.SIGTERM)
                except (ProcessLookupError, PermissionError):
                    child.proc.terminate()
        deadline = time.monotonic() + timeout_s
        for child in children:
            remaining = max(0.0, deadline - time.monotonic())
            try:
                child.proc.wait(timeout=remaining)
            except subprocess.TimeoutExpired:
                try:
                    os.killpg(child.proc.pid, signal.SIGKILL)
                except (ProcessLookupError, PermissionError):
                    child.proc.kill()
                child.proc.wait()
            else:
                print(f"[{child.spec.name}] stopped")
        if nats_proc is not None and nats_proc.poll() is None:
            nats_proc.terminate()
            try:
                nats_proc.wait(timeout=2)
            except subprocess.TimeoutExpired:
                nats_proc.kill()
    return exit_code
