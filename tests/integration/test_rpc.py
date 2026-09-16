from __future__ import annotations

import asyncio
import subprocess
import time

import pytest
from jobs.v1.jobs_pb2 import RunRequest
from mica import App, AppConfig, CallCancelled, CallTimeout, RpcError, RpcCode
from mica_tokens import Worker
from tests.conftest import harness_env


@pytest.mark.asyncio
async def test_python_client_cpp_server(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(cpp_harness), "serve-run", "ok"], env=env)
    try:
        await asyncio.sleep(0.4)
        app = App("py-client", AppConfig(name="py-client", transport_url=nats_url))
        await app.start()
        req = RunRequest()
        req.job_id.value = "job-1"
        req.input = "task-1"
        resp = await app.call(Worker.Run, req, timeout=2)
        assert resp.accepted is True
        await app.shutdown()
    finally:
        server.terminate()
        server.wait(timeout=5)


def test_cpp_client_go_server(nats_url: str, cpp_harness, go_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(go_harness), "serve-run", "ok"], env=env)
    time.sleep(0.4)
    try:
        out = subprocess.check_output([str(cpp_harness), "call-run"], env=env, text=True, timeout=5)
        assert "accepted=1" in out or "accepted=true" in out.lower()
    finally:
        server.terminate()
        server.wait(timeout=5)


@pytest.mark.asyncio
async def test_go_client_python_server(nats_url: str, go_harness) -> None:
    app = App("py-server", AppConfig(name="py-server", transport_url=nats_url))

    @app.serve(Worker.Run)
    async def run_job(request: RunRequest):
        from jobs.v1.jobs_pb2 import RunResponse

        resp = RunResponse()
        resp.accepted = True
        return resp

    await app.start()
    proc = subprocess.Popen(
        [str(go_harness), "call-run"],
        env=harness_env(nats_url),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    try:
        stdout, _ = await asyncio.to_thread(proc.communicate, timeout=5)
        assert proc.returncode == 0
        assert "accepted=" in stdout
    finally:
        await app.shutdown()


@pytest.mark.asyncio
async def test_rpc_invalid_argument(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(cpp_harness), "serve-run", "invalid"], env=env)
    try:
        await asyncio.sleep(0.4)
        app = App("py-client", AppConfig(name="py-client", transport_url=nats_url))
        await app.start()
        req = RunRequest()
        req.job_id.value = "job-1"
        req.input = "task-1"
        with pytest.raises(RpcError) as raised:
            await app.call(Worker.Run, req, timeout=2)
        assert raised.value.code == RpcCode.INVALID_ARGUMENT
        await app.shutdown()
    finally:
        server.terminate()
        server.wait(timeout=5)


@pytest.mark.asyncio
async def test_rpc_timeout(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(cpp_harness), "serve-run", "sleep"], env=env)
    try:
        await asyncio.sleep(0.4)
        app = App("py-client", AppConfig(name="py-client", transport_url=nats_url))
        await app.start()
        req = RunRequest()
        req.job_id.value = "job-1"
        req.input = "task-1"
        with pytest.raises(CallTimeout):
            await app.call(Worker.Run, req, timeout=0.3)
        await app.shutdown()
    finally:
        server.terminate()
        server.wait(timeout=5)


@pytest.mark.asyncio
async def test_rpc_cancelled(nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    server = subprocess.Popen([str(cpp_harness), "serve-run", "sleep"], env=env)
    try:
        await asyncio.sleep(0.4)
        app = App("py-client", AppConfig(name="py-client", transport_url=nats_url))
        await app.start()
        req = RunRequest()
        req.job_id.value = "job-1"
        req.input = "task-1"
        task = asyncio.create_task(app.call(Worker.Run, req, timeout=5))
        await asyncio.sleep(0.05)
        task.cancel()
        with pytest.raises(CallCancelled):
            await task
        await app.shutdown()
    finally:
        server.terminate()
        server.wait(timeout=5)
