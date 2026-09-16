from __future__ import annotations

from dataclasses import dataclass

import pytest


@dataclass(frozen=True)
class Profile:
    name: str
    duration_s: float
    event_rate: int
    rpc_rate: int
    rpc_concurrency: int
    warmup: int
    p99_event_ms: float
    p99_rpc_ms: float
    p99_pipeline_ms: float
    rpc_timeout_s: float
    drain_s: float

    @property
    def event_count(self) -> int:
        return int(self.duration_s * self.event_rate)

    @property
    def rpc_count(self) -> int:
        return int(self.duration_s * self.rpc_rate)

    @property
    def wait_s(self) -> float:
        return self.duration_s + self.drain_s + 3.0


SMOKE = Profile(
    name="smoke",
    duration_s=2.0,
    event_rate=1000,
    rpc_rate=200,
    rpc_concurrency=20,
    warmup=100,
    p99_event_ms=20.0,
    p99_rpc_ms=20.0,
    p99_pipeline_ms=50.0,
    rpc_timeout_s=1.0,
    drain_s=2.0,
)

HEAVY = Profile(
    name="heavy",
    duration_s=15.0,
    event_rate=1000,
    rpc_rate=200,
    rpc_concurrency=20,
    warmup=100,
    p99_event_ms=20.0,
    p99_rpc_ms=20.0,
    p99_pipeline_ms=50.0,
    rpc_timeout_s=1.0,
    drain_s=2.0,
)


def profile_params() -> list:
    return [
        pytest.param(SMOKE, id="smoke"),
        pytest.param(HEAVY, id="heavy", marks=pytest.mark.load),
    ]
