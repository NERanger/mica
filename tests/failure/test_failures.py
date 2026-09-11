from __future__ import annotations

import asyncio
import subprocess
from pathlib import Path

import pytest

from camera.v1.camera_pb2 import PoseChanged, SetPoseRequest
from mica import App, AppConfig, RpcError, RpcCode, TransportError
from mica_tokens import CameraControl
from tests.conftest import harness_env


@pytest.mark.asyncio
async def test_nats_disconnected(nats_server_bin: str) -> None:
    import socket
    import time

    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    proc = subprocess.Popen(
        [nats_server_bin, "-a", "127.0.0.1", "-p", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    url = f"nats://127.0.0.1:{port}"
    try:
        for _ in range(50):
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.1):
                    break
            except OSError:
                time.sleep(0.05)
        app = App("x", AppConfig(name="x", transport_url=url))
        await app.start()
        proc.kill()
        proc.wait(timeout=2)
        await asyncio.sleep(0.2)
        event = PoseChanged()
        event.camera_id.value = "cam"
        with pytest.raises((TransportError, Exception)):
            await app.publish(event)
        await app.shutdown()
    finally:
        if proc.poll() is None:
            proc.kill()


@pytest.mark.asyncio
async def test_nats_unavailable() -> None:
    app = App("x", AppConfig(name="x", transport_url="nats://127.0.0.1:1"))
    with pytest.raises(TransportError):
        await app.start()


@pytest.mark.asyncio
async def test_malformed_event_does_not_crash(nats_url: str) -> None:
    app = App("sub", AppConfig(name="sub", transport_url=nats_url))
    called = False

    @app.subscribe(PoseChanged)
    async def on_event(event: PoseChanged) -> None:
        nonlocal called
        called = True

    await app.start()
    await app._nc.publish("event.camera.v1.PoseChanged", b"not-a-protobuf")
    await asyncio.sleep(0.3)
    assert called is False
    await app.shutdown()


@pytest.mark.asyncio
async def test_subscriber_handler_failure(nats_url: str) -> None:
    app = App("sub", AppConfig(name="sub", transport_url=nats_url))
    saw = asyncio.Event()

    @app.subscribe(PoseChanged)
    async def on_event(event: PoseChanged) -> None:
        saw.set()
        raise RuntimeError("handler boom")

    await app.start()
    event = PoseChanged()
    event.camera_id.value = "cam"
    event.pose.pan = 1
    await app.publish(event)
    await asyncio.wait_for(saw.wait(), timeout=2)
    assert app.health().state == "RUNNING"
    await app.shutdown()


@pytest.mark.asyncio
async def test_rpc_internal_error(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(cpp_harness), "serve-setpose", "internal"], env=env)
    try:
        await asyncio.sleep(0.4)
        app = App("client", AppConfig(name="client", transport_url=nats_url))
        await app.start()
        req = SetPoseRequest()
        req.camera_id.value = "cam-1"
        req.pose.pan = 1
        with pytest.raises(RpcError) as raised:
            await app.call(CameraControl.SetPose, req, timeout=2)
        assert raised.value.code == RpcCode.INTERNAL
        await app.shutdown()
    finally:
        server.terminate()
        server.wait(timeout=5)
