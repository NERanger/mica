set(DEMO_GENERATED_DIR "${CMAKE_SOURCE_DIR}/examples/demo/generated/cpp")

add_executable(mica-cpp-harness
  main.cpp
  "${DEMO_GENERATED_DIR}/camera/v1/camera.pb.cc"
  "${DEMO_GENERATED_DIR}/tracking/v1/tracking.pb.cc"
)

target_include_directories(mica-cpp-harness PRIVATE "${DEMO_GENERATED_DIR}")
target_link_libraries(mica-cpp-harness PRIVATE mica)
