package contracts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mica/cli/internal/workspace"
)

func Generate(ws *workspace.Workspace) error {
	buf := os.Getenv("MICA_BUF")
	if buf == "" {
		buf = "buf"
	}
	if _, err := exec.LookPath(buf); err != nil {
		return fmt.Errorf("%s is required; run scripts/bootstrap", buf)
	}
	generated := ws.App.GeneratedDir
	if err := os.MkdirAll(filepath.Join(generated, "python"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(generated, "cpp"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(generated, "go"), 0o755); err != nil {
		return err
	}
	if err := run(buf, ws.App.Dir, "lint"); err != nil {
		return err
	}
	if err := run(buf, ws.App.Dir, "generate"); err != nil {
		return err
	}
	image := ImagePath(ws)
	if err := run(buf, ws.App.Dir, "build", "-o", image); err != nil {
		return err
	}
	if err := ensurePythonPackages(filepath.Join(generated, "python")); err != nil {
		return err
	}
	return WriteTokens(image, generated, ws.App.GoRuntime)
}

func run(name, dir string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func ensurePythonPackages(root string) error {
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, "_pb2.py") {
			return nil
		}
		for current := filepath.Dir(path); current != root && current != filepath.Dir(current); current = filepath.Dir(current) {
			init := filepath.Join(current, "__init__.py")
			if _, err := os.Stat(init); os.IsNotExist(err) {
				if err := os.WriteFile(init, nil, 0o644); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
