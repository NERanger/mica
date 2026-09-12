// The C++ worker: serves one RPC, publishes one event.
//
// It knows nothing about who calls it or who listens. Its only protocol
// dependency is generated contract code (jobs/v1/jobs.pb.h) and the MICA
// C++ runtime.
#include <chrono>
#include <cstdint>
#include <iostream>
#include <mutex>

#include "jobs/v1/jobs.pb.h"
#include "mica/app.hpp"
#include "mica/tokens.hpp"

int main() {
  mica::App app{"worker"};
  std::mutex mu;
  std::string last_output;

  // serve() registers one RPC handler. The token Worker_Run comes from
  // generated code; there is no subject string in application code.
  app.serve(mica::tokens::Worker_Run, [&](const jobs::v1::RunRequest& request) {
    if (!request.has_job_id() || request.job_id().value().empty() || request.input().empty()) {
      // Throwing RpcError returns a typed error to the caller.
      throw mica::RpcError(mica::RpcCode::InvalidArgument, "job_id and input are required");
    }
    // Handlers may run concurrently; guard component-local state.
    jobs::v1::JobCompleted completed;
    {
      std::lock_guard<std::mutex> lock(mu);
      last_output = "done:" + request.input();
      completed.mutable_job_id()->CopyFrom(request.job_id());
      completed.set_output(last_output);
    }
    const auto ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                        std::chrono::system_clock::now().time_since_epoch())
                        .count();
    completed.set_timestamp_ns(static_cast<std::uint64_t>(ns));
    std::cout << "Run accepted input=" << request.input() << std::endl;
    // An RPC handler can publish events as part of its work.
    // Fire-and-forget: any number of subscribers may react, or none.
    app.publish(completed);
    jobs::v1::RunResponse response;
    response.set_accepted(true);
    return response;
  });

  app.run();
  return 0;
}
