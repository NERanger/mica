from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import tomllib

LANGUAGES = {"python", "cpp", "go"}
RESTART_POLICIES = {"never", "on-failure"}
DEFAULT_SHUTDOWN_TIMEOUT_MS = 10000


class ManifestError(Exception):
    pass


@dataclass
class ComponentManifest:
    name: str
    language: str
    path: Path
    publishes: list[str] = field(default_factory=list)
    subscribes: list[str] = field(default_factory=list)
    calls: list[str] = field(default_factory=list)
    provides: list[str] = field(default_factory=list)


@dataclass
class ProcessSpec:
    name: str
    command: str
    args: list[str] = field(default_factory=list)
    working_directory: str | None = None
    env: dict[str, str] = field(default_factory=dict)
    component: str = ""
    restart: str = "never"
    shutdown_timeout_ms: int | None = None


@dataclass
class AppManifest:
    name: str
    path: Path
    transport_kind: str
    transport_url: str
    processes: list[ProcessSpec]
    shutdown_timeout_ms: int = DEFAULT_SHUTDOWN_TIMEOUT_MS


def load_toml(path: Path) -> dict[str, Any]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise ManifestError(f"cannot read {path}: {exc}") from exc
    try:
        return tomllib.loads(text)
    except tomllib.TOMLDecodeError as exc:
        raise ManifestError(f"invalid TOML in {path}: {exc}") from exc


def parse_component(path: Path) -> ComponentManifest:
    data = load_toml(path)
    component = data.get("component")
    if not isinstance(component, dict):
        raise ManifestError(f"{path}: missing [component] table")
    name = component.get("name")
    language = component.get("language")
    if not name or not isinstance(name, str):
        raise ManifestError(f"{path}: component.name is required")
    if language not in LANGUAGES:
        raise ManifestError(
            f"{path}: language must be one of {sorted(LANGUAGES)}, got {language!r}"
        )
    def _str_list(key: str) -> list[str]:
        value = component.get(key, [])
        if not isinstance(value, list) or any(not isinstance(v, str) for v in value):
            raise ManifestError(f"{path}: component.{key} must be a list of strings")
        return list(value)

    return ComponentManifest(
        name=name,
        language=language,
        path=path,
        publishes=_str_list("publishes"),
        subscribes=_str_list("subscribes"),
        calls=_str_list("calls"),
        provides=_str_list("provides"),
    )


def parse_app(path: Path) -> AppManifest:
    data = load_toml(path)
    app = data.get("app")
    if not isinstance(app, dict) or not app.get("name"):
        raise ManifestError(f"{path}: [app].name is required")
    transport = data.get("transport")
    if not isinstance(transport, dict):
        raise ManifestError(f"{path}: missing [transport] table")
    kind = transport.get("kind")
    url = transport.get("url")
    if kind != "nats":
        raise ManifestError(f"{path}: transport.kind must be 'nats'")
    if not url or not isinstance(url, str):
        raise ManifestError(f"{path}: transport.url is required")
    shutdown = app.get("shutdown_timeout_ms", DEFAULT_SHUTDOWN_TIMEOUT_MS)
    if not isinstance(shutdown, int) or shutdown <= 0:
        raise ManifestError(f"{path}: app.shutdown_timeout_ms must be a positive integer")
    processes_raw = data.get("process", [])
    if not isinstance(processes_raw, list) or not processes_raw:
        raise ManifestError(f"{path}: at least one [[process]] is required")
    processes: list[ProcessSpec] = []
    names: set[str] = set()
    for index, item in enumerate(processes_raw):
        if not isinstance(item, dict):
            raise ManifestError(f"{path}: process {index} is invalid")
        name = item.get("name")
        command = item.get("command")
        component = item.get("component")
        if not name or not isinstance(name, str):
            raise ManifestError(f"{path}: process {index} missing name")
        if name in names:
            raise ManifestError(f"{path}: duplicate process name {name!r}")
        names.add(name)
        if not command or not isinstance(command, str):
            raise ManifestError(f"{path}: process {name!r} missing command")
        if not component or not isinstance(component, str):
            raise ManifestError(f"{path}: process {name!r} missing component")
        args = item.get("args", [])
        if not isinstance(args, list) or any(not isinstance(a, str) for a in args):
            raise ManifestError(f"{path}: process {name!r} args must be a list of strings")
        restart = item.get("restart", "never")
        if restart not in RESTART_POLICIES:
            raise ManifestError(
                f"{path}: process {name!r} restart must be one of {sorted(RESTART_POLICIES)}"
            )
        env = item.get("env", {})
        if not isinstance(env, dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in env.items()):
            raise ManifestError(f"{path}: process {name!r} env must be a string table")
        wd = item.get("working_directory")
        if wd is not None and not isinstance(wd, str):
            raise ManifestError(f"{path}: process {name!r} working_directory must be a string")
        proc_shutdown = item.get("shutdown_timeout_ms")
        if proc_shutdown is not None and (not isinstance(proc_shutdown, int) or proc_shutdown <= 0):
            raise ManifestError(f"{path}: process {name!r} shutdown_timeout_ms must be a positive integer")
        processes.append(
            ProcessSpec(
                name=name,
                command=command,
                args=list(args),
                working_directory=wd,
                env=dict(env),
                component=component,
                restart=restart,
                shutdown_timeout_ms=proc_shutdown,
            )
        )
    return AppManifest(
        name=str(app["name"]),
        path=path,
        transport_kind=kind,
        transport_url=url,
        processes=processes,
        shutdown_timeout_ms=shutdown,
    )
