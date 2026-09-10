from __future__ import annotations

from mica.cli.validate import LoadedApp


def graph_ascii(loaded: LoadedApp) -> str:
    names = [proc.spec_name for proc in loaded.processes]
    languages = {
        proc.spec_name: proc.component.language.upper() if proc.component.language != "cpp" else "C++"
        for proc in loaded.processes
    }
    if loaded.processes and loaded.processes[0].component.language == "python":
        languages[loaded.processes[0].spec_name] = "Python"
    for proc in loaded.processes:
        if proc.component.language == "python":
            languages[proc.spec_name] = "Python"
        elif proc.component.language == "go":
            languages[proc.spec_name] = "Go"
        else:
            languages[proc.spec_name] = "C++"

    width = max(len(name) for name in names + [" "]) + 2
    width = max(width, 18)

    def box(name: str) -> list[str]:
        label = languages[name]
        inner = width
        border = "+" + "-" * inner + "+"
        return [
            border,
            "|" + name.ljust(inner) + "|",
            "|" + label.ljust(inner) + "|",
            border,
        ]

    edges: list[tuple[str, str, str]] = []
    publishers: dict[str, list[str]] = {}
    subscribers: dict[str, list[str]] = {}
    callers: dict[str, list[str]] = {}
    providers: dict[str, list[str]] = {}
    for proc in loaded.processes:
        for item in proc.component.publishes:
            publishers.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.subscribes:
            subscribers.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.calls:
            callers.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.provides:
            providers.setdefault(item, []).append(proc.spec_name)
    for contract, pubs in publishers.items():
        for src in pubs:
            for dst in subscribers.get(contract, []):
                edges.append((src, dst, contract))
    for contract, srcs in callers.items():
        for src in srcs:
            for dst in providers.get(contract, []):
                edges.append((src, dst, f"RPC {contract}"))

    lines: list[str] = []
    boxed: set[str] = set()
    if not names:
        return ""

    def ensure_box(name: str) -> None:
        if name in boxed:
            return
        for line in box(name):
            lines.append(line)
        boxed.add(name)

    for name in names:
        ensure_box(name)
        outgoing = [edge for edge in edges if edge[0] == name]
        for _, dst, label in outgoing:
            lines.append(" " * (width // 2) + "|")
            lines.append(" " * (width // 2) + f"| {label}")
            lines.append(" " * (width // 2) + "v")
            if dst in boxed:
                lines.append(" " * ((width - len(dst)) // 2) + dst)
            else:
                ensure_box(dst)
        if outgoing:
            lines.append("")
    return "\n".join(lines).rstrip() + "\n"


def graph_dot(loaded: LoadedApp) -> str:
    lines = [f'digraph "{loaded.app.name}" {{', "  rankdir=TB;"]
    for proc in loaded.processes:
        lang = proc.component.language
        lines.append(
            f'  "{proc.spec_name}" [label="{proc.spec_name}\\n{lang}"];'
        )
    seen: set[tuple[str, str, str]] = set()
    pubs: dict[str, list[str]] = {}
    subs: dict[str, list[str]] = {}
    calls: dict[str, list[str]] = {}
    prov: dict[str, list[str]] = {}
    for proc in loaded.processes:
        for item in proc.component.publishes:
            pubs.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.subscribes:
            subs.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.calls:
            calls.setdefault(item, []).append(proc.spec_name)
        for item in proc.component.provides:
            prov.setdefault(item, []).append(proc.spec_name)
    for contract, srcs in pubs.items():
        for src in srcs:
            for dst in subs.get(contract, []):
                key = (src, dst, contract)
                if key in seen:
                    continue
                seen.add(key)
                lines.append(f'  "{src}" -> "{dst}" [label="{contract}"];')
    for contract, srcs in calls.items():
        for src in srcs:
            for dst in prov.get(contract, []):
                key = (src, dst, contract)
                if key in seen:
                    continue
                seen.add(key)
                lines.append(f'  "{src}" -> "{dst}" [label="RPC {contract}"];')
    lines.append("}")
    return "\n".join(lines) + "\n"
