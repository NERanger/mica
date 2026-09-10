from __future__ import annotations

from pathlib import Path

import tomllib

from mica.subject import event_subject, rpc_subject_from_contract

CASES = Path(__file__).with_name("cases.toml")


def test_subject_cases() -> None:
    data = tomllib.loads(CASES.read_text(encoding="utf-8"))
    for item in data.get("event", []):
        assert event_subject(item["contract"]) == item["expected_subject"]
    for item in data.get("rpc", []):
        assert rpc_subject_from_contract(item["contract"]) == item["expected_subject"]
