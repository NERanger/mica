from __future__ import annotations

import json

from mica.cli.validate import LoadedApp, ValidationResult, validate_app


LANGUAGE_LABEL = {"python": "Python", "cpp": "C++", "go": "Go"}


def inspect_text(loaded: LoadedApp) -> str:
    lines = [f"Application: {loaded.app.name}", ""]
    for proc in loaded.processes:
        comp = proc.component
        lang = LANGUAGE_LABEL.get(comp.language, comp.language)
        lines.append(f"{proc.spec_name} [{lang}]")
        _section(lines, "provides", comp.provides)
        _section(lines, "publishes", comp.publishes)
        _section(lines, "subscribes", comp.subscribes)
        _section(lines, "calls", comp.calls)
        lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def inspect_json(loaded: LoadedApp) -> str:
    payload = {
        "application": loaded.app.name,
        "transport": {
            "kind": loaded.app.transport_kind,
            "url": loaded.app.transport_url,
        },
        "processes": [
            {
                "name": proc.spec_name,
                "component": proc.component.name,
                "language": proc.component.language,
                "provides": proc.component.provides,
                "publishes": proc.component.publishes,
                "subscribes": proc.component.subscribes,
                "calls": proc.component.calls,
            }
            for proc in loaded.processes
        ],
    }
    return json.dumps(payload, indent=2) + "\n"


def _section(lines: list[str], title: str, items: list[str]) -> None:
    if not items:
        return
    lines.append(f"  {title}:")
    for item in items:
        lines.append(f"    {item}")
    lines.append("")


def render_inspect(result: ValidationResult, fmt: str) -> str:
    if result.loaded is None:
        return ""
    if fmt == "json":
        return inspect_json(result.loaded)
    return inspect_text(result.loaded)
