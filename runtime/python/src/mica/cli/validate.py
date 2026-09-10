from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from mica.cli.contracts import (
    ContractCatalog,
    ContractCatalogError,
    find_image,
    load_catalog,
)
from mica.cli.manifest import (
    AppManifest,
    ComponentManifest,
    ManifestError,
    parse_app,
    parse_component,
)


@dataclass
class LoadedProcess:
    spec_name: str
    component: ComponentManifest


@dataclass
class LoadedApp:
    app: AppManifest
    processes: list[LoadedProcess]
    catalog: ContractCatalog


@dataclass
class ValidationResult:
    errors: list[str]
    warnings: list[str]
    loaded: LoadedApp | None = None

    @property
    def ok(self) -> bool:
        return not self.errors


def load_app(app_path: Path, image: Path | None = None) -> tuple[LoadedApp | None, list[str]]:
    errors: list[str] = []
    try:
        app = parse_app(app_path)
    except ManifestError as exc:
        return None, [str(exc)]
    base = app_path.parent
    try:
        catalog = load_catalog(find_image(base, image))
    except ContractCatalogError as exc:
        return None, [str(exc)]
    processes: list[LoadedProcess] = []
    component_names: set[str] = set()
    for spec in app.processes:
        component_path = (base / spec.component).resolve()
        if not component_path.is_file():
            errors.append(f"process {spec.name!r}: component manifest not found: {spec.component}")
            continue
        try:
            component = parse_component(component_path)
        except ManifestError as exc:
            errors.append(str(exc))
            continue
        if component.name in component_names:
            errors.append(f"duplicate component name {component.name!r}")
        component_names.add(component.name)
        processes.append(LoadedProcess(spec_name=spec.name, component=component))
    if errors:
        return None, errors
    return LoadedApp(app=app, processes=processes, catalog=catalog), []


def validate_app(app_path: Path, image: Path | None = None) -> ValidationResult:
    loaded, errors = load_app(app_path, image)
    if loaded is None:
        return ValidationResult(errors=errors, warnings=[])
    warnings: list[str] = []
    catalog = loaded.catalog
    for proc in loaded.processes:
        comp = proc.component
        for contract_id in comp.publishes + comp.subscribes:
            if not catalog.is_event(contract_id):
                errors.append(
                    f"component {comp.name}: {contract_id} is not a known event message"
                )
        for contract_id in comp.calls + comp.provides:
            if not catalog.is_rpc(contract_id):
                errors.append(
                    f"component {comp.name}: {contract_id} is not a known RPC method"
                )
    publishers: dict[str, list[str]] = {}
    subscribers: dict[str, list[str]] = {}
    callers: dict[str, list[str]] = {}
    providers: dict[str, list[str]] = {}
    for proc in loaded.processes:
        name = proc.component.name
        for item in proc.component.publishes:
            publishers.setdefault(item, []).append(name)
        for item in proc.component.subscribes:
            subscribers.setdefault(item, []).append(name)
        for item in proc.component.calls:
            callers.setdefault(item, []).append(name)
        for item in proc.component.provides:
            providers.setdefault(item, []).append(name)
    for contract_id, names in publishers.items():
        if contract_id not in subscribers:
            warnings.append(
                f"event {contract_id} is published by {', '.join(names)} but has no subscriber"
            )
    for contract_id, names in subscribers.items():
        if contract_id not in publishers:
            warnings.append(
                f"event {contract_id} is subscribed by {', '.join(names)} but has no publisher"
            )
    for contract_id, names in callers.items():
        if contract_id not in providers:
            warnings.append(
                f"RPC {contract_id} is called by {', '.join(names)} but has no provider"
            )
    for contract_id, names in providers.items():
        if contract_id not in callers:
            warnings.append(
                f"RPC {contract_id} is provided by {', '.join(names)} but is never called"
            )
    return ValidationResult(errors=errors, warnings=warnings, loaded=loaded)
