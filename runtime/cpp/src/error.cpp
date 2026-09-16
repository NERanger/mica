#include "mica/error.hpp"

namespace mica {

const char* rpc_code_name(RpcCode code) {
  switch (code) {
    case RpcCode::Unspecified:
      return "UNSPECIFIED";
    case RpcCode::Ok:
      return "OK";
    case RpcCode::InvalidArgument:
      return "INVALID_ARGUMENT";
    case RpcCode::NotFound:
      return "NOT_FOUND";
    case RpcCode::Unavailable:
      return "UNAVAILABLE";
    case RpcCode::Internal:
      return "INTERNAL";
  }
  return "UNSPECIFIED";
}

std::string RpcError::format(RpcCode code, const std::string& message) {
  std::string out = rpc_code_name(code);
  if (!message.empty()) {
    out += ": ";
    out += message;
  }
  return out;
}

}  // namespace mica
