from __future__ import annotations

import sys
from pathlib import Path

def _add_generated_path() -> None:
    here = Path(__file__).resolve()
    for parent in here.parents:
        generated = parent / "generated" / "python"
        if generated.is_dir():
            path = str(generated)
            if path not in sys.path:
                sys.path.append(path)
            break


_add_generated_path()

from mica.app import App
from mica.config import AppConfig, PROTOCOL_VERSION
from mica.errors import MicaError, ProtocolError, RpcCode, RpcError, TransportError
from mica.health import Health, State
from mica.method import RpcMethod
from mica.subject import event_subject, rpc_subject, rpc_subject_from_contract

__all__ = [
    "App",
    "AppConfig",
    "Health",
    "MicaError",
    "PROTOCOL_VERSION",
    "ProtocolError",
    "RpcCode",
    "RpcError",
    "RpcMethod",
    "State",
    "TransportError",
    "event_subject",
    "rpc_subject",
    "rpc_subject_from_contract",
]
