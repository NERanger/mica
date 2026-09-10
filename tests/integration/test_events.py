from __future__ import annotations

import asyncio
import subprocess
import time

import pytest
from camera.v1.camera_pb2 import PoseChanged
from mica import App, AppConfig
from tests.conftest import harness_env


@pytest.mark.asyncio
async def test_python_to_cpp_event(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    sub = subprocess.Popen(
        [str(cpp_harness), "subscribe-pose"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    await asyncio.sleep(0.4)
    app = App("py-pub", AppConfig(name="py-pub", transport_url=nats_url))
    await app.start()
    event = PoseChanged()
    event.camera_id.value = "cam-1"
    event.pose.pan = 1.5
    await app.publish(event)
    await app.shutdown()
    try:
        stdout, _ = sub.communicate(timeout=5)
    except subprocess.TimeoutExpired:
        sub.kill()
        raise
    assert sub.returncode == 0
    assert "got pan=" in stdout


def test_cpp_to_go_event(nats_url: str, cpp_harness, go_harness) -> None:
    env = harness_env(nats_url)
    sub = subprocess.Popen(
        [str(go_harness), "subscribe-pose"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    time.sleep(0.4)
    if sub.poll() is not None:
        stdout, _ = sub.communicate()
        raise AssertionError(f"go harness exited early {sub.returncode}: {stdout}")
    subprocess.check_call([str(cpp_harness), "publish-pose"], env=env, timeout=5)
    stdout, _ = sub.communicate(timeout=5)
    assert sub.returncode == 0, stdout
    assert "got pan=" in stdout


@pytest.mark.asyncio
async def test_go_to_python_event(nats_url: str, go_harness) -> None:
    received = asyncio.Event()
    app = App("py-sub", AppConfig(name="py-sub", transport_url=nats_url))

    @app.subscribe(PoseChanged)
    async def on_event(event: PoseChanged) -> None:
        assert abs(event.pose.pan - 1.5) < 1e-5
        received.set()

    await app.start()
    proc = subprocess.Popen([str(go_harness), "publish-pose"], env=harness_env(nats_url))
    try:
        await asyncio.wait_for(received.wait(), timeout=5)
    finally:
        proc.wait(timeout=5)
        await app.shutdown()
