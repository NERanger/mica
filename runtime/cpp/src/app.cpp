#include "mica/app.hpp"

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstring>
#include <iostream>
#include <mutex>
#include <stdexcept>
#include <thread>
#include <vector>

#include <nats.h>
#include <signal.h>

#include "mica/v1/runtime.pb.h"

namespace mica {
namespace {

std::atomic<App*> g_running_app{nullptr};

void handle_os_signal(int) {
  App* app = g_running_app.load();
  if (app != nullptr) {
    app->shutdown();
  }
}

std::string make_message_id() {
  static std::atomic<std::uint64_t> counter{1};
  return std::to_string(counter.fetch_add(1));
}

std::uint64_t now_ns() {
  auto now = std::chrono::system_clock::now().time_since_epoch();
  return static_cast<std::uint64_t>(
      std::chrono::duration_cast<std::chrono::nanoseconds>(now).count());
}

mica::v1::Envelope make_envelope(const std::string& contract_id, const std::string& sender,
                                 const std::string& payload, RpcCode code, const std::string& message,
                                 bool with_status) {
  mica::v1::Envelope env;
  env.set_protocol_version(kProtocolVersion);
  env.set_message_id(make_message_id());
  env.set_sender_id(sender);
  env.set_timestamp_ns(now_ns());
  env.set_contract_id(contract_id);
  env.set_payload(payload);
  if (with_status) {
    auto* status = env.mutable_status();
    status->set_code(static_cast<mica::v1::RpcCode>(code));
    status->set_message(message);
  }
  return env;
}

}  // namespace

struct App::Impl {
  AppConfig config;
  natsConnection* conn = nullptr;
  std::mutex mu;
  std::condition_variable cv;
  State state = State::Stopped;
  std::chrono::steady_clock::time_point started_at{};
  std::vector<natsSubscription*> subs;
  struct PendingEvent {
    std::string subject;
    std::function<std::unique_ptr<google::protobuf::Message>()> factory;
    std::function<void(const google::protobuf::Message&)> handler;
  };
  struct PendingRpc {
    std::string subject;
    std::string contract_id;
    std::function<std::unique_ptr<google::protobuf::Message>()> request_factory;
    std::function<std::string(const google::protobuf::Message&)> handler;
  };
  std::vector<PendingEvent> pending_events;
  std::vector<PendingRpc> pending_rpcs;
  std::string failed_message;

