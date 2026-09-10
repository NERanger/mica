from __future__ import annotations

import os
from dataclasses import dataclass

DEFAULT_TRANSPORT_URL = "nats://127.0.0.1:4222"
DEFAULT_RPC_TIMEOUT_S = 5.0
PROTOCOL_VERSION = 1


def _env(name: str, default: str) -> str:
    value = os.environ.get(name)
    return default if value is None or value == "" else value


@dataclass
class AppConfig:
    name: str
    transport_url: str = DEFAULT_TRANSPORT_URL
    rpc_timeout_s: float = DEFAULT_RPC_TIMEOUT_S

    @classmethod
    def from_env(cls, name: str) -> AppConfig:
        timeout_raw = os.environ.get("MICA_RPC_TIMEOUT_S")
        timeout = DEFAULT_RPC_TIMEOUT_S if not timeout_raw else float(timeout_raw)
        return cls(
            name=_env("MICA_COMPONENT_NAME", name),
            transport_url=_env("MICA_NATS_URL", DEFAULT_TRANSPORT_URL),
            rpc_timeout_s=timeout,
        )
