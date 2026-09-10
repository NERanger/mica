#pragma once

#include <chrono>
#include <string>

namespace mica {

inline constexpr int kProtocolVersion = 1;
inline constexpr const char* kDefaultTransportUrl = "nats://127.0.0.1:4222";

struct AppConfig {
  std::string name;
  std::string transport_url = kDefaultTransportUrl;
  std::chrono::milliseconds rpc_timeout{5000};

  static AppConfig from_env(std::string name);
};

}  // namespace mica
