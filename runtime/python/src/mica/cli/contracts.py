from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path

from google.protobuf.descriptor_pb2 import FileDescriptorSet


class ContractCatalogError(Exception):
    pass


@dataclass
class ContractCatalog:
    messages: set[str] = field(default_factory=set)
    services: set[str] = field(default_factory=set)
    methods: set[str] = field(default_factory=set)

    def is_event(self, contract_id: str) -> bool:
        return contract_id in self.messages

    def is_rpc(self, contract_id: str) -> bool:
        return contract_id in self.methods


def find_repo_root(start: Path) -> Path | None:
    for parent in [start, *start.parents]:
        if (parent / "pyproject.toml").is_file() and (parent / "buf.yaml").is_file():
            return parent
    return None


def find_image(start: Path, explicit: Path | None = None) -> Path:
    if explicit is not None:
        if not explicit.is_file():
            raise ContractCatalogError(f"descriptor image not found: {explicit}")
        return explicit
    root = find_repo_root(start.resolve())
    if root is None:
        raise ContractCatalogError("cannot locate mica repository root from " + str(start))
    image = root / "generated" / "image.binpb"
    if not image.is_file():
        raise ContractCatalogError("missing generated/image.binpb; run ./scripts/generate")
    return image


def load_catalog(image: Path) -> ContractCatalog:
    data = image.read_bytes()
    fds = FileDescriptorSet()
    try:
        fds.ParseFromString(data)
    except Exception as exc:
        raise ContractCatalogError(f"invalid descriptor image {image}: {exc}") from exc
    catalog = ContractCatalog()
    for fd in fds.file:
        package = fd.package
        for message in fd.message_type:
            catalog.messages.add(f"{package}.{message.name}" if package else message.name)
        for service in fd.service:
            service_name = f"{package}.{service.name}" if package else service.name
            catalog.services.add(service_name)
            for method in service.method:
                catalog.methods.add(f"{service_name}.{method.name}")
    return catalog
