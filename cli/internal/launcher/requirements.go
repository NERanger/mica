package launcher

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"mica/cli/internal/spec"
)

func checkRequirements(dep *spec.Deployment, root string) []string {
	var failures []string
	if dep.Platform.OS != "" && dep.Platform.OS != runtime.GOOS {
		failures = append(failures, fmt.Sprintf("application targets os %s but this host is %s", dep.Platform.OS, runtime.GOOS))
	}
	if dep.Platform.Arch != "" && dep.Platform.Arch != runtime.GOARCH {
		failures = append(failures, fmt.Sprintf("application targets arch %s but this host is %s", dep.Platform.Arch, runtime.GOARCH))
	}
	if failure := transportFailure(dep); failure != "" {
		failures = append(failures, failure)
	}
	for _, requirement := range dep.Requirements {
		label := requirement.Component
		if label == "" {
			label = "application"
		}
		for _, name := range requirement.Executables {
			if _, err := exec.LookPath(name); err != nil {
				failures = append(failures, fmt.Sprintf("component %s: required executable %q not found", label, name))
			}
		}
		for _, name := range requirement.Libraries {
			if !libraryAvailable(name) {
				failures = append(failures, fmt.Sprintf("component %s: required library %q not found", label, name))
			}
		}
		if requirement.PythonVersion != "" {
			version, err := pythonVersion()
			if err != nil {
				failures = append(failures, fmt.Sprintf("component %s: python3 not available: %v", label, err))
			} else if !versionSatisfies(version, requirement.PythonVersion) {
				failures = append(failures, fmt.Sprintf("component %s: python3 %s does not satisfy %s", label, version, requirement.PythonVersion))
			}
		}
		for _, requirementSpec := range requirement.PythonPackages {
			name, constraints := splitConstraints(requirementSpec)
			if name == "mica" {
				if err := pythonImport(name, root, dep.Environment.PythonPath); err != nil {
					failures = append(failures, fmt.Sprintf("component %s: python package %q is not importable", label, name))
				}
				continue
			}
			version, err := pythonPackageVersion(name)
			if err != nil {
				failures = append(failures, fmt.Sprintf("component %s: python package %q not installed", label, name))
				continue
			}
			for _, constraint := range constraints {
				if !versionSatisfies(version, constraint) {
					failures = append(failures, fmt.Sprintf("component %s: python package %s %s does not satisfy %s", label, name, version, constraint))
				}
			}
		}
	}
	return failures
}

func libraryAvailable(name string) bool {
	if out, err := exec.Command("ldconfig", "-p").Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && fields[0] == name {
				return true
			}
		}
		return false
	}
	dirs := []string{"/lib", "/lib64", "/usr/lib", "/usr/lib64", "/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu"}
	for _, dir := range dirs {
		if matches, _ := filepath.Glob(filepath.Join(dir, name)); len(matches) > 0 {
			return true
		}
	}
	return false
}

func pythonVersion() (string, error) {
	out, err := exec.Command("python3", "-c", "import sys; print('.'.join(str(p) for p in sys.version_info[:3]))").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func pythonPackageVersion(name string) (string, error) {
	if index := strings.Index(name, "["); index >= 0 {
		name = name[:index]
	}
	out, err := exec.Command(
		"python3",
		"-c",
		"import importlib.metadata as m, sys; print(m.version(sys.argv[1]))",
		name,
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func pythonImport(name, root string, pythonPath []string) error {
	cmd := exec.Command(
		"python3",
		"-c",
		"import importlib, sys; importlib.import_module(sys.argv[1])",
		name,
	)
	var paths []string
	for _, entry := range pythonPath {
		paths = append(paths, resolveDir(root, entry))
	}
	if existing := os.Getenv("PYTHONPATH"); existing != "" {
		paths = append(paths, existing)
	}
	if len(paths) > 0 {
		cmd.Env = append(os.Environ(), "PYTHONPATH="+strings.Join(paths, string(os.PathListSeparator)))
	}
	return cmd.Run()
}

func splitConstraint(value string) (string, string) {
	name, constraints := splitConstraints(value)
	if len(constraints) == 0 {
		return name, ""
	}
	return name, constraints[0]
}

func splitConstraints(value string) (string, []string) {
	value = strings.TrimSpace(value)
	for _, operator := range []string{"!=", ">=", "<=", "==", ">", "<"} {
		if index := strings.Index(value, operator); index > 0 {
			name := strings.TrimSpace(value[:index])
			rest := strings.TrimSpace(value[index:])
			var constraints []string
			for _, constraint := range strings.Split(rest, ",") {
				if constraint = strings.TrimSpace(constraint); constraint != "" {
					constraints = append(constraints, constraint)
				}
			}
			return name, constraints
		}
	}
	return value, nil
}

func versionSatisfies(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return true
	}
	operator := ""
	for _, candidate := range []string{">=", "<=", "==", "!=", ">", "<"} {
		if strings.HasPrefix(constraint, candidate) {
			operator = candidate
			constraint = strings.TrimSpace(strings.TrimPrefix(constraint, candidate))
			break
		}
	}
	if operator == "" {
		operator = "=="
	}
	comparison := compareVersions(version, constraint)
	switch operator {
	case ">=":
		return comparison >= 0
	case "<=":
		return comparison <= 0
	case ">":
		return comparison > 0
	case "<":
		return comparison < 0
	case "==":
		return comparison == 0
	case "!=":
		return comparison != 0
	}
	return false
}

func compareVersions(a, b string) int {
	left := versionParts(a)
	right := versionParts(b)
	for i := 0; i < len(left) || i < len(right); i++ {
		var l, r int
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l != r {
			if l < r {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(value string) []int {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	var parts []int
	for _, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil {
			break
		}
		parts = append(parts, number)
	}
	return parts
}

func transportFailure(dep *spec.Deployment) string {
	if dep.Transport.Kind == "" {
		return "application transport kind is missing"
	}
	if dep.Transport.Kind != "nats" {
		return fmt.Sprintf("unsupported application transport %q", dep.Transport.Kind)
	}
	u, err := url.Parse(dep.Transport.URL)
	if err != nil || u.Scheme != "nats" || u.Hostname() == "" {
		return fmt.Sprintf("invalid NATS transport URL %q", dep.Transport.URL)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Sprintf("invalid NATS transport URL %q", dep.Transport.URL)
	}
	port := u.Port()
	if port == "" {
		port = "4222"
	}
	address := net.JoinHostPort(u.Hostname(), port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return fmt.Sprintf("NATS transport %s is unavailable: %v", dep.Transport.URL, err)
	}
	_ = conn.Close()
	return ""
}

func resolve(root, command string) string {
	if strings.ContainsRune(command, filepath.Separator) || strings.HasPrefix(command, ".") {
		if filepath.IsAbs(command) {
			return command
		}
		return filepath.Join(root, command)
	}
	return command
}

func resolveDir(root, dir string) string {
	if dir == "" {
		return root
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(root, dir)
}
