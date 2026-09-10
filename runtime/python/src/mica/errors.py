from __future__ import annotations

from enum import IntEnum


class RpcCode(IntEnum):
    UNSPECIFIED = 0
    OK = 1
    INVALID_ARGUMENT = 2
    NOT_FOUND = 3
    UNAVAILABLE = 4
    INTERNAL = 5
    TIMEOUT = 6
    CANCELLED = 7


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
