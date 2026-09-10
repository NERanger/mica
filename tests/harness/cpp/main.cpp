#include <atomic>
#include <chrono>
#include <fstream>
#include <iostream>
#include <string>
#include <thread>

#include "camera/v1/camera.pb.h"
#include "mica/app.hpp"
#include "mica/tokens.hpp"
#include "tracking/v1/tracking.pb.h"

namespace {

void usage() {
  std::cerr << "usage: mica-cpp-harness <command> [args]\n";
}

camera::v1::PoseChanged sample_pose() {
  camera::v1::PoseChanged event;
  event.mutable_camera_id()->set_value("cam-1");
  event.mutable_pose()->set_pan(1.5);
  event.mutable_pose()->set_tilt(2.5);
  event.mutable_pose()->set_zoom(3.5);
  event.set_timestamp_ns(42);
  return event;
}

int proto_roundtrip(const std::string& in_path, const std::string& out_path) {
  std::ifstream in(in_path, std::ios::binary);
  std::string bytes((std::istreambuf_iterator<char>(in)), std::istreambuf_iterator<char>());
  camera::v1::PoseChanged event;
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
  if (cmd == "publish-pose") {
    app.start();
    app.publish(sample_pose());
    std::this_thread::sleep_for(std::chrono::milliseconds(200));
    app.shutdown();
    return 0;
  }
  if (cmd == "subscribe-pose") {
    std::atomic<bool> got{false};
    app.subscribe<camera::v1::PoseChanged>([&](const camera::v1::PoseChanged& event) {
      std::cout << "got pan=" << event.pose().pan() << std::endl;
      got = true;
    });
    app.start();
    for (int i = 0; i < 50 && !got.load(); ++i) {
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    app.shutdown();
    return got.load() ? 0 : 1;
  }
  if (cmd == "serve-setpose") {
    std::string behavior = argc > 2 ? argv[2] : "ok";
    app.serve(mica::tokens::CameraControl_SetPose, [&](const camera::v1::SetPoseRequest& request) {
      if (behavior == "invalid") {
        throw mica::RpcError(mica::RpcCode::InvalidArgument, "bad pose");
      }
      if (behavior == "internal") {
        throw std::runtime_error("boom");
      }
      if (behavior == "sleep") {
        std::this_thread::sleep_for(std::chrono::seconds(3));
      }
      camera::v1::SetPoseResponse response;
      response.set_accepted(true);
      (void)request;
      return response;
    });
    app.run();
    return 0;
  }
  if (cmd == "call-setpose") {
    app.start();
    camera::v1::SetPoseRequest request;
    request.mutable_camera_id()->set_value("cam-1");
    request.mutable_pose()->set_pan(1);
    int rc = 0;
    try {
      auto response = app.call(mica::tokens::CameraControl_SetPose, request,
                               std::chrono::milliseconds(1000));
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
  usage();
  return 2;
}
