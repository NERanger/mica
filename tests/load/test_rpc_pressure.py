from __future__ import annotations

import pytest

from tests.load.profile import HEAVY, profile_params
from tests.load.work import run_rpc


@pytest.mark.asyncio
@pytest.mark.parametrize("profile", profile_params())
@pytest.mark.parametrize(
    "client,server",
    [
        ("python", "cpp"),
        ("python", "go"),
        ("go", "python"),
    ],
)
async def test_rpc_pairs(profile, client, server, nats_url, cpp_harness, go_harness) -> None:
    await run_rpc(profile, client, server, nats_url, cpp_harness, go_harness)


@pytest.mark.load
@pytest.mark.asyncio
@pytest.mark.parametrize(
    "client,server",
    [
        ("python", "python"),
        ("cpp", "python"),
        ("cpp", "go"),
        ("go", "cpp"),
    ],
)
async def test_rpc_heavy_pairs(client, server, nats_url, cpp_harness, go_harness) -> None:
    await run_rpc(HEAVY, client, server, nats_url, cpp_harness, go_harness)
