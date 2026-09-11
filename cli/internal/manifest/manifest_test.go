package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseApp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "component.toml"), `
[component]
name = "tracker"
language = "python"
publishes = ["tracking.v1.PersonTracked"]
subscribes = ["camera.v1.PoseChanged"]
calls = []
provides = []

[build]
adapter = "python"

[build.python]
source = "."
module = "tracker"

[artifact]
kind = "python"
path = "."

[run]
command = "python3"
args = ["-m", "tracker"]
`)
	appPath := filepath.Join(dir, "app.toml")
	writeFile(t, appPath, `
[app]
name = "demo"
shutdown_timeout_ms = 4000

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "tracker"
component = "./component.toml"
env = { PYTHONUNBUFFERED = "1" }
`)
	app, err := ParseApp(appPath)
	if err != nil {
		t.Fatal(err)
	}
	if app.Name != "demo" || app.ShutdownTimeoutMS != 4000 {
		t.Fatalf("unexpected app: %+v", app)
	}
	if app.GeneratedDir != filepath.Join(dir, "generated") {
		t.Fatalf("unexpected generated dir: %s", app.GeneratedDir)
	}
	if app.GoRuntime != "mica/runtime/go/mica" {
		t.Fatalf("unexpected go runtime: %s", app.GoRuntime)
	}
	if len(app.Processes) != 1 || app.Processes[0].ComponentPath != filepath.Join(dir, "component.toml") {
		t.Fatalf("unexpected processes: %+v", app.Processes)
	}
	component, err := ParseComponent(app.Processes[0].ComponentPath)
	if err != nil {
		t.Fatal(err)
	}
	if component.Build.Adapter != AdapterPython || component.Run.Command != "python3" {
		t.Fatalf("unexpected component: %+v", component)
	}
}

func TestParseAppRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	appPath := filepath.Join(dir, "app.toml")
	writeFile(t, appPath, `
[app]
name = "demo"
mystery = true

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "tracker"
component = "./component.toml"
command = "python3"
`)
	if _, err := ParseApp(appPath); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestParseComponentRequiresArtifact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.toml")
	writeFile(t, path, `
[component]
name = "worker"
language = "go"

[build]
adapter = "go"

[build.go]
package = "."
output = "worker"
`)
	if _, err := ParseComponent(path); err == nil {
		t.Fatal("expected missing artifact error")
	}
}

func TestParseComponentPythonRequiresRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.toml")
	writeFile(t, path, `
[component]
name = "worker"
language = "python"

[build]
adapter = "python"

[build.python]
source = "."

[artifact]
kind = "python"
path = "."
`)
	if _, err := ParseComponent(path); err == nil {
		t.Fatal("expected missing run error")
	}
}

func TestParseComponentRejectsArtifactEscape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "component.toml")
	writeFile(t, path, `
[component]
name = "worker"
language = "go"

[build]
adapter = "go"

[build.go]
package = "."
output = "worker"

[artifact]
kind = "executable"
path = "../worker"
`)
	if _, err := ParseComponent(path); err == nil {
		t.Fatal("expected artifact path escape error")
	}
}