  ~Impl() {
    for (auto* sub : subs) {
      natsSubscription_Destroy(sub);
    }
    if (conn != nullptr) {
      natsConnection_Destroy(conn);
    }
  }
};

App::App(std::string name) : App(name, AppConfig::from_env(std::move(name))) {}

App::App(std::string name, AppConfig config) : impl_(std::make_unique<Impl>()) {
  if (config.name.empty()) {
    config.name = std::move(name);
  }
  impl_->config = std::move(config);
}

App::~App() {
  try {
    shutdown();
  } catch (...) {
  }
}

void App::publish_event(const google::protobuf::Message& event) {
  if (impl_->state != State::Running || impl_->conn == nullptr) {
    throw TransportError("app is not running");
  }
  const std::string contract_id = event.GetDescriptor()->full_name();
  const std::string subject = event_subject(contract_id);
  auto env = make_envelope(contract_id, impl_->config.name, event.SerializeAsString(), RpcCode::Ok,
                           "", false);
  std::string bytes;
  if (!env.SerializeToString(&bytes)) {
    throw ProtocolError("failed to serialize envelope");
  }
  natsStatus status =
      natsConnection_Publish(impl_->conn, subject.c_str(), bytes.data(), static_cast<int>(bytes.size()));
  if (status != NATS_OK) {
    throw TransportError(std::string("publish failed: ") + natsStatus_GetText(status));
  }
}

void App::subscribe_event(const google::protobuf::Descriptor* descriptor,
                          std::function<std::unique_ptr<google::protobuf::Message>()> factory,
                          std::function<void(const google::protobuf::Message&)> handler) {
  Impl::PendingEvent pending{
      event_subject(descriptor->full_name()),
      std::move(factory),
      std::move(handler),
  };
  std::lock_guard<std::mutex> lock(impl_->mu);
  impl_->pending_events.push_back(std::move(pending));
}

void App::serve_method(std::string service, std::string method, std::string contract_id,
                       std::function<std::unique_ptr<google::protobuf::Message>()> request_factory,
                       std::function<std::string(const google::protobuf::Message&)> handler) {
  Impl::PendingRpc pending{
      rpc_subject(service, method),
      std::move(contract_id),
      std::move(request_factory),
      std::move(handler),
  };
  std::lock_guard<std::mutex> lock(impl_->mu);
  impl_->pending_rpcs.push_back(std::move(pending));
}

std::string App::call_method(const std::string& service, const std::string& method,
                             const std::string& contract_id, const google::protobuf::Message& request,
                             std::chrono::milliseconds timeout) {
  if (impl_->state != State::Running || impl_->conn == nullptr) {
    throw TransportError("app is not running");
  }
  if (timeout.count() <= 0) {
    timeout = impl_->config.rpc_timeout;
  }
  const std::string subject = rpc_subject(service, method);
  auto env = make_envelope(contract_id, impl_->config.name, request.SerializeAsString(), RpcCode::Ok,
                           "", false);
  std::string bytes;
  if (!env.SerializeToString(&bytes)) {
    throw ProtocolError("failed to serialize envelope");
  }
  natsMsg* reply = nullptr;
  natsStatus status =
      natsConnection_Request(&reply, impl_->conn, subject.c_str(), bytes.data(),
                             static_cast<int>(bytes.size()), timeout.count());
  if (status == NATS_TIMEOUT) {
    throw RpcError(RpcCode::Timeout, "rpc timed out");
  }
  if (status == NATS_NO_RESPONDERS) {
    throw RpcError(RpcCode::Unavailable, "no rpc provider");
  }
  if (status != NATS_OK) {
    throw TransportError(std::string("rpc transport failure: ") + natsStatus_GetText(status));
  }
  std::unique_ptr<natsMsg, void (*)(natsMsg*)> guard(reply, natsMsg_Destroy);
  mica::v1::Envelope decoded;
  if (!decoded.ParseFromArray(natsMsg_GetData(reply), natsMsg_GetDataLength(reply))) {
    throw ProtocolError("malformed envelope");
  }
  if (decoded.protocol_version() != kProtocolVersion) {
    throw ProtocolError("unsupported mica protocol version");
  }
  if (decoded.has_status()) {
    auto code = static_cast<RpcCode>(decoded.status().code());
    if (code != RpcCode::Unspecified && code != RpcCode::Ok) {
      throw RpcError(code, decoded.status().message());
    }
  }
  return decoded.payload();
}

namespace {

struct EventClosure {
  std::function<std::unique_ptr<google::protobuf::Message>()> factory;
  std::function<void(const google::protobuf::Message&)> handler;
};

struct RpcClosure {
  std::string sender;
  std::string contract_id;
  natsConnection* conn;
  std::function<std::unique_ptr<google::protobuf::Message>()> request_factory;
  std::function<std::string(const google::protobuf::Message&)> handler;
};

void event_callback(natsConnection*, natsSubscription*, natsMsg* msg, void* closure) {
  auto* ctx = static_cast<EventClosure*>(closure);
  std::unique_ptr<natsMsg, void (*)(natsMsg*)> guard(msg, natsMsg_Destroy);
  mica::v1::Envelope env;
  if (!env.ParseFromArray(natsMsg_GetData(msg), natsMsg_GetDataLength(msg))) {
    std::cerr << "[mica] dropping malformed event envelope\n";
    return;
  }
  if (env.protocol_version() != kProtocolVersion) {
    std::cerr << "[mica] dropping event with unsupported protocol version\n";
    return;
  }
  auto event = ctx->factory();
  if (!event->ParseFromString(env.payload())) {
    std::cerr << "[mica] dropping malformed event payload\n";
    return;
  }
  try {
    ctx->handler(*event);
  } catch (...) {
    std::cerr << "[mica] event handler failed\n";
  }
}

void rpc_callback(natsConnection*, natsSubscription*, natsMsg* msg, void* closure) {
  auto* ctx = static_cast<RpcClosure*>(closure);
  std::unique_ptr<natsMsg, void (*)(natsMsg*)> guard(msg, natsMsg_Destroy);
  RpcCode code = RpcCode::Ok;
  std::string message;
  std::string payload;
  mica::v1::Envelope env;
  if (!env.ParseFromArray(natsMsg_GetData(msg), natsMsg_GetDataLength(msg))) {
    code = RpcCode::InvalidArgument;
    message = "malformed request";
  } else {
    auto request = ctx->request_factory();
    if (!request->ParseFromString(env.payload())) {
      code = RpcCode::InvalidArgument;
      message = "malformed request";
    } else {
      try {
        payload = ctx->handler(*request);
      } catch (const RpcError& err) {
        code = err.code();
        message = err.rpc_message();
        payload.clear();
      } catch (...) {
        std::cerr << "[mica] rpc handler failed\n";
        code = RpcCode::Internal;
        message = "internal error";
        payload.clear();
      }
    }
  }
  const char* reply = natsMsg_GetReply(msg);
  if (reply == nullptr || reply[0] == '\0') {
    return;
  }
  auto out = make_envelope(ctx->contract_id, ctx->sender, payload, code, message, true);
  std::string bytes;
  if (!out.SerializeToString(&bytes)) {
    return;
  }
  natsConnection_Publish(ctx->conn, reply, bytes.data(), static_cast<int>(bytes.size()));
}

}  // namespace

void App::start() {
  if (impl_->state == State::Running) {
    return;
  }
  impl_->state = State::Starting;
  natsStatus status = natsConnection_ConnectTo(&impl_->conn, impl_->config.transport_url.c_str());
  if (status != NATS_OK) {
    impl_->state = State::Failed;
    throw TransportError(std::string("failed to connect to transport at ") +
                         impl_->config.transport_url + ": " + natsStatus_GetText(status));
  }
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    for (auto& pending : impl_->pending_events) {
      auto* closure = new EventClosure{pending.factory, pending.handler};
      natsSubscription* sub = nullptr;
      status = natsConnection_Subscribe(&sub, impl_->conn, pending.subject.c_str(), event_callback,
                                        closure);
      if (status != NATS_OK) {
        impl_->state = State::Failed;
        throw TransportError(std::string("subscribe failed: ") + natsStatus_GetText(status));
      }
      impl_->subs.push_back(sub);
    }
    for (auto& pending : impl_->pending_rpcs) {
      auto* closure = new RpcClosure{impl_->config.name, pending.contract_id, impl_->conn,
                                     pending.request_factory, pending.handler};
      natsSubscription* sub = nullptr;
      status = natsConnection_Subscribe(&sub, impl_->conn, pending.subject.c_str(), rpc_callback,
                                        closure);
      if (status != NATS_OK) {
        impl_->state = State::Failed;
        throw TransportError(std::string("serve failed: ") + natsStatus_GetText(status));
      }
      impl_->subs.push_back(sub);
    }
  }
  impl_->started_at = std::chrono::steady_clock::now();
  impl_->state = State::Running;
}

void App::run() {
  start();
  g_running_app.store(this);
  signal(SIGINT, handle_os_signal);
  signal(SIGTERM, handle_os_signal);
  std::unique_lock<std::mutex> lock(impl_->mu);
  impl_->cv.wait(lock, [&] { return impl_->state != State::Running; });
}

void App::shutdown() {
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    if (impl_->state == State::Stopped || impl_->state == State::Stopping) {
      impl_->state = State::Stopped;
      impl_->cv.notify_all();
      return;
    }
    impl_->state = State::Stopping;
  }
  if (impl_->conn != nullptr) {
    natsConnection_Drain(impl_->conn);
  }
  {
    std::lock_guard<std::mutex> lock(impl_->mu);
    impl_->state = State::Stopped;
    impl_->cv.notify_all();
  }
  App* expected = this;
  g_running_app.compare_exchange_strong(expected, nullptr);
}

Health App::health() const {
  Health health;
  health.state = impl_->state;
  if (impl_->state == State::Running) {
    health.uptime_ms = static_cast<std::uint64_t>(
        std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::steady_clock::now() -
                                                              impl_->started_at)
            .count());
  }
  return health;
}

}  // namespace mica
