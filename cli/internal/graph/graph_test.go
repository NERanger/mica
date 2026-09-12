package graph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mica/cli/internal/workspace"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testWorkspace(t *testing.T) *workspace.Workspace {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "worker.toml"), `
[component]
name = "worker"
language = "cpp"
publishes = ["jobs.v1.JobCompleted"]
provides = ["jobs.v1.Worker.Run"]

[build]
adapter = "cmake"

[build.cmake]
target = "worker"

[artifact]
kind = "executable"
path = "worker"
`)
	write(t, filepath.Join(dir, "client.toml"), `
[component]
name = "client"
language = "go"
subscribes = ["jobs.v1.JobCompleted"]
calls = ["jobs.v1.Worker.Run"]

[build]
adapter = "go"

[build.go]
package = "."
output = "client"

[artifact]
kind = "executable"
path = "client"
`)
	app := filepath.Join(dir, "app.toml")
	write(t, app, `
[app]
name = "demo"

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "work"
component = "./worker.toml"

[[process]]
name = "cli"
component = "./client.toml"
`)
	ws, err := workspace.Load(app)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestASCII(t *testing.T) {
	text := ASCII(testWorkspace(t))
	for _, want := range []string{"work", "cli", "C++", "Go", "jobs.v1.JobCompleted", "RPC jobs.v1.Worker.Run"} {
		if !strings.Contains(text, want) {
			t.Fatalf("graph missing %q:\n%s", want, text)
		}
	}
}

func TestDOT(t *testing.T) {
	text := DOT(testWorkspace(t))
	for _, want := range []string{"digraph \"demo\"", "\"work\" -> \"cli\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("dot missing %q:\n%s", want, text)
		}
	}
}
