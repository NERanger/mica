#include <algorithm>
#include <atomic>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <cstdint>
#include <fstream>
#include <iostream>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include "audit/v1/audit.pb.h"
#include "jobs/v1/jobs.pb.h"
#include "mica/app.hpp"
#include "mica/tokens.hpp"

namespace {

constexpr int kWarmup = 100;

void usage() {
  std::cerr << "usage: mica-cpp-harness <command> [args]\n";
}

std::uint64_t now_ns() {
  auto now = std::chrono::system_clock::now().time_since_epoch();
  return static_cast<std::uint64_t>(
      std::chrono::duration_cast<std::chrono::nanoseconds>(now).count());
}

jobs::v1::JobCompleted sample_completed() {
  jobs::v1::JobCompleted event;
  event.mutable_job_id()->set_value("job-1");
  event.set_output("done:task-1");
  event.set_timestamp_ns(42);
  return event;
}

int proto_roundtrip(const std::string& in_path, const std::string& out_path) {
  std::ifstream in(in_path, std::ios::binary);
  std::string bytes((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
  jobs::v1::JobCompleted event;
  if (!event.ParseFromString(bytes)) {
    std::cerr << "parse failed\n";
    return 1;
  }
  std::ofstream out(out_path, std::ios::binary);
  std::string encoded;
  event.SerializeToString(&encoded);
  out.write(encoded.data(), static_cast<std::streamsize>(encoded.size()));
  return 0;
}

bool parse_positive(const char* text, int* out) {
  try {
    int value = std::stoi(text);
    if (value <= 0) {
      return false;
    }
    *out = value;
    return true;
  } catch (...) {
    return false;
  }
}

bool parse_positive_double(const char* text, double* out) {
  try {
    double value = std::stod(text);
    if (value <= 0) {
      return false;
    }
    *out = value;
    return true;
  } catch (...) {
    return false;
  }
}

std::uint64_t percentile_ns(std::vector<std::uint64_t> samples, double p) {
  if (samples.size() > static_cast<std::size_t>(kWarmup)) {
    samples.erase(samples.begin(), samples.begin() + kWarmup);
  }
  if (samples.empty()) {
    return 0;
  }
  std::sort(samples.begin(), samples.end());
  std::size_t k = static_cast<std::size_t>(std::ceil(p / 100.0 * static_cast<double>(samples.size())));
  if (k == 0) {
    k = 1;
  }
  --k;
  if (k >= samples.size()) {
    k = samples.size() - 1;
  }
  return samples[k];
}

std::uint64_t max_ns(std::vector<std::uint64_t> samples) {
  if (samples.size() > static_cast<std::size_t>(kWarmup)) {
    samples.erase(samples.begin(), samples.begin() + kWarmup);
  }
  if (samples.empty()) {
    return 0;
  }
  return *std::max_element(samples.begin(), samples.end());
}

int percentile_samples(int n) {
  if (n > kWarmup) {
    return n - kWarmup;
  }
  return n;
}

void print_load(const char* role, int received, int expected, int sent, int ok, int errors,
                int timeouts, int unavailable, const std::vector<std::uint64_t>& samples) {
  std::cout << "MICA_LOAD {"
            << "\"role\":\"" << role << "\""
            << ",\"received\":" << received
            << ",\"expected\":" << expected
            << ",\"sent\":" << sent
            << ",\"ok\":" << ok
            << ",\"errors\":" << errors
            << ",\"timeouts\":" << timeouts
            << ",\"unavailable\":" << unavailable
            << ",\"p50_ns\":" << percentile_ns(samples, 50)
            << ",\"p99_ns\":" << percentile_ns(samples, 99)
            << ",\"max_ns\":" << max_ns(samples)
            << ",\"percentile_samples\":" << percentile_samples(static_cast<int>(samples.size()))
            << "}\n";
}

int publish_load(mica::App& app, int count, int rate) {
  app.start();
  const auto interval = std::chrono::nanoseconds(1'000'000'000 / rate);
  auto next = std::chrono::steady_clock::now();
  int sent = 0;
  int errors = 0;
  for (int i = 0; i < count; ++i) {
    jobs::v1::JobCompleted event;
    event.mutable_job_id()->set_value(std::to_string(i));
    event.set_output("done:" + std::to_string(i));
    event.set_timestamp_ns(now_ns());
    try {
      app.publish(event);
      ++sent;
    } catch (...) {
      ++errors;
    }
    next += interval;
    std::this_thread::sleep_until(next);
  }
  std::this_thread::sleep_for(std::chrono::milliseconds(200));
  print_load("publish", 0, 0, sent, sent, errors, 0, 0, {});
  app.shutdown();
  return 0;
}

int subscribe_load(mica::App& app, int expect, double timeout_s) {
  std::mutex mu;
  std::condition_variable cv;
  std::vector<std::uint64_t> samples;
  app.subscribe<jobs::v1::JobCompleted>([&](const jobs::v1::JobCompleted& event) {
    const std::uint64_t now = now_ns();
    const std::uint64_t ts = event.timestamp_ns();
    const std::uint64_t lat = now >= ts ? now - ts : 0;
    std::lock_guard<std::mutex> lock(mu);
    samples.push_back(lat);
    if (static_cast<int>(samples.size()) >= expect) {
      cv.notify_one();
    }
  });
  app.start();
  {
    std::unique_lock<std::mutex> lock(mu);
    cv.wait_for(lock, std::chrono::duration<double>(timeout_s),
                [&] { return static_cast<int>(samples.size()) >= expect; });
  }
  std::vector<std::uint64_t> copy;
  {
    std::lock_guard<std::mutex> lock(mu);
    copy = samples;
  }
  print_load("subscribe", static_cast<int>(copy.size()), expect, 0, 0, 0, 0, 0, copy);
  app.shutdown();
  return 0;
}

int call_load(mica::App& app, int count, int rate, int concurrency) {
  app.start();
  std::mutex mu;
  std::vector<std::uint64_t> samples;
  std::atomic<int> ok{0};
  std::atomic<int> errors{0};
  std::atomic<int> timeouts{0};
  std::atomic<int> unavailable{0};
  const auto interval = std::chrono::nanoseconds(1'000'000'000 / rate);
  const auto origin = std::chrono::steady_clock::now();
  std::atomic<int> next{0};
  auto worker = [&]() {
    while (true) {
      const int i = next.fetch_add(1);
      if (i >= count) {
        return;
      }
      std::this_thread::sleep_until(origin + interval * i);
      jobs::v1::RunRequest request;
      request.mutable_job_id()->set_value(std::to_string(i));
      request.set_input("task");
      const auto t0 = std::chrono::steady_clock::now();
      try {
        (void)app.call(mica::tokens::Worker_Run, request, std::chrono::milliseconds(1000));
        const auto t1 = std::chrono::steady_clock::now();
        const auto lat = std::chrono::duration_cast<std::chrono::nanoseconds>(t1 - t0).count();
        ok.fetch_add(1);
        std::lock_guard<std::mutex> lock(mu);
        samples.push_back(static_cast<std::uint64_t>(lat));
      } catch (const mica::RpcError& err) {
        errors.fetch_add(1);
        if (err.code() == mica::RpcCode::Timeout) {
          timeouts.fetch_add(1);
        } else if (err.code() == mica::RpcCode::Unavailable) {
          unavailable.fetch_add(1);
        }
      } catch (...) {
        errors.fetch_add(1);
      }
    }
  };
  std::vector<std::thread> threads;
  threads.reserve(static_cast<std::size_t>(concurrency));
  for (int t = 0; t < concurrency; ++t) {
    threads.emplace_back(worker);
  }
  for (auto& thread : threads) {
    thread.join();
  }
  std::vector<std::uint64_t> copy;
  {
    std::lock_guard<std::mutex> lock(mu);
    copy = samples;
  }
  print_load("call", ok.load(), count, count, ok.load(), errors.load(), timeouts.load(),
             unavailable.load(), copy);
  app.shutdown();
  return 0;
}

int serve_pipeline(mica::App& app) {
  app.serve(mica::tokens::Worker_Run, [&](const jobs::v1::RunRequest& request) {
    jobs::v1::JobCompleted event;
    event.mutable_job_id()->CopyFrom(request.job_id());
    event.set_output("done:" + request.input());
    event.set_timestamp_ns(now_ns());
    app.publish(event);
    jobs::v1::RunResponse response;
    response.set_accepted(true);
    return response;
  });
  app.run();
  return 0;
}

}  // namespace

int main(int argc, char** argv) {
  if (argc < 2) {
    usage();
    return 2;
  }
  const std::string cmd = argv[1];
  if (cmd == "proto-roundtrip") {
    if (argc < 4) {
      return 2;
    }
    return proto_roundtrip(argv[2], argv[3]);
  }

  mica::AppConfig config = mica::AppConfig::from_env("cpp-harness");
  mica::App app(config.name, config);

  if (cmd == "event-pub") {
    app.run();
    return 1;
  }
  if (cmd == "publish-completed") {
    app.start();
    app.publish(sample_completed());
    std::this_thread::sleep_for(std::chrono::milliseconds(200));
    app.shutdown();
    return 0;
  }
  if (cmd == "subscribe-completed") {
    std::atomic<bool> got{false};
    app.subscribe<jobs::v1::JobCompleted>([&](const jobs::v1::JobCompleted& event) {
      std::cout << "got output=" << event.output() << std::endl;
      got = true;
    });
    app.start();
    for (int i = 0; i < 50 && !got.load(); ++i) {
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    app.shutdown();
    return got.load() ? 0 : 1;
  }
  if (cmd == "serve-run") {
    std::string behavior = argc > 2 ? argv[2] : "ok";
    app.serve(mica::tokens::Worker_Run, [&](const jobs::v1::RunRequest& request) {
      if (behavior == "invalid") {
        throw mica::RpcError(mica::RpcCode::InvalidArgument, "bad job");
      }
      if (behavior == "internal") {
        throw std::runtime_error("boom");
      }
      if (behavior == "sleep") {
        std::this_thread::sleep_for(std::chrono::seconds(3));
      }
      jobs::v1::RunResponse response;
      response.set_accepted(true);
      (void)request;
      return response;
    });
    app.run();
    return 0;
  }
  if (cmd == "call-run") {
    app.start();
    jobs::v1::RunRequest request;
    request.mutable_job_id()->set_value("job-1");
    request.set_input("task-1");
    int rc = 0;
    try {
      auto response = app.call(mica::tokens::Worker_Run, request, std::chrono::milliseconds(1000));
      std::cout << "accepted=" << response.accepted() << std::endl;
    } catch (const mica::RpcError& err) {
      std::cerr << mica::rpc_code_name(err.code()) << std::endl;
      rc = 10 + static_cast<int>(err.code());
    } catch (const std::exception& err) {
      std::cerr << err.what() << std::endl;
      rc = 1;
    }
    app.shutdown();
    return rc;
  }
  if (cmd == "publish-load") {
    int count = 0;
    int rate = 0;
    if (argc < 4 || !parse_positive(argv[2], &count) || !parse_positive(argv[3], &rate)) {
      return 2;
    }
    return publish_load(app, count, rate);
  }
  if (cmd == "subscribe-load") {
    int expect = 0;
    double timeout_s = 0;
    if (argc < 4 || !parse_positive(argv[2], &expect) ||
        !parse_positive_double(argv[3], &timeout_s)) {
      return 2;
    }
    return subscribe_load(app, expect, timeout_s);
  }
  if (cmd == "call-load") {
    int count = 0;
    int rate = 0;
    int concurrency = 0;
    if (argc < 5 || !parse_positive(argv[2], &count) || !parse_positive(argv[3], &rate) ||
        !parse_positive(argv[4], &concurrency)) {
      return 2;
    }
    return call_load(app, count, rate, concurrency);
  }
  if (cmd == "serve-pipeline") {
    return serve_pipeline(app);
  }
  usage();
  return 2;
}
