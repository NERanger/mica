#pragma once

#include <chrono>
#include <functional>
#include <memory>
#include <string>
#include <utility>

#include <google/protobuf/message.h>

#include "mica/config.hpp"
#include "mica/error.hpp"
#include "mica/health.hpp"
#include "mica/method.hpp"
#include "mica/subject.hpp"

namespace mica {

class App {
 public:
  explicit App(std::string name);
  App(std::string name, AppConfig config);
  ~App();

  App(const App&) = delete;
  App& operator=(const App&) = delete;

  template <typename Event>
  void publish(const Event& event);

  template <typename Event, typename Handler>
  void subscribe(Handler handler);

  template <typename Request, typename Response, typename Handler>
  void serve(const RpcMethod<Request, Response>& method, Handler handler);

  template <typename Request, typename Response>
  Response call(const RpcMethod<Request, Response>& method, const Request& request,
                std::chrono::milliseconds timeout = std::chrono::milliseconds{0});

  void start();
  void run();
  void shutdown();
  Health health() const;

 private:
  void publish_event(const google::protobuf::Message& event);
  void subscribe_event(const google::protobuf::Descriptor* descriptor,
                       std::function<std::unique_ptr<google::protobuf::Message>()> factory,
                       std::function<void(const google::protobuf::Message&)> handler);
  void serve_method(std::string service, std::string method, std::string contract_id,
                    std::function<std::unique_ptr<google::protobuf::Message>()> request_factory,
                    std::function<std::string(const google::protobuf::Message&)> handler);
  std::string call_method(const std::string& service, const std::string& method,
                          const std::string& contract_id, const google::protobuf::Message& request,
                          std::chrono::milliseconds timeout);
  void write_surface();

  struct Impl;
  std::unique_ptr<Impl> impl_;
};

template <typename Event>
void App::publish(const Event& event) {
  publish_event(event);
}

template <typename Event, typename Handler>
void App::subscribe(Handler handler) {
  subscribe_event(
      Event::GetDescriptor(),
      []() -> std::unique_ptr<google::protobuf::Message> { return std::make_unique<Event>(); },
      [handler](const google::protobuf::Message& message) {
        handler(static_cast<const Event&>(message));
      });
}

template <typename Request, typename Response, typename Handler>
void App::serve(const RpcMethod<Request, Response>& method, Handler handler) {
  serve_method(
      std::string(method.service), std::string(method.method), method.contract_id(),
      []() -> std::unique_ptr<google::protobuf::Message> { return std::make_unique<Request>(); },
      [handler](const google::protobuf::Message& message) {
        Response response = handler(static_cast<const Request&>(message));
        return response.SerializeAsString();
      });
}

template <typename Request, typename Response>
Response App::call(const RpcMethod<Request, Response>& method, const Request& request,
                   std::chrono::milliseconds timeout) {
  std::string payload =
      call_method(std::string(method.service), std::string(method.method), method.contract_id(),
                  request, timeout);
  Response response;
  if (!response.ParseFromString(payload)) {
    throw ProtocolError("malformed rpc response payload");
  }
  return response;
}

}  // namespace mica
