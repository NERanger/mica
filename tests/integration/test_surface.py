from __future__ import annotations

import asyncio
import json
import subprocess

from jobs.v1.jobs_pb2 import JobCompleted, RunRequest, RunResponse
from mica import App, AppConfig
from mica_tokens import Worker
from tests.conftest import harness_env


async def test_python_surface(nats_url: str, tmp_path) -> None:
    surface_file = tmp_path / "client.json"
    app = App(
        "client",
        AppConfig(name="client", transport_url=nats_url, surface_file=str(surface_file)),
    )

    @app.subscribe(JobCompleted)
    async def on_event(event: JobCompleted) -> None:
        pass

    @app.serve(Worker.Run)
    async def run_job(request: RunRequest) -> RunResponse:
        return RunResponse(accepted=True)

    await app.start()
    report = json.loads(surface_file.read_text())
    assert report["component"] == "client"
    assert report["subscribes"] == ["jobs.v1.JobCompleted"]
    assert report["provides"] == ["jobs.v1.Worker.Run"]
    assert report["publishes"] == []
    assert report["calls"] == []

    event = JobCompleted()
    event.job_id.value = "job-1"
    await app.publish(event)

    request = RunRequest()
    request.job_id.value = "job-1"
    response = await app.call(Worker.Run, request, timeout=2)
    assert response.accepted

    await app.shutdown()
    report = json.loads(surface_file.read_text())
    assert report["publishes"] == ["jobs.v1.JobCompleted"]
    assert report["calls"] == ["jobs.v1.Worker.Run"]


async def test_cpp_surface(nats_url: str, cpp_harness, tmp_path) -> None:
    surface_file = tmp_path / "cpp.json"
    env = harness_env(nats_url)
    env["MICA_SURFACE_FILE"] = str(surface_file)
    sub = subprocess.Popen(
        [str(cpp_harness), "subscribe-completed"],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    await asyncio.sleep(0.4)
    app = App("py-pub", AppConfig(name="py-pub", transport_url=nats_url))
    await app.start()
    event = JobCompleted()
    event.job_id.value = "job-1"
    event.output = "done:task-1"
    await app.publish(event)
    await app.shutdown()
    try:
        stdout, _ = sub.communicate(timeout=5)
    except subprocess.TimeoutExpired:
        sub.kill()
        raise
    assert sub.returncode == 0, stdout
    report = json.loads(surface_file.read_text())
    assert report["subscribes"] == ["jobs.v1.JobCompleted"]


def test_go_surface(nats_url: str, go_harness, tmp_path) -> None:
    surface_file = tmp_path / "go.json"
    env = harness_env(nats_url)
    env["MICA_SURFACE_FILE"] = str(surface_file)
    subprocess.check_call(
        [str(go_harness), "publish-completed"], env=env, timeout=10
    )
    report = json.loads(surface_file.read_text())
    assert report["publishes"] == ["jobs.v1.JobCompleted"]
    assert report["subscribes"] == []
