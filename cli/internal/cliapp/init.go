package cliapp

import (
	"fmt"
	"os"
	"path/filepath"
)

func runInit(args []string) int {
	fs := newFlagSet("init")
	force := fs.Bool("force", false, "overwrite existing files")
	if err := parse(fs, args); err != nil {
		return 2
	}
	dir := fs.Arg(0)
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fail(err)
	}
	name := filepath.Base(abs)
	files := initFiles(name)
	for _, file := range files {
		target := filepath.Join(abs, file.path)
		if _, err := os.Stat(target); err == nil && !*force {
			return fail(fmt.Errorf("%s already exists; use --force to overwrite", target))
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fail(err)
		}
		if err := os.WriteFile(target, []byte(file.content), file.mode); err != nil {
			return fail(err)
		}
	}
	fmt.Printf("[mica] initialized workspace %s\n", abs)
	return 0
}

type initFile struct {
	path    string
	content string
	mode    os.FileMode
}

func initFiles(name string) []initFile {
	return []initFile{
		{
			path: "app.toml",
			content: fmt.Sprintf(`[app]
name = %q
shutdown_timeout_ms = 5000

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "hello"
component = "./components/hello/component.toml"
restart = "never"
`, name),
			mode: 0o644,
		},
		{
			path: "buf.yaml",
			content: `version: v2
modules:
  - path: contracts
lint:
  use:
    - STANDARD
  except:
    - SERVICE_SUFFIX
breaking:
  use:
    - FILE
`,
			mode: 0o644,
		},
		{
			path: "buf.gen.yaml",
			content: `version: v2
plugins:
  - protoc_builtin: python
    out: generated/python
  - protoc_builtin: pyi
    out: generated/python
  - protoc_builtin: cpp
    out: generated/cpp
  - local: protoc-gen-go
    out: generated/go
    opt:
      - paths=source_relative
`,
			mode: 0o644,
		},
		{
			path: ".gitignore",
			content: `/.mica/
/generated/
/dist/
*.pyc
__pycache__/
`,
			mode: 0o644,
		},
		{
			path: "contracts/hello/v1/hello.proto",
			content: fmt.Sprintf(`syntax = "proto3";

package hello.v1;

option go_package = "example.com/%s/generated/go/hello/v1;hellov1";

message Hello {
  string message = 1;
}
`, name),
			mode: 0o644,
		},
		{
			path: "components/hello/component.toml",
			content: `[component]
name = "hello"
language = "python"

publishes = []
subscribes = []
calls = []
provides = []

[build]
adapter = "python"

[build.python]
source = "."
module = "hello"

[artifact]
kind = "python"
path = "."

[run]
command = "python3"
args = ["-m", "hello"]

[requirements]
executables = ["python3"]

[requirements.python]
version = ">=3.12"
packages = ["nats-py>=2.9.0", "protobuf>=4.21.12,<6"]
`,
			mode: 0o644,
		},
		{
			path:    "components/hello/hello/__init__.py",
			content: "",
			mode:    0o644,
		},
		{
			path: "components/hello/hello/__main__.py",
			content: `from mica import App

app = App("hello")


async def main() -> None:
    print("hello from mica")


if __name__ == "__main__":
    app.run(main)
`,
			mode: 0o644,
		},
		{
			path:    "tests/.gitkeep",
			content: "",
			mode:    0o644,
		},
	}
}
