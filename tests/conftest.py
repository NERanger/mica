from __future__ import annotations

import os
import shutil
import socket
import subprocess
import time
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
TOOLS = ROOT / ".tools" / "bin"


def _which(name: str) -> str | None:
    env_path = os.environ.get("PATH", "")
    path = os.pathsep.join([str(TOOLS), env_path])
    return shutil.which(name, path=path)


def free_port() -> int:
    sock = socket.socket()
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return port


@pytest.fixture(scope="session")
def repo_root() -> Path:
    return ROOT


@pytest.fixture(scope="session")
def nats_server_bin() -> str:
    binary = _which("nats-server")
    if binary is None:
        pytest.skip("nats-server not available")
    return binary


@pytest.fixture
def nats_url(nats_server_bin: str) -> str:
    port = free_port()
    proc = subprocess.Popen(
        [nats_server_bin, "-a", "127.0.0.1", "-p", str(port)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    url = f"nats://127.0.0.1:{port}"
    try:
        for _ in range(50):
            if proc.poll() is not None:
                pytest.fail("nats-server exited")
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.1):
                    break
            except OSError:
                time.sleep(0.05)
        else:
            pytest.fail("nats-server did not start")
        yield url
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            proc.kill()


@pytest.fixture(scope="session")
def cpp_harness() -> Path:
    path = ROOT / "build" / "bin" / "mica-cpp-harness"
    if not path.is_file():
        pytest.skip("C++ harness not built")
    return path


@pytest.fixture(scope="session")
def go_harness() -> Path:
    path = ROOT / "build" / "bin" / "mica-go-harness"
    if not path.is_file():
        pytest.skip("Go harness not built")
    return path


def harness_env(nats_url: str) -> dict[str, str]:
    env = os.environ.copy()
    env["MICA_NATS_URL"] = nats_url
    env["PATH"] = str(TOOLS) + os.pathsep + env.get("PATH", "")
    env["PYTHONPATH"] = os.pathsep.join(
        [
            str(ROOT / "runtime" / "python" / "src"),
            str(ROOT / "examples" / "demo" / "generated" / "python"),
            env.get("PYTHONPATH", ""),
        ]
    )
    return env
