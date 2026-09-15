#include "mica/config.hpp"

#include <cstdlib>
#include <string>

namespace mica {

AppConfig AppConfig::from_env(std::string name) {
  AppConfig config;
  if (const char* component = std::getenv("MICA_COMPONENT_NAME");
      component != nullptr && component[0] != '\0') {
    config.name = component;
  } else {
    config.name = std::move(name);
  }
  if (const char* url = std::getenv("MICA_NATS_URL"); url != nullptr && url[0] != '\0') {
    config.transport_url = url;
  }
  if (const char* timeout = std::getenv("MICA_RPC_TIMEOUT_MS");
      timeout != nullptr && timeout[0] != '\0') {
    config.rpc_timeout = std::chrono::milliseconds{std::stoll(timeout)};
  }
  if (const char* surface = std::getenv("MICA_SURFACE_FILE");
      surface != nullptr && surface[0] != '\0') {
    config.surface_file = surface;
  }
  return config;
}

}  // namespace mica
