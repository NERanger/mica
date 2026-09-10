from __future__ import annotations

from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True)
class RpcMethod:
    service: str
    method: str
    request_type: type
    response_type: type

    @property
    def contract_id(self) -> str:
        return f"{self.service}.{self.method}"
