package manifest

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultShutdownTimeoutMS = 10000

	AdapterCMake  = "cmake"
	AdapterGo     = "go"
	AdapterPython = "python"

	ArtifactExecutable   = "executable"
	ArtifactPythonSource = "python"

	RestartNever     = "never"
	RestartOnFailure = "on-failure"

	TransportNATS = "nats"
)

var Languages = map[string]bool{"python": true, "cpp": true, "go": true}

type App struct {
	Path              string
	Dir               string
	Name              string
	ShutdownTimeoutMS int
	TransportKind     string
	TransportURL      string
	ContractsDir      string
	GeneratedDir      string
	GoRuntime         string
	Processes         []Process
	Test              *TestSpec
}

type Process struct {
	Name              string
	Component         string
	ComponentPath     string
	Args              []string
	Env               map[string]string
	WorkingDirectory  string
	Restart           string
	ShutdownTimeoutMS int
}

type Component struct {
	Path         string
	Dir          string
	Name         string
	Language     string
	Publishes    []string
	Subscribes   []string
	Calls        []string
	Provides     []string
	Build        BuildSpec
	Artifact     ArtifactSpec
	Run          *RunSpec
	Test         *TestSpec
	Requirements Requirements
}

type BuildSpec struct {
	Adapter string
	CMake   CMakeBuild
	Go      GoBuild
	Python  PythonBuild
}

type CMakeBuild struct {
	SourceDir     string
	Target        string
	ConfigureArgs []string
	BuildArgs     []string
}

type GoBuild struct {
	ModuleDir string
	Package   string
	Output    string
	BuildArgs []string
}

type PythonBuild struct {
	Source string
	Module string
}

type ArtifactSpec struct {
	Kind string
	Path string
}

type RunSpec struct {
	Command          string
	Args             []string
	WorkingDirectory string
}

type TestSpec struct {
	Command string
	Args    []string
}

type Requirements struct {
	Executables []string
	Libraries   []string
	Python      *PythonRequirements
}

type PythonRequirements struct {
	Version  string
	Packages []string
}

type appFile struct {
	App struct {
		Name              string `toml:"name"`
		ShutdownTimeoutMS *int   `toml:"shutdown_timeout_ms"`
	} `toml:"app"`
	Transport struct {
		Kind string `toml:"kind"`
		URL  string `toml:"url"`
	} `toml:"transport"`
	Generated *struct {
		GoRuntime string `toml:"go_runtime"`
	} `toml:"generated"`
	Test    *testFile     `toml:"test"`
	Process []processFile `toml:"process"`
}

type processFile struct {
	Name              string            `toml:"name"`
	Component         string            `toml:"component"`
	Args              []string          `toml:"args"`
	Env               map[string]string `toml:"env"`
	WorkingDirectory  string            `toml:"working_directory"`
	Restart           string            `toml:"restart"`
	ShutdownTimeoutMS *int              `toml:"shutdown_timeout_ms"`
}

type testFile struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

type componentFile struct {
	Component struct {
		Name       string   `toml:"name"`
		Language   string   `toml:"language"`
		Publishes  []string `toml:"publishes"`
		Subscribes []string `toml:"subscribes"`
		Calls      []string `toml:"calls"`
		Provides   []string `toml:"provides"`
	} `toml:"component"`
	Build struct {
		Adapter string `toml:"adapter"`
		CMake   *struct {
			SourceDir     string   `toml:"source_dir"`
			Target        string   `toml:"target"`
			ConfigureArgs []string `toml:"configure_args"`
			BuildArgs     []string `toml:"build_args"`
		} `toml:"cmake"`
		Go *struct {
			ModuleDir string   `toml:"module_dir"`
			Package   string   `toml:"package"`
			Output    string   `toml:"output"`
			BuildArgs []string `toml:"build_args"`
		} `toml:"go"`
		Python *struct {
			Source string `toml:"source"`
			Module string `toml:"module"`
		} `toml:"python"`
	} `toml:"build"`
	Artifact *struct {
		Kind string `toml:"kind"`
		Path string `toml:"path"`
	} `toml:"artifact"`
	Run *struct {
		Command          string   `toml:"command"`
		Args             []string `toml:"args"`
		WorkingDirectory string   `toml:"working_directory"`
	} `toml:"run"`
	Test         *testFile `toml:"test"`
	Requirements *struct {
		Executables []string `toml:"executables"`
		Libraries   []string `toml:"libraries"`
		Python      *struct {
			Version  string   `toml:"version"`
			Packages []string `toml:"packages"`
		} `toml:"python"`
	} `toml:"requirements"`
}

