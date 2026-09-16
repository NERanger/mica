from __future__ import annotations

import asyncio
import time
from pathlib import Path

from audit.v1.audit_pb2 import JobRecorded
from jobs.v1.jobs_pb2 import JobCompleted, RunRequest, RunResponse
from mica import App, AppConfig, RpcCode, RpcError
from mica_tokens import Worker

from tests.conftest import harness_env
from tests.load.harness import run_report_async, start, stop, wait_report
from tests.load.profile import Profile
from tests.load.stats import assert_counts, assert_p99, summarize


class HopCollector:
    def __init__(self) -> None:
        self.samples: list[int] = []

    def add(self, timestamp_ns: int) -> None:
        now = time.time_ns()
        self.samples.append(now - timestamp_ns if now >= timestamp_ns else 0)

    @property
    def received(self) -> int:
        return len(self.samples)


async def wait_count(get_n, expected: int, timeout_s: float) -> None:
    deadline = time.monotonic() + timeout_s
    while get_n() < expected:
        if time.monotonic() >= deadline:
            return
        await asyncio.sleep(0.005)


async def publish_events(app: App, count: int, rate: int) -> None:
    interval = 1.0 / rate
    next_t = time.perf_counter()
    for i in range(count):
        event = JobCompleted()
        event.job_id.value = str(i)
        event.output = f"done:{i}"
        event.timestamp_ns = time.time_ns()
        await app.publish(event)
        next_t += interval
        delay = next_t - time.perf_counter()
        if delay > 0:
            await asyncio.sleep(delay)


async def call_rpc(
    app: App, count: int, rate: int, concurrency: int, timeout_s: float
) -> tuple[list[int], int, int, int]:
    sem = asyncio.Semaphore(concurrency)
    interval = 1.0 / rate
    origin = time.perf_counter()

    async def one(i: int) -> tuple[int | None, RpcCode | None]:
        delay = (origin + i * interval) - time.perf_counter()
        if delay > 0:
            await asyncio.sleep(delay)
        async with sem:
            req = RunRequest()
            req.job_id.value = str(i)
            req.input = "task"
            t0 = time.perf_counter_ns()
            try:
                await app.call(Worker.Run, req, timeout=timeout_s)
                return time.perf_counter_ns() - t0, None
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
    return samples, errors, timeouts, unavailable


def _bin(name: str, cpp_harness: Path | None, go_harness: Path | None) -> Path:
    if name == "cpp":
        assert cpp_harness is not None
        return cpp_harness
    assert go_harness is not None
    return go_harness


async def run_events(
    profile: Profile,
    pub: str,
    sub: str,
    nats_url: str,
    cpp_harness: Path | None,
    go_harness: Path | None,
) -> None:
    env = harness_env(nats_url)
    count = profile.event_count
    wait_s = profile.wait_s
    sub_proc = None
    sub_app = None
    collector = HopCollector()
    try:
        if sub == "python":
            sub_app = App("py-sub", AppConfig(name="py-sub", transport_url=nats_url))

            @sub_app.subscribe(JobCompleted)
            async def on_event(event: JobCompleted) -> None:
                collector.add(event.timestamp_ns)

            await sub_app.start()
            await asyncio.sleep(0.4)
        else:
            sub_proc = start(
                _bin(sub, cpp_harness, go_harness),
                ["subscribe-load", str(count), str(wait_s)],
                env,
            )
            await asyncio.sleep(0.4)

        if pub == "python":
            pub_app = App("py-pub", AppConfig(name="py-pub", transport_url=nats_url))
            await pub_app.start()
            try:
                await publish_events(pub_app, count, profile.event_rate)
            finally:
                await pub_app.shutdown()
        else:
            report_pub = await run_report_async(
                _bin(pub, cpp_harness, go_harness),
                ["publish-load", str(count), str(profile.event_rate)],
                env,
                wait_s + 5,
            )
            assert_counts(received=report_pub["sent"], expected=count, errors=report_pub["errors"])

        if sub == "python":
            await wait_count(lambda: collector.received, count, profile.drain_s)
            assert sub_app is not None
            await sub_app.shutdown()
            sub_app = None
            assert_counts(received=collector.received, expected=count)
            summary = summarize(collector.samples, profile.warmup)
            assert_p99(summary["p99_ns"], profile.p99_event_ms, f"event {pub}->{sub}")
        else:
            assert sub_proc is not None
            report = wait_report(sub_proc, wait_s + 5)
            sub_proc = None
            assert_counts(received=report["received"], expected=count, errors=report["errors"])
            assert_p99(report["p99_ns"], profile.p99_event_ms, f"event {pub}->{sub}")
    finally:
        stop(sub_proc)
        if sub_app is not None:
            await sub_app.shutdown()


async def run_rpc(
    profile: Profile,
    client: str,
    server: str,
    nats_url: str,
    cpp_harness: Path | None,
    go_harness: Path | None,
) -> None:
    env = harness_env(nats_url)
    count = profile.rpc_count
    wait_s = profile.wait_s
    server_proc = None
    server_app = None
    try:
        if server == "python":
            server_app = App("py-server", AppConfig(name="py-server", transport_url=nats_url))

            @server_app.serve(Worker.Run)
            async def run_job(request: RunRequest) -> RunResponse:
                resp = RunResponse()
                resp.accepted = True
                return resp

            await server_app.start()
            await asyncio.sleep(0.4)
        else:
            server_proc = start(_bin(server, cpp_harness, go_harness), ["serve-run", "ok"], env)
            await asyncio.sleep(0.4)

        if client == "python":
            client_app = App("py-client", AppConfig(name="py-client", transport_url=nats_url))
            await client_app.start()
            try:
                samples, errors, timeouts, unavailable = await call_rpc(
                    client_app,
                    count,
                    profile.rpc_rate,
                    profile.rpc_concurrency,
                    profile.rpc_timeout_s,
                )
            finally:
                await client_app.shutdown()
            assert_counts(
                received=len(samples),
                expected=count,
                errors=errors,
                timeouts=timeouts,
                unavailable=unavailable,
            )
            summary = summarize(samples, profile.warmup)
            assert_p99(summary["p99_ns"], profile.p99_rpc_ms, f"rpc {client}->{server}")
        else:
            report = await run_report_async(
                _bin(client, cpp_harness, go_harness),
                [
                    "call-load",
                    str(count),
                    str(profile.rpc_rate),
                    str(profile.rpc_concurrency),
                ],
                env,
                wait_s + 5,
            )
            assert_counts(
                received=report["ok"],
                expected=count,
                errors=report["errors"],
                timeouts=report["timeouts"],
                unavailable=report["unavailable"],
            )
            assert_p99(report["p99_ns"], profile.p99_rpc_ms, f"rpc {client}->{server}")
    finally:
        stop(server_proc)
        if server_app is not None:
            await server_app.shutdown()
