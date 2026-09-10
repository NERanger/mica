add_executable(mica_subject_test main.cpp)
target_link_libraries(mica_subject_test PRIVATE mica)
add_test(
  NAME mica_subject_test
  COMMAND mica_subject_test "${CMAKE_SOURCE_DIR}/tests/compatibility/cases.toml"
)
