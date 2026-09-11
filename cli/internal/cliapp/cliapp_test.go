package cliapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitScaffoldsWorkspace(t *testing.T) {
	dir := t.TempDir()
	if code := Run([]string{"init", dir}); code != 0 {
		t.Fatalf("init returned %d", code)
	}
	for _, path := range []string{
		"app.toml",
		"buf.yaml",
		"buf.gen.yaml",
		"contracts/hello/v1/hello.proto",
		"components/hello/component.toml",
		"components/hello/hello/__main__.py",
	} {
		if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
}

func TestInitRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	if code := Run([]string{"init", dir}); code != 0 {
		t.Fatalf("init returned %d", code)
	}
	if code := Run([]string{"init", dir}); code == 0 {
		t.Fatal("expected second init to fail without --force")
	}
	if code := Run([]string{"init", dir, "--force"}); code != 0 {
		t.Fatalf("forced init returned %d", code)
	}
}
