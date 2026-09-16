from __future__ import annotations

from enum import IntEnum


class RpcCode(IntEnum):
    UNSPECIFIED = 0
    OK = 1
    INVALID_ARGUMENT = 2
    NOT_FOUND = 3
    UNAVAILABLE = 4
    INTERNAL = 5


def is_wire_error(code: RpcCode) -> bool:
    return code in (
        RpcCode.INVALID_ARGUMENT,
        RpcCode.NOT_FOUND,
        RpcCode.UNAVAILABLE,
        RpcCode.INTERNAL,
    )


class MicaError(Exception):
    pass


class TransportError(MicaError):
    pass


class ProtocolError(MicaError):
    pass


class RpcError(MicaError):
    def __init__(self, code: RpcCode, message: str = "") -> None:
        self.code = RpcCode(code)
        self.message = message
        super().__init__(f"{self.code.name}: {message}" if message else self.code.name)


class CallTimeout(MicaError):
    def __init__(self, message: str = "rpc timed out") -> None:
        self.message = message
        super().__init__(message)


class CallCancelled(MicaError):
    def __init__(self, message: str = "rpc cancelled") -> None:
        self.message = message
        super().__init__(message)
