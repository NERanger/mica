def normalize_full_name(name: str) -> str:
    return name.lstrip(".")


def event_subject(message_full_name: str) -> str:
    return "event." + normalize_full_name(message_full_name)


def rpc_subject(service_full_name: str, method: str) -> str:
    return "rpc." + normalize_full_name(service_full_name) + "." + method


def rpc_subject_from_contract(contract_id: str) -> str:
    service, method = split_rpc_contract(contract_id)
    return rpc_subject(service, method)


def split_rpc_contract(contract_id: str) -> tuple[str, str]:
    name = normalize_full_name(contract_id)
    service, method = name.rsplit(".", 1)
    return service, method
