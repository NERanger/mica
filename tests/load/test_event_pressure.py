from __future__ import annotations

import asyncio

import pytest

from mica import App, AppConfig
from tests.conftest import harness_env
from tests.load.harness import start, stop, wait_report
from tests.load.profile import HEAVY, profile_params
from tests.load.stats import assert_counts, assert_p99
from tests.load.work import publish_events, run_events


@pytest.mark.asyncio
@pytest.mark.parametrize("profile", profile_params())
@pytest.mark.parametrize(
    "pub,sub",
    [
        ("python", "cpp"),
        ("cpp", "python"),
        ("go", "python"),
    ],
)
async def test_event_pairs(profile, pub, sub, nats_url, cpp_harness, go_harness) -> None:
    await run_events(profile, pub, sub, nats_url, cpp_harness, go_harness)


@pytest.mark.load
@pytest.mark.asyncio
@pytest.mark.parametrize(
    "pub,sub",
    [
        ("python", "go"),
        ("python", "python"),
        ("cpp", "go"),
        ("go", "cpp"),
    ],
)
async def test_event_heavy_pairs(pub, sub, nats_url, cpp_harness, go_harness) -> None:
    await run_events(HEAVY, pub, sub, nats_url, cpp_harness, go_harness)


@pytest.mark.load
@pytest.mark.asyncio
async def test_event_fanout_python_to_cpp_and_go(nats_url, cpp_harness, go_harness) -> None:
    profile = HEAVY
    env = harness_env(nats_url)
    count = profile.event_count
    wait_s = profile.wait_s
    cpp_sub = start(cpp_harness, ["subscribe-load", str(count), str(wait_s)], env)
    go_sub = start(go_harness, ["subscribe-load", str(count), str(wait_s)], env)
    pub = App("py-pub", AppConfig(name="py-pub", transport_url=nats_url))
    try:
        await asyncio.sleep(0.4)
        await pub.start()
        try:
            await publish_events(pub, count, profile.event_rate)
        finally:
            await pub.shutdown()
        cpp_report = wait_report(cpp_sub, wait_s + 5)
        cpp_sub = None
        go_report = wait_report(go_sub, wait_s + 5)
        go_sub = None
        assert_counts(received=cpp_report["received"], expected=count, errors=cpp_report["errors"])
        assert_counts(received=go_report["received"], expected=count, errors=go_report["errors"])
        assert_p99(cpp_report["p99_ns"], profile.p99_event_ms, "event fanout cpp")
        assert_p99(go_report["p99_ns"], profile.p99_event_ms, "event fanout go")
    finally:
        stop(cpp_sub)
        stop(go_sub)
