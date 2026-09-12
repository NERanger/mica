from __future__ import annotations

import subprocess
from pathlib import Path

from jobs.v1.jobs_pb2 import JobCompleted


def test_protobuf_roundtrip(tmp_path: Path, cpp_harness: Path, go_harness: Path) -> None:
    event = JobCompleted()
    event.job_id.value = "job-1"
    event.output = "done:task-1"
    event.timestamp_ns = 42
    first = tmp_path / "py.bin"
    second = tmp_path / "cpp.bin"
    third = tmp_path / "go.bin"
    first.write_bytes(event.SerializeToString())
    subprocess.check_call([str(cpp_harness), "proto-roundtrip", str(first), str(second)])
    subprocess.check_call([str(go_harness), "proto-roundtrip", str(second), str(third)])
    parsed = JobCompleted()
    parsed.ParseFromString(third.read_bytes())
    assert parsed.job_id.value == "job-1"
    assert parsed.output == "done:task-1"
    assert parsed.timestamp_ns == 42