func ParseApp(path string) (*App, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var raw appFile
	if err := decodeStrict(abs, &raw); err != nil {
		return nil, err
	}
	if raw.App.Name == "" {
		return nil, fmt.Errorf("%s: [app].name is required", abs)
	}
	if raw.Transport.Kind != TransportNATS {
		return nil, fmt.Errorf("%s: transport.kind must be %q", abs, TransportNATS)
	}
	if raw.Transport.URL == "" {
		return nil, fmt.Errorf("%s: transport.url is required", abs)
	}
	transportURL, err := url.Parse(raw.Transport.URL)
	if err != nil || transportURL.Scheme != TransportNATS || transportURL.Hostname() == "" || transportURL.Path != "" || transportURL.RawQuery != "" || transportURL.Fragment != "" {
		return nil, fmt.Errorf("%s: transport.url must be a valid nats:// URL", abs)
	}
	shutdown := DefaultShutdownTimeoutMS
	if raw.App.ShutdownTimeoutMS != nil {
		shutdown = *raw.App.ShutdownTimeoutMS
		if shutdown <= 0 {
			return nil, fmt.Errorf("%s: app.shutdown_timeout_ms must be a positive integer", abs)
		}
	}
	dir := filepath.Dir(abs)
	contracts := filepath.Join(dir, "contracts")
	generated := filepath.Join(dir, "generated")
	goRuntime := "mica/runtime/go/mica"
	if raw.Generated != nil {
		if raw.Generated.GoRuntime != "" {
			goRuntime = raw.Generated.GoRuntime
		}
	}
	if len(raw.Process) == 0 {
		return nil, fmt.Errorf("%s: at least one [[process]] is required", abs)
	}
	seen := map[string]bool{}
	processes := make([]Process, 0, len(raw.Process))
	for i, item := range raw.Process {
		if item.Name == "" {
			return nil, fmt.Errorf("%s: process %d missing name", abs, i)
		}
		if seen[item.Name] {
			return nil, fmt.Errorf("%s: duplicate process name %q", abs, item.Name)
		}
		seen[item.Name] = true
		if item.Component == "" {
			return nil, fmt.Errorf("%s: process %q missing component", abs, item.Name)
		}
		restart := item.Restart
		if restart == "" {
			restart = RestartNever
		}
		if restart != RestartNever && restart != RestartOnFailure {
			return nil, fmt.Errorf("%s: process %q restart must be %q or %q", abs, item.Name, RestartNever, RestartOnFailure)
		}
		procShutdown := shutdown
		if item.ShutdownTimeoutMS != nil {
			procShutdown = *item.ShutdownTimeoutMS
			if procShutdown <= 0 {
				return nil, fmt.Errorf("%s: process %q shutdown_timeout_ms must be a positive integer", abs, item.Name)
			}
		}
		componentPath := resolve(dir, item.Component)
		processes = append(processes, Process{
			Name:              item.Name,
			Component:         item.Component,
			ComponentPath:     componentPath,
			Args:              append([]string(nil), item.Args...),
			Env:               copyEnv(item.Env),
			WorkingDirectory:  item.WorkingDirectory,
			Restart:           restart,
			ShutdownTimeoutMS: procShutdown,
		})
	}
	return &App{
		Path:              abs,
		Dir:               dir,
		Name:              raw.App.Name,
		ShutdownTimeoutMS: shutdown,
		TransportKind:     raw.Transport.Kind,
		TransportURL:      raw.Transport.URL,
		ContractsDir:      contracts,
		GeneratedDir:      generated,
		GoRuntime:         goRuntime,
		Processes:         processes,
		Test:              toTestSpec(raw.Test),
	}, nil
}

