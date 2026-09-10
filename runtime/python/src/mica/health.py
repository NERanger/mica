from __future__ import annotations

from dataclasses import dataclass

from mica.errors import RpcCode


class State:
    UNSPECIFIED = "UNSPECIFIED"
    STARTING = "STARTING"
    RUNNING = "RUNNING"
    STOPPING = "STOPPING"
    STOPPED = "STOPPED"
    FAILED = "FAILED"


@dataclass
class Health:
    state: str
    message: str
    uptime_ms: int


def rpc_code_name(code: RpcCode) -> str:
    return RpcCode(code).name
