include(FetchContent)

find_package(Protobuf REQUIRED)

set(NATS_BUILD_STREAMING OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_EXAMPLES OFF CACHE BOOL "" FORCE)
set(NATS_BUILD_WITH_TLS ON CACHE BOOL "" FORCE)

FetchContent_Declare(
  nats_c
  GIT_REPOSITORY https://github.com/nats-io/nats.c.git
  GIT_TAG v3.8.3
  EXCLUDE_FROM_ALL
)
FetchContent_MakeAvailable(nats_c)
FetchContent_GetProperties(nats_c)

if(NOT DEFINED MICA_RUNTIME_GENERATED_DIR)
  set(MICA_RUNTIME_GENERATED_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../../generated/cpp")
endif()

add_library(mica
  src/subject.cpp
  src/error.cpp
  src/config.cpp
  src/health.cpp
  src/app.cpp
  "${MICA_RUNTIME_GENERATED_DIR}/mica/v1/runtime.pb.cc"
)

target_include_directories(mica
  PUBLIC
    "${CMAKE_CURRENT_SOURCE_DIR}/include"
    "${MICA_RUNTIME_GENERATED_DIR}"
    ${Protobuf_INCLUDE_DIRS}
  PRIVATE
    "${nats_c_SOURCE_DIR}/src"
)

target_link_libraries(mica
  PUBLIC
    protobuf::libprotobuf
    nats_static
)

target_compile_features(mica PUBLIC cxx_std_20)
