package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeWorkspaceFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRejectsDuplicateComponentNames(t *testing.T) {
	dir := t.TempDir()
	component := func(name string) string {
		return "\n[component]\nname = \"" + name + "\"\nlanguage = \"go\"\n\n[build]\nadapter = \"go\"\n\n[build.go]\npackage = \".\"\noutput = \"worker\"\n\n[artifact]\nkind = \"executable\"\npath = \"worker\"\n"
	}
	writeWorkspaceFile(t, filepath.Join(dir, "one", "component.toml"), component("worker"))
	writeWorkspaceFile(t, filepath.Join(dir, "two", "component.toml"), component("worker"))
	writeWorkspaceFile(t, filepath.Join(dir, "app.toml"), `
[app]
name = "duplicate"

[transport]
kind = "nats"
url = "nats://127.0.0.1:4222"

[[process]]
name = "one"
component = "./one/component.toml"

[[process]]
name = "two"
component = "./two/component.toml"
`)
	if _, err := Load(filepath.Join(dir, "app.toml")); err == nil {
		t.Fatal("expected duplicate component name error")
	}
}
