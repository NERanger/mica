cmake_minimum_required(VERSION 3.20)
project(worker LANGUAGES CXX)

# Both variables are injected by `mica build`; this file never hardcodes
# paths into the framework checkout.
if(NOT MICA_SDK_DIR)
  message(FATAL_ERROR "MICA_SDK_DIR is required; run mica build")
endif()
if(NOT MICA_GENERATED_DIR)
  message(FATAL_ERROR "MICA_GENERATED_DIR is required; run mica build")
endif()

add_subdirectory("${MICA_SDK_DIR}/runtime/cpp" "${CMAKE_BINARY_DIR}/mica-runtime")

# The only protocol dependency is generated contract code plus the MICA
# C++ runtime. There is no dependency on the recorder or client source.
add_executable(worker
  main.cpp
  "${MICA_GENERATED_DIR}/jobs/v1/jobs.pb.cc"
)

target_include_directories(worker PRIVATE "${MICA_GENERATED_DIR}")
target_link_libraries(worker PRIVATE mica)
