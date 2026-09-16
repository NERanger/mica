from __future__ import annotations

import asyncio
import time

import pytest
from audit.v1.audit_pb2 import JobRecorded
from jobs.v1.jobs_pb2 import JobCompleted, RunRequest
from mica import App, AppConfig, RpcCode, RpcError
from mica_tokens import Worker

from tests.conftest import harness_env
from tests.load.harness import run_report_async, start, stop
from tests.load.profile import HEAVY, Profile, profile_params
from tests.load.stats import assert_counts, assert_p99, summarize
from tests.load.work import HopCollector, wait_count


async def _pipeline_python_client(profile: Profile, nats_url: str, cpp_harness) -> None:
    env = harness_env(nats_url)
    count = profile.rpc_count
    server = start(cpp_harness, ["serve-pipeline"], env)
    app = App("py-pipe", AppConfig(name="py-pipe", transport_url=nats_url))
    completed = HopCollector()
    recorded = HopCollector()
    pipeline: list[int] = []
    starts: dict[str, int] = {}
    try:
        await asyncio.sleep(0.4)

        @app.subscribe(JobCompleted)
        async def on_completed(event: JobCompleted) -> None:
            completed.add(event.timestamp_ns)
            rec = JobRecorded()
            rec.job_id = event.job_id.value
            rec.summary = event.output
            rec.timestamp_ns = time.time_ns()
            await app.publish(rec)

        @app.subscribe(JobRecorded)
        async def on_recorded(event: JobRecorded) -> None:
            recorded.add(event.timestamp_ns)
            t0 = starts.get(event.job_id)
            if t0 is not None:
                pipeline.append(time.time_ns() - t0)

        await app.start()

        sem = asyncio.Semaphore(profile.rpc_concurrency)
        interval = 1.0 / profile.rpc_rate
        origin = time.perf_counter()

        async def one(i: int) -> tuple[int | None, RpcCode | None]:
            delay = (origin + i * interval) - time.perf_counter()
            if delay > 0:
                await asyncio.sleep(delay)
            async with sem:
                job_id = str(i)
                req = RunRequest()
                req.job_id.value = job_id
                req.input = "task"
                starts[job_id] = time.time_ns()
                t_rpc = time.perf_counter_ns()
                try:
                    await app.call(Worker.Run, req, timeout=profile.rpc_timeout_s)
                    return time.perf_counter_ns() - t_rpc, None
                except RpcError as exc:
                    return None, exc.code

        results = await asyncio.gather(*[one(i) for i in range(count)])
        samples: list[int] = []
        errors = timeouts = unavailable = 0
        for lat, code in results:
            if code is None:
                assert lat is not None
                samples.append(lat)
            else:
                errors += 1
                if code == RpcCode.TIMEOUT:
                    timeouts += 1
                elif code == RpcCode.UNAVAILABLE:
                    unavailable += 1
        await wait_count(lambda: recorded.received, count, profile.drain_s)
        assert_counts(
            received=len(samples),
            expected=count,
            errors=errors,
            timeouts=timeouts,
            unavailable=unavailable,
        )
        assert_counts(received=completed.received, expected=count)
        assert_counts(received=recorded.received, expected=count)
        rpc = summarize(samples, profile.warmup)
        hop_c = summarize(completed.samples, profile.warmup)
        hop_r = summarize(recorded.samples, profile.warmup)
        pipe = summarize(pipeline, profile.warmup)
        assert_p99(rpc["p99_ns"], profile.p99_rpc_ms, "pipeline rpc")
        assert_p99(hop_c["p99_ns"], profile.p99_event_ms, "pipeline JobCompleted")
        assert_p99(hop_r["p99_ns"], profile.p99_event_ms, "pipeline JobRecorded")
        assert_p99(pipe["p99_ns"], profile.p99_pipeline_ms, "pipeline e2e")
    finally:
        stop(server)
        await app.shutdown()


@pytest.mark.asyncio
@pytest.mark.parametrize("profile", profile_params())
async def test_pipeline_python_client_cpp_worker(profile, nats_url, cpp_harness) -> None:
    await _pipeline_python_client(profile, nats_url, cpp_harness)


@pytest.mark.load
@pytest.mark.asyncio
async def test_pipeline_go_client_cpp_worker(nats_url, cpp_harness, go_harness) -> None:
    profile = HEAVY
    env = harness_env(nats_url)
    count = profile.rpc_count
    wait_s = profile.wait_s
    server = start(cpp_harness, ["serve-pipeline"], env)
    app = App("py-recorder", AppConfig(name="py-recorder", transport_url=nats_url))
    completed = HopCollector()
    recorded = HopCollector()
    try:
        await asyncio.sleep(0.4)

        @app.subscribe(JobCompleted)
        async def on_completed(event: JobCompleted) -> None:
            completed.add(event.timestamp_ns)
            rec = JobRecorded()
            rec.job_id = event.job_id.value
            rec.summary = event.output
            rec.timestamp_ns = time.time_ns()
            await app.publish(rec)

        @app.subscribe(JobRecorded)
        async def on_recorded(event: JobRecorded) -> None:
            recorded.add(event.timestamp_ns)

        await app.start()
        report = await run_report_async(
            go_harness,
            ["call-load", str(count), str(profile.rpc_rate), str(profile.rpc_concurrency)],
            env,
            wait_s + 5,
        )
        await wait_count(lambda: recorded.received, count, profile.drain_s)
        assert_counts(
            received=report["ok"],
            expected=count,
            errors=report["errors"],
            timeouts=report["timeouts"],
            unavailable=report["unavailable"],
        )
        assert_p99(report["p99_ns"], profile.p99_rpc_ms, "pipeline go rpc")
        assert_counts(received=completed.received, expected=count)
        assert_counts(received=recorded.received, expected=count)
        assert_p99(
            summarize(completed.samples, profile.warmup)["p99_ns"],
            profile.p99_event_ms,
            "pipeline JobCompleted",
        )
        assert_p99(
            summarize(recorded.samples, profile.warmup)["p99_ns"],
            profile.p99_event_ms,
            "pipeline JobRecorded",
        )
    finally:
        stop(server)
        await app.shutdown()
