package sdk

import (
	"os"
	"path/filepath"
)

func Root() string {
	if value := os.Getenv("MICA_SDK_ROOT"); value != "" {
		return value
	}
	var starts []string
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	for _, start := range starts {
		if root := find(start); root != "" {
			return root
		}
	}
	return ""
}

func find(start string) string {
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if isRoot(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func isRoot(dir string) bool {
	candidates := []string{
		filepath.Join(dir, "runtime", "cpp", "include", "mica", "app.hpp"),
		filepath.Join(dir, "runtime", "python", "src", "mica", "__init__.py"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func PythonRuntimeDir() string {
	root := Root()
	if root == "" {
		return ""
	}
	path := filepath.Join(root, "runtime", "python", "src", "mica")
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return path
	}
	return ""
}
