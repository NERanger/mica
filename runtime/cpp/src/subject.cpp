#include "mica/subject.hpp"

#include <stdexcept>

namespace mica {
namespace {

std::string_view strip_dot(std::string_view name) {
  if (!name.empty() && name.front() == '.') {
    return name.substr(1);
  }
  return name;
}

}  // namespace

std::string normalize_full_name(std::string_view name) {
  return std::string(strip_dot(name));
}

std::string event_subject(std::string_view message_full_name) {
  return "event." + normalize_full_name(message_full_name);
}

std::string rpc_subject(std::string_view service_full_name, std::string_view method) {
  return "rpc." + normalize_full_name(service_full_name) + "." + std::string(method);
}

void split_rpc_contract(std::string_view contract_id, std::string& service, std::string& method) {
  auto name = strip_dot(contract_id);
  auto pos = name.rfind('.');
  if (pos == std::string_view::npos) {
    throw std::invalid_argument("invalid rpc contract id");
  }
  service = std::string(name.substr(0, pos));
  method = std::string(name.substr(pos + 1));
}

std::string rpc_subject_from_contract(std::string_view contract_id) {
  std::string service;
  std::string method;
  split_rpc_contract(contract_id, service, method);
  return rpc_subject(service, method);
}

}  // namespace mica
