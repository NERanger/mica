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
	write(t, filepath.Join(dir, "camera.toml"), `
[component]
name = "camera"
language = "cpp"
publishes = ["camera.v1.PoseChanged"]
provides = ["camera.v1.CameraControl.SetPose"]

[build]
adapter = "cmake"

[build.cmake]
target = "camera"

[artifact]
kind = "executable"
path = "camera"
`)
	write(t, filepath.Join(dir, "planner.toml"), `
[component]
name = "planner"
language = "go"
subscribes = ["camera.v1.PoseChanged"]
calls = ["camera.v1.CameraControl.SetPose"]

[build]
adapter = "go"

[build.go]
package = "."
output = "planner"

[artifact]
kind = "executable"
path = "planner"
`)
	app := filepath.Join(dir, "app.toml")
	write(t, app, `
[app]
name = "demo"

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "cam"
component = "./camera.toml"

[[process]]
name = "plan"
component = "./planner.toml"
`)
	ws, err := workspace.Load(app)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestASCII(t *testing.T) {
	text := ASCII(testWorkspace(t))
	for _, want := range []string{"cam", "plan", "C++", "Go", "camera.v1.PoseChanged", "RPC camera.v1.CameraControl.SetPose"} {
		if !strings.Contains(text, want) {
			t.Fatalf("graph missing %q:\n%s", want, text)
		}
	}
}

func TestDOT(t *testing.T) {
	text := DOT(testWorkspace(t))
	for _, want := range []string{"digraph \"demo\"", "\"cam\" -> \"plan\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("dot missing %q:\n%s", want, text)
		}
	}
}
