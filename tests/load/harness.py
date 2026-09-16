from __future__ import annotations

import asyncio
import json
import subprocess
from collections.abc import Sequence
from pathlib import Path

PREFIX = "MICA_LOAD "


def start(binary: Path, args: Sequence[str], env: dict[str, str]) -> subprocess.Popen[str]:
    return subprocess.Popen(
        [str(binary), *args],
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )


def parse_report(stdout: str) -> dict | None:
    for line in stdout.splitlines():
        if line.startswith(PREFIX):
            return json.loads(line[len(PREFIX) :])
    return None


def wait_report(proc: subprocess.Popen[str], timeout: float) -> dict:
    try:
        stdout, stderr = proc.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        proc.kill()
        stdout, stderr = proc.communicate()
        raise AssertionError(f"harness timed out stdout={stdout} stderr={stderr}") from None
    report = parse_report(stdout)
    if report is None:
        raise AssertionError(
            f"no MICA_LOAD line rc={proc.returncode} stdout={stdout} stderr={stderr}"
        )
    return report


def run_report(binary: Path, args: Sequence[str], env: dict[str, str], timeout: float) -> dict:
    return wait_report(start(binary, args, env), timeout)


async def run_report_async(
    binary: Path, args: Sequence[str], env: dict[str, str], timeout: float
) -> dict:
    return await asyncio.to_thread(run_report, binary, args, env, timeout)


def stop(proc: subprocess.Popen[str] | None) -> None:
    if proc is None or proc.poll() is not None:
        return
    proc.terminate()
    try:
        proc.wait(timeout=2)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=2)
