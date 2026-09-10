#pragma once

#include <stdexcept>
#include <string>

namespace mica {

enum class RpcCode {
  Unspecified = 0,
  Ok = 1,
  InvalidArgument = 2,
  NotFound = 3,
  Unavailable = 4,
  Internal = 5,
  Timeout = 6,
  Cancelled = 7,
};

class MicaError : public std::runtime_error {
 public:
  explicit MicaError(const std::string& message) : std::runtime_error(message) {}
};

class TransportError : public MicaError {
 public:
  explicit TransportError(const std::string& message) : MicaError(message) {}
};

class ProtocolError : public MicaError {
 public:
  explicit ProtocolError(const std::string& message) : MicaError(message) {}
};

class RpcError : public MicaError {
 public:
  RpcError(RpcCode code, std::string message)
      : MicaError(format(code, message)), code_(code), message_(std::move(message)) {}

  RpcCode code() const { return code_; }
  const std::string& rpc_message() const { return message_; }

 private:
  static std::string format(RpcCode code, const std::string& message);

  RpcCode code_;
  std::string message_;
};

const char* rpc_code_name(RpcCode code);

}  // namespace mica
