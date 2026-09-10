#include "mica/health.hpp"

namespace mica {

const char* state_name(State state) {
  switch (state) {
    case State::Unspecified:
      return "UNSPECIFIED";
    case State::Starting:
      return "STARTING";
    case State::Running:
      return "RUNNING";
    case State::Stopping:
      return "STOPPING";
    case State::Stopped:
      return "STOPPED";
    case State::Failed:
      return "FAILED";
  }
  return "UNSPECIFIED";
}

}  // namespace mica
