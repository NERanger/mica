set(DEMO_GENERATED_DIR "${CMAKE_SOURCE_DIR}/examples/demo/generated/cpp")

add_executable(mica-cpp-harness
  main.cpp
  "${DEMO_GENERATED_DIR}/jobs/v1/jobs.pb.cc"
  "${DEMO_GENERATED_DIR}/audit/v1/audit.pb.cc"
)

target_include_directories(mica-cpp-harness PRIVATE "${DEMO_GENERATED_DIR}")
target_link_libraries(mica-cpp-harness PRIVATE mica)
