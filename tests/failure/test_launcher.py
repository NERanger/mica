from __future__ import annotations

import os
import signal
import subprocess
import time
from pathlib import Path

from tests.conftest import ROOT, TOOLS


def test_child_exit(tmp_path: Path, nats_url: str) -> None:
    component = tmp_path / "component.toml"
    component.write_text(
        """
[component]
name = "boom"
language = "python"
publishes = []
subscribes = []
calls = []
provides = []
""",
        encoding="utf-8",
    )
    app = tmp_path / "app.toml"
    app.write_text(
        f"""
[app]
name = "fail"

[transport]
kind = "nats"
url = "{nats_url}"

[[process]]
name = "boom"
command = "python3"
args = ["-c", "raise SystemExit(3)"]
component = "./component.toml"
restart = "never"
""",
        encoding="utf-8",
    )
    env = os.environ.copy()
    env["PATH"] = str(TOOLS) + os.pathsep + env.get("PATH", "")
    env["PYTHONPATH"] = os.pathsep.join(
        [
            str(ROOT / "runtime" / "python" / "src"),
            str(ROOT / "generated" / "python"),
            env.get("PYTHONPATH", ""),
        ]
    )
    proc = subprocess.run(
        ["python3", "-m", "mica.cli.main", "run", str(app)],
        env=env,
        cwd=str(ROOT),
        capture_output=True,
        text=True,
        timeout=10,
    )
    assert proc.returncode != 0


def test_sigint_shutdown(tmp_path: Path, nats_url: str) -> None:
    component = tmp_path / "component.toml"
    component.write_text(
        """
[component]
name = "sleep"
language = "python"
publishes = []
subscribes = []
calls = []
provides = []
""",
        encoding="utf-8",
    )
    app = tmp_path / "app.toml"
    app.write_text(
        f"""
[app]
name = "sig"
shutdown_timeout_ms = 3000

[transport]
kind = "nats"
url = "{nats_url}"

[[process]]
name = "sleep"
command = "python3"
args = ["-c", "import time; time.sleep(30)"]
component = "./component.toml"
restart = "never"
""",
        encoding="utf-8",
    )
    env = os.environ.copy()
    env["PATH"] = str(TOOLS) + os.pathsep + env.get("PATH", "")
    env["PYTHONPATH"] = os.pathsep.join(
        [
            str(ROOT / "runtime" / "python" / "src"),
            str(ROOT / "generated" / "python"),
            env.get("PYTHONPATH", ""),
        ]
    )
    proc = subprocess.Popen(
        ["python3", "-m", "mica.cli.main", "run", str(app)],
        env=env,
        cwd=str(ROOT),
        start_new_session=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    time.sleep(0.5)
    os.kill(proc.pid, signal.SIGINT)
    try:
        proc.wait(timeout=8)
    except subprocess.TimeoutExpired:
        os.killpg(proc.pid, signal.SIGKILL)
        raise
    assert proc.returncode is not None
