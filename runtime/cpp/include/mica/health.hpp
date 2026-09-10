#pragma once

#include <cstdint>
#include <string>

namespace mica {

enum class State {
  Unspecified,
  Starting,
  Running,
  Stopping,
  Stopped,
  Failed,
};

struct Health {
  State state = State::Stopped;
  std::string message;
  std::uint64_t uptime_ms = 0;
};

const char* state_name(State state);

}  // namespace mica
