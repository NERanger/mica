from __future__ import annotations

import os

import pytest


def pytest_collection_modifyitems(config: pytest.Config, items: list[pytest.Item]) -> None:
    if os.environ.get("MICA_LOAD") == "1":
        return
    selected: list[pytest.Item] = []
    deselected: list[pytest.Item] = []
    for item in items:
        if item.get_closest_marker("load"):
            deselected.append(item)
        else:
            selected.append(item)
    if deselected:
        config.hook.pytest_deselected(items=deselected)
        items[:] = selected
