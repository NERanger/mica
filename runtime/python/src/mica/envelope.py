from __future__ import annotations

import time
import uuid

from mica.config import PROTOCOL_VERSION
from mica.errors import ProtocolError, RpcCode, RpcError, is_wire_error

try:
    from mica import runtime_pb2
except ImportError as exc:  # pragma: no cover - generated during scripts/generate
    runtime_pb2 = None
    _IMPORT_ERROR = exc
else:
    _IMPORT_ERROR = None


def _pb() -> object:
    if runtime_pb2 is None:
        raise ProtocolError(
            "mica runtime protobuf is missing; run ./scripts/generate"
        ) from _IMPORT_ERROR
    return runtime_pb2


def encode(
    payload: bytes,
    *,
    contract_id: str,
    sender_id: str,
    code: RpcCode | None = None,
    message: str = "",
) -> bytes:
    pb = _pb()
    env = pb.Envelope()
    env.protocol_version = PROTOCOL_VERSION
    env.message_id = str(uuid.uuid4())
    env.sender_id = sender_id
    env.timestamp_ns = time.time_ns()
    env.contract_id = contract_id
    env.payload = payload
    if code is not None:
        env.status.code = int(code)
        env.status.message = message
    return env.SerializeToString()


def decode(data: bytes):
    pb = _pb()
    env = pb.Envelope()
    try:
        env.ParseFromString(data)
    except Exception as exc:
        raise ProtocolError("malformed envelope") from exc
    if env.protocol_version != PROTOCOL_VERSION:
        raise ProtocolError(
            f"unsupported mica protocol version {env.protocol_version}"
        )
    return env


def status_error(env) -> RpcError | None:
    if not env.HasField("status"):
        return None
    try:
        code = RpcCode(env.status.code)
    except ValueError as exc:
        raise ProtocolError("invalid rpc status") from exc
    if code in (RpcCode.UNSPECIFIED, RpcCode.OK):
        return None
    if not is_wire_error(code):
        raise ProtocolError("invalid rpc status")
    return RpcError(code, env.status.message)
