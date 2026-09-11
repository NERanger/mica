package adapters

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"mica/cli/internal/fsutil"
	"mica/cli/internal/manifest"
	"mica/cli/internal/sdk"
	"mica/cli/internal/workspace"
)

type Artifact struct {
	Kind string
	Path string
	Run  manifest.RunSpec
}

func OutDir(ws *workspace.Workspace, component *manifest.Component) string {
	return filepath.Join(ws.App.Dir, ".mica", "build", component.Name)
}

func Build(ws *workspace.Workspace, component *manifest.Component) (*Artifact, error) {
	switch component.Build.Adapter {
	case manifest.AdapterCMake:
		return buildCMake(ws, component)
	case manifest.AdapterGo:
		return buildGo(ws, component)
	case manifest.AdapterPython:
		return buildPython(ws, component)
	}
	return nil, fmt.Errorf("component %s: unknown adapter %q", component.Name, component.Build.Adapter)
}

func Resolve(ws *workspace.Workspace, component *manifest.Component) (*Artifact, error) {
	switch component.Artifact.Kind {
	case manifest.ArtifactExecutable:
		path := filepath.Join(OutDir(ws, component), component.Artifact.Path)
		if !manifest.Exists(path) {
			return nil, fmt.Errorf("component %s: artifact %s is missing; run mica build", component.Name, path)
		}
		return &Artifact{Kind: component.Artifact.Kind, Path: path}, nil
	case manifest.ArtifactPythonSource:
		path := filepath.Join(OutDir(ws, component), component.Artifact.Path)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("component %s: artifact %s is missing; run mica build", component.Name, path)
		}
		return &Artifact{Kind: component.Artifact.Kind, Path: path, Run: *component.Run}, nil
	}
	return nil, fmt.Errorf("component %s: unknown artifact kind %q", component.Name, component.Artifact.Kind)
}

func Test(ws *workspace.Workspace, component *manifest.Component) error {
	if component.Test != nil {
		return runCommand(component.Dir, component.Test.Command, component.Test.Args...)
	}
	switch component.Build.Adapter {
	case manifest.AdapterCMake:
		outDir := OutDir(ws, component)
		if manifest.Exists(filepath.Join(outDir, "CTestTestfile.cmake")) {
			return runCommand("", "ctest", "--test-dir", outDir, "--output-on-failure")
		}
	case manifest.AdapterGo:
		moduleDir := filepath.Join(component.Dir, component.Build.Go.ModuleDir)
		return runCommand(moduleDir, "go", "test", component.Build.Go.Package)
	case manifest.AdapterPython:
		source := filepath.Join(component.Dir, component.Build.Python.Source)
		if hasPythonTests(source) {
			return runCommand(component.Dir, "python3", "-m", "pytest", source)
		}
	}
	return nil
}

func buildCMake(ws *workspace.Workspace, component *manifest.Component) (*Artifact, error) {
	outDir := OutDir(ws, component)
	sourceDir := filepath.Join(component.Dir, component.Build.CMake.SourceDir)
	if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("component %s: cmake source_dir not found: %s", component.Name, sourceDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	configure := []string{
		"-S", sourceDir,
		"-B", outDir,
		"-DCMAKE_BUILD_TYPE=Release",
		"-DCMAKE_RUNTIME_OUTPUT_DIRECTORY=" + outDir,
		"-DMICA_WORKSPACE_DIR=" + ws.App.Dir,
		"-DMICA_GENERATED_DIR=" + filepath.Join(ws.App.GeneratedDir, "cpp"),
	}
	if root := sdk.Root(); root != "" {
		configure = append(configure, "-DMICA_SDK_DIR="+root)
		if deps := filepath.Join(root, "build", "_deps"); dirExists(deps) {
			configure = append(configure, "-DFETCHCONTENT_BASE_DIR="+deps)
		}
	}
	configure = append(configure, component.Build.CMake.ConfigureArgs...)
	if err := runCommand("", "cmake", configure...); err != nil {
		return nil, err
	}
	build := []string{"--build", outDir, "--target", component.Build.CMake.Target}
	build = append(build, component.Build.CMake.BuildArgs...)
	if err := runCommand("", "cmake", build...); err != nil {
		return nil, err
	}
	path := filepath.Join(outDir, component.Artifact.Path)
	if !manifest.Exists(path) {
		return nil, fmt.Errorf("component %s: cmake target %s did not produce %s", component.Name, component.Build.CMake.Target, path)
	}
	return &Artifact{Kind: manifest.ArtifactExecutable, Path: path}, nil
}

func buildGo(ws *workspace.Workspace, component *manifest.Component) (*Artifact, error) {
	outDir := OutDir(ws, component)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	moduleDir := filepath.Join(component.Dir, component.Build.Go.ModuleDir)
	output := filepath.Join(outDir, component.Build.Go.Output)
	args := []string{"build", "-o", output}
	args = append(args, component.Build.Go.BuildArgs...)
	args = append(args, component.Build.Go.Package)
	if err := runCommand(moduleDir, "go", args...); err != nil {
		return nil, err
	}
	if !manifest.Exists(output) {
		return nil, fmt.Errorf("component %s: go build did not produce %s", component.Name, output)
	}
	return &Artifact{Kind: manifest.ArtifactExecutable, Path: output}, nil
}

func buildPython(ws *workspace.Workspace, component *manifest.Component) (*Artifact, error) {
	outDir := OutDir(ws, component)
	source := filepath.Join(component.Dir, component.Build.Python.Source)
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("component %s: python source not found: %s", component.Name, source)
	}
	if err := fsutil.RemoveAll(outDir); err != nil {
		return nil, err
	}
	if err := fsutil.CopyTree(source, outDir); err != nil {
		return nil, err
	}
	if component.Build.Python.Module != "" && !pythonModuleExists(outDir, component.Build.Python.Module) {
		return nil, fmt.Errorf("component %s: python module %q was not found in the artifact source", component.Name, component.Build.Python.Module)
	}
	interpreter := "python3"
	if component.Run != nil && component.Run.Command != "" {
		interpreter = component.Run.Command
	}
	if err := runCommand("", interpreter, "-m", "compileall", "-q", outDir); err != nil {
		return nil, err
	}
	return &Artifact{Kind: manifest.ArtifactPythonSource, Path: outDir, Run: *component.Run}, nil
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func hasPythonTests(dir string) bool {
	found := false
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return nil
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "test_") && strings.HasSuffix(info.Name(), ".py") {
			found = true
		}
		return nil
	})
	return found
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pythonModuleExists(root, module string) bool {
	path := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(module, ".", "/")))
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return true
	}
	_, err := os.Stat(path + ".py")
	return err == nil
}
