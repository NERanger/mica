#include <chrono>
#include <cstdint>
#include <iostream>
#include <mutex>

#include "camera/v1/camera.pb.h"
#include "mica/app.hpp"
#include "mica/tokens.hpp"

int main() {
  mica::App app{"camera-control"};
  camera::v1::Pose current;
  std::mutex mu;

  app.serve(mica::tokens::CameraControl_SetPose, [&](const camera::v1::SetPoseRequest& request) {
    if (!request.has_camera_id() || request.camera_id().value().empty() || !request.has_pose()) {
      throw mica::RpcError(mica::RpcCode::InvalidArgument, "camera_id and pose are required");
    }
    camera::v1::PoseChanged changed;
    {
      std::lock_guard<std::mutex> lock(mu);
      current = request.pose();
      *changed.mutable_camera_id() = request.camera_id();
      *changed.mutable_pose() = current;
    }
    const auto ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                        std::chrono::system_clock::now().time_since_epoch())
                        .count();
    changed.set_timestamp_ns(static_cast<std::uint64_t>(ns));
    std::cout << "SetPose accepted pan=" << request.pose().pan() << std::endl;
    app.publish(changed);
    camera::v1::SetPoseResponse response;
    response.set_accepted(true);
    return response;
  });

  app.run();
  return 0;
}
