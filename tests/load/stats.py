from __future__ import annotations

import math


def percentile_ns(samples: list[int], p: float) -> int:
    if not samples:
        return 0
    xs = sorted(samples)
    k = int(math.ceil(p / 100.0 * len(xs))) - 1
    k = min(max(k, 0), len(xs) - 1)
    return xs[k]


def summarize(samples: list[int], warmup: int) -> dict[str, int]:
    used = samples[warmup:] if len(samples) > warmup else list(samples)
    if not used:
        return {"p50_ns": 0, "p99_ns": 0, "max_ns": 0, "percentile_samples": 0}
    return {
        "p50_ns": percentile_ns(used, 50),
        "p99_ns": percentile_ns(used, 99),
        "max_ns": max(used),
        "percentile_samples": len(used),
    }


def assert_counts(
    *,
    received: int,
    expected: int,
    errors: int = 0,
    timeouts: int = 0,
    unavailable: int = 0,
) -> None:
    assert errors == 0, f"errors={errors}"
    assert timeouts == 0, f"timeouts={timeouts}"
    assert unavailable == 0, f"unavailable={unavailable}"
    assert received == expected, f"received {received} expected {expected}"


def assert_p99(p99_ns: int, limit_ms: float, label: str) -> None:
    p99_ms = p99_ns / 1_000_000
    assert p99_ms < limit_ms, f"{label} p99 {p99_ms:.2f}ms >= {limit_ms}ms"