func ParseComponent(path string) (*Component, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	var raw componentFile
	if err := decodeStrict(abs, &raw); err != nil {
		return nil, err
	}
	if raw.Component.Name == "" {
		return nil, fmt.Errorf("%s: component.name is required", abs)
	}
	if !Languages[raw.Component.Language] {
		return nil, fmt.Errorf("%s: component.language must be one of cpp, go, python", abs)
	}
	dir := filepath.Dir(abs)
	component := &Component{
		Path:       abs,
		Dir:        dir,
		Name:       raw.Component.Name,
		Language:   raw.Component.Language,
		Publishes:  append([]string(nil), raw.Component.Publishes...),
		Subscribes: append([]string(nil), raw.Component.Subscribes...),
		Calls:      append([]string(nil), raw.Component.Calls...),
		Provides:   append([]string(nil), raw.Component.Provides...),
		Test:       toTestSpec(raw.Test),
	}
	if raw.Build.Adapter == "" {
		return nil, fmt.Errorf("%s: component.build.adapter is required", abs)
	}
	component.Build.Adapter = raw.Build.Adapter
	switch raw.Build.Adapter {
	case AdapterCMake:
		if raw.Build.CMake == nil {
			return nil, fmt.Errorf("%s: build.cmake table is required for the cmake adapter", abs)
		}
		if raw.Build.CMake.Target == "" {
			return nil, fmt.Errorf("%s: build.cmake.target is required", abs)
		}
		sourceDir := raw.Build.CMake.SourceDir
		if sourceDir == "" {
			sourceDir = "."
		}
		component.Build.CMake = CMakeBuild{
			SourceDir:     sourceDir,
			Target:        raw.Build.CMake.Target,
			ConfigureArgs: append([]string(nil), raw.Build.CMake.ConfigureArgs...),
			BuildArgs:     append([]string(nil), raw.Build.CMake.BuildArgs...),
		}
	case AdapterGo:
		if raw.Build.Go == nil {
			return nil, fmt.Errorf("%s: build.go table is required for the go adapter", abs)
		}
		if raw.Build.Go.Package == "" {
			return nil, fmt.Errorf("%s: build.go.package is required", abs)
		}
		if raw.Build.Go.Output == "" {
			return nil, fmt.Errorf("%s: build.go.output is required", abs)
		}
		if err := validateRelativePath(raw.Build.Go.Output, "build.go.output", false); err != nil {
			return nil, fmt.Errorf("%s: %w", abs, err)
		}
		moduleDir := raw.Build.Go.ModuleDir
		if moduleDir == "" {
			moduleDir = "."
		}
		component.Build.Go = GoBuild{
			ModuleDir: moduleDir,
			Package:   raw.Build.Go.Package,
			Output:    raw.Build.Go.Output,
			BuildArgs: append([]string(nil), raw.Build.Go.BuildArgs...),
		}
	case AdapterPython:
		if raw.Build.Python == nil {
			return nil, fmt.Errorf("%s: build.python table is required for the python adapter", abs)
		}
		source := raw.Build.Python.Source
		if source == "" {
			source = "."
		}
		component.Build.Python = PythonBuild{
			Source: source,
			Module: raw.Build.Python.Module,
		}
	default:
		return nil, fmt.Errorf("%s: unknown build adapter %q", abs, raw.Build.Adapter)
	}
	if raw.Artifact == nil {
		return nil, fmt.Errorf("%s: [artifact] is required", abs)
	}
	if raw.Artifact.Kind == "" {
		return nil, fmt.Errorf("%s: artifact.kind is required", abs)
	}
	if raw.Artifact.Path == "" {
		return nil, fmt.Errorf("%s: artifact.path is required", abs)
	}
	if err := validateRelativePath(raw.Artifact.Path, "artifact.path", true); err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}
	switch raw.Artifact.Kind {
	case ArtifactExecutable, ArtifactPythonSource:
	default:
		return nil, fmt.Errorf("%s: artifact.kind must be %q or %q", abs, ArtifactExecutable, ArtifactPythonSource)
	}
	component.Artifact = ArtifactSpec{Kind: raw.Artifact.Kind, Path: raw.Artifact.Path}
	if raw.Run != nil {
		if raw.Run.Command == "" {
			return nil, fmt.Errorf("%s: run.command is required when [run] is present", abs)
		}
		component.Run = &RunSpec{
			Command:          raw.Run.Command,
			Args:             append([]string(nil), raw.Run.Args...),
			WorkingDirectory: raw.Run.WorkingDirectory,
		}
	}
	if raw.Requirements != nil {
		component.Requirements.Executables = append([]string(nil), raw.Requirements.Executables...)
		component.Requirements.Libraries = append([]string(nil), raw.Requirements.Libraries...)
		if raw.Requirements.Python != nil {
			component.Requirements.Python = &PythonRequirements{
				Version:  raw.Requirements.Python.Version,
				Packages: append([]string(nil), raw.Requirements.Python.Packages...),
			}
		}
	}
	if component.Artifact.Kind == ArtifactPythonSource && component.Run == nil {
		return nil, fmt.Errorf("%s: [run] is required for python artifacts", abs)
	}
	return component, nil
}

func decodeStrict(path string, out any) error {
	md, err := toml.DecodeFile(path, out)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	keys := md.Undecoded()
	if len(keys) > 0 {
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			names = append(names, key.String())
		}
		sort.Strings(names)
		return fmt.Errorf("%s: unknown keys: %v", path, names)
	}
	return nil
}

func resolve(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

func validateRelativePath(path, field string, allowDot bool) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("%s must be a relative path", field)
	}
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must stay inside the component build output", field)
	}
	if !allowDot && cleaned == "." {
		return fmt.Errorf("%s must identify a file", field)
	}
	return nil
}

func copyEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = value
	}
	return out
}

func toTestSpec(raw *testFile) *TestSpec {
	if raw == nil {
		return nil
	}
	return &TestSpec{Command: raw.Command, Args: append([]string(nil), raw.Args...)}
}

func Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
