from __future__ import annotations

from pathlib import Path

from mica.cli.main import main
from tests.conftest import ROOT

DEMO = ROOT / "examples" / "demo" / "app.toml"


def test_validate_demo(capsys) -> None:
    assert main(["validate", str(DEMO)]) == 0
    out = capsys.readouterr()
    assert "ok:" in out.out


def test_inspect_demo(capsys) -> None:
    assert main(["inspect", str(DEMO)]) == 0
    out = capsys.readouterr().out
    assert "Application: demo" in out
    assert "camera-control [C++]" in out
    assert "tracker [Python]" in out
    assert "planner [Go]" in out
    assert "camera.v1.CameraControl.SetPose" in out


def test_inspect_json(capsys) -> None:
    assert main(["inspect", str(DEMO), "--format", "json"]) == 0
    out = capsys.readouterr().out
    assert '"application": "demo"' in out


def test_graph_ascii(capsys) -> None:
    assert main(["graph", str(DEMO)]) == 0
    out = capsys.readouterr().out
    assert "camera-control" in out
    assert "PoseChanged" in out


def test_graph_dot(capsys) -> None:
    assert main(["graph", str(DEMO), "--format", "dot"]) == 0
    out = capsys.readouterr().out
    assert "digraph" in out
