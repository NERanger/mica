cmake_minimum_required(VERSION 3.20)
project(camera-control LANGUAGES CXX)

if(NOT MICA_SDK_DIR)
  message(FATAL_ERROR "MICA_SDK_DIR is required; run mica build")
endif()
if(NOT MICA_GENERATED_DIR)
  message(FATAL_ERROR "MICA_GENERATED_DIR is required; run mica build")
endif()

add_subdirectory("${MICA_SDK_DIR}/runtime/cpp" "${CMAKE_BINARY_DIR}/mica-runtime")

add_executable(camera-control
  main.cpp
  "${MICA_GENERATED_DIR}/camera/v1/camera.pb.cc"
)

target_include_directories(camera-control PRIVATE "${MICA_GENERATED_DIR}")
target_link_libraries(camera-control PRIVATE mica)
