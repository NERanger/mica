#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

#include "mica/subject.hpp"

namespace {

std::string unquote(std::string value) {
  if (value.size() >= 2 && value.front() == '"' && value.back() == '"') {
    return value.substr(1, value.size() - 2);
  }
  return value;
}

struct Case {
  std::string kind;
  std::string contract;
  std::string expected;
};

std::vector<Case> load_cases(const std::string& path) {
  std::ifstream in(path);
  if (!in) {
    throw std::runtime_error("cannot open " + path);
  }
  std::vector<Case> cases;
  Case current;
  std::string line;
  auto flush = [&] {
    if (!current.kind.empty()) {
      cases.push_back(current);
      current = {};
    }
  };
  while (std::getline(in, line)) {
    if (line.rfind("[[", 0) == 0) {
      flush();
      if (line.find("event") != std::string::npos) {
        current.kind = "event";
      } else if (line.find("rpc") != std::string::npos) {
        current.kind = "rpc";
      }
      continue;
    }
    auto eq = line.find('=');
    if (eq == std::string::npos) {
      continue;
    }
    std::string key = line.substr(0, eq);
    std::string value = unquote(line.substr(eq + 1));
    while (!key.empty() && key.back() == ' ') {
      key.pop_back();
    }
    while (!value.empty() && value.front() == ' ') {
      value.erase(value.begin());
    }
    value = unquote(value);
    if (key == "contract") {
      current.contract = value;
    } else if (key == "expected_subject") {
      current.expected = value;
    }
  }
  flush();
  return cases;
}

}  // namespace

int main(int argc, char** argv) {
  if (argc < 2) {
    std::cerr << "usage: mica_subject_test cases.toml\n";
    return 2;
  }
  try {
    for (const auto& item : load_cases(argv[1])) {
      std::string actual;
      if (item.kind == "event") {
        actual = mica::event_subject(item.contract);
      } else {
        actual = mica::rpc_subject_from_contract(item.contract);
      }
      if (actual != item.expected) {
        std::cerr << item.contract << ": got " << actual << " expected " << item.expected << "\n";
        return 1;
      }
    }
  } catch (const std::exception& exc) {
    std::cerr << exc.what() << "\n";
    return 1;
  }
  return 0;
}
