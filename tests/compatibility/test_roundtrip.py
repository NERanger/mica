from __future__ import annotations

import subprocess
from pathlib import Path

from camera.v1.camera_pb2 import PoseChanged


def test_protobuf_roundtrip(tmp_path: Path, cpp_harness: Path, go_harness: Path) -> None:
    event = PoseChanged()
    event.camera_id.value = "cam-1"
    event.pose.pan = 1.5
    event.pose.tilt = 2.5
    event.pose.zoom = 3.5
    event.timestamp_ns = 42
    first = tmp_path / "py.bin"
    second = tmp_path / "cpp.bin"
    third = tmp_path / "go.bin"
    first.write_bytes(event.SerializeToString())
    subprocess.check_call([str(cpp_harness), "proto-roundtrip", str(first), str(second)])
    subprocess.check_call([str(go_harness), "proto-roundtrip", str(second), str(third)])
    parsed = PoseChanged()
    parsed.ParseFromString(third.read_bytes())
    assert parsed.camera_id.value == "cam-1"
    assert abs(parsed.pose.pan - 1.5) < 1e-5
    assert parsed.timestamp_ns == 42
