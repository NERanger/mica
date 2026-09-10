#pragma once

#include <string>
#include <string_view>

namespace mica {

std::string normalize_full_name(std::string_view name);
std::string event_subject(std::string_view message_full_name);
std::string rpc_subject(std::string_view service_full_name, std::string_view method);
std::string rpc_subject_from_contract(std::string_view contract_id);
void split_rpc_contract(std::string_view contract_id, std::string& service, std::string& method);

}  // namespace mica
