from __future__ import annotations

import json
import os

KINDS = ("publishes", "subscribes", "calls", "provides")


class Surface:
    def __init__(self) -> None:
        self._observed: dict[str, set[str]] = {kind: set() for kind in KINDS}

    def record(self, kind: str, contract_id: str) -> None:
        self._observed[kind].add(contract_id)

    def as_dict(self, component: str) -> dict[str, object]:
        return {
            "component": component,
            **{kind: sorted(ids) for kind, ids in self._observed.items()},
        }

    def write(self, path: str, component: str) -> None:
        directory = os.path.dirname(path)
        if directory:
            os.makedirs(directory, exist_ok=True)
        with open(path, "w", encoding="utf-8") as handle:
            json.dump(self.as_dict(component), handle)
