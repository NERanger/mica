package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mica/cli/internal/spec"
)

func writeSurfaceFile(t *testing.T, root, name, body string) {
	t.Helper()
	path := surfacePath(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSurfaceDriftUndeclaredUse(t *testing.T) {
	root := t.TempDir()
	writeSurfaceFile(t, root, "client", `{
		"component": "client",
		"publishes": ["audit.v1.JobRecorded"],
		"subscribes": ["jobs.v1.JobCompleted"],
		"calls": ["jobs.v1.Worker.Run"],
		"provides": ["jobs.v1.Worker.Check"]
	}`)
	process := spec.Process{Name: "client"}
	warnings := surfaceDrift(root, process)
	if len(warnings) != 4 {
		t.Fatalf("expected 4 warnings, got %v", warnings)
	}
	for _, warning := range warnings {
		if !strings.HasSuffix(warning, "is not declared in component.toml") {
			t.Fatalf("unexpected warning: %s", warning)
		}
	}
}

func TestSurfaceDriftDeclaredNotRegistered(t *testing.T) {
	root := t.TempDir()
	writeSurfaceFile(t, root, "worker", `{
		"component": "worker",
		"publishes": [],
		"subscribes": [],
		"calls": [],
		"provides": []
	}`)
	process := spec.Process{
		Name:       "worker",
		Subscribes: []string{"jobs.v1.JobCompleted"},
		Provides:   []string{"jobs.v1.Worker.Run"},
		Publishes:  []string{"audit.v1.JobRecorded"},
		Calls:      []string{"jobs.v1.Worker.Check"},
	}
	warnings := surfaceDrift(root, process)
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings (publishes/calls may be unexercised), got %v", warnings)
	}
	for _, warning := range warnings {
		if !strings.HasSuffix(warning, "no handler was registered") {
			t.Fatalf("unexpected warning: %s", warning)
		}
	}
}

func TestSurfaceDriftMatch(t *testing.T) {
	root := t.TempDir()
	writeSurfaceFile(t, root, "client", `{
		"component": "client",
		"publishes": [],
		"subscribes": ["audit.v1.JobRecorded"],
		"calls": ["jobs.v1.Worker.Run"],
		"provides": []
	}`)
	process := spec.Process{
		Name:       "client",
		Subscribes: []string{"audit.v1.JobRecorded"},
		Calls:      []string{"jobs.v1.Worker.Run"},
	}
	if warnings := surfaceDrift(root, process); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestSurfaceDriftMissingFile(t *testing.T) {
	root := t.TempDir()
	process := spec.Process{Name: "gone", Subscribes: []string{"jobs.v1.JobCompleted"}}
	if warnings := surfaceDrift(root, process); len(warnings) != 0 {
		t.Fatalf("expected no warnings without surface file, got %v", warnings)
	}
}

func TestSurfaceDriftMalformedFile(t *testing.T) {
	root := t.TempDir()
	writeSurfaceFile(t, root, "bad", "not json")
	process := spec.Process{Name: "bad", Subscribes: []string{"jobs.v1.JobCompleted"}}
	if warnings := surfaceDrift(root, process); len(warnings) != 0 {
		t.Fatalf("expected no warnings for malformed file, got %v", warnings)
	}
}
