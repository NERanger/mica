#pragma once

#include <string>
#include <string_view>

namespace mica {

template <typename Request, typename Response>
struct RpcMethod {
  using RequestType = Request;
  using ResponseType = Response;

  std::string_view service;
  std::string_view method;

  std::string contract_id() const {
    return std::string(service) + "." + std::string(method);
  }
};

}  // namespace mica
