package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"

	"mica/cli/internal/adapters"
	"mica/cli/internal/build"
	"mica/cli/internal/fsutil"
	"mica/cli/internal/manifest"
	"mica/cli/internal/sdk"
	"mica/cli/internal/spec"
	"mica/cli/internal/workspace"
)

type Options struct {
	Output    string
	Local     bool
	Host      string
	SSHPort   int
	StartNATS bool
	Generate  bool
}

func Run(ws *workspace.Workspace, opts Options) error {
	result, err := build.Run(ws, opts.Generate)
	if err != nil {
		return err
	}
	staging, err := Prepare(ws, result)
	if err != nil {
		return err
	}
	output := opts.Output
	if output == "" {
		output = filepath.Join(ws.App.Dir, "dist", ws.App.Name+".run")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := Package(staging, output, ws.App.Name); err != nil {
		return err
	}
	fmt.Printf("[mica] wrote %s\n", output)
	if opts.Local {
		return runLocal(ws, output, opts.StartNATS)
	}
	if opts.Host != "" {
		return RunSSH(output, opts.Host, opts.SSHPort)
	}
	return nil
}

func Prepare(ws *workspace.Workspace, result *build.Result) (string, error) {
	staging := filepath.Join(ws.App.Dir, ".mica", "deploy", ws.App.Name)
	if err := fsutil.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", err
	}
	deployment := &spec.Deployment{}
	deployment.App.Name = ws.App.Name
	deployment.App.ShutdownTimeoutMS = ws.App.ShutdownTimeoutMS
	deployment.Transport.Kind = ws.App.TransportKind
	deployment.Transport.URL = ws.App.TransportURL
	deployment.Platform.OS = runtime.GOOS
	deployment.Platform.Arch = runtime.GOARCH
	deployment.Environment.PythonPath = []string{"generated/python", "python"}
	runtimeDir := sdk.PythonRuntimeDir()
	hasPython := false
	for _, process := range ws.App.Processes {
		component, ok := ws.ProcessComponent(process)
		if !ok {
			return "", fmt.Errorf("process %q: component not loaded", process.Name)
		}
		artifact, ok := result.Artifacts[component.Path]
		if !ok {
			var err error
			artifact, err = adapters.Resolve(ws, component)
			if err != nil {
				return "", err
			}
		}
		destDir := filepath.Join(staging, "components", process.Name)
		var record spec.Process
		record.Name = process.Name
		record.Component = component.Name
		record.Env = process.Env
		record.Restart = process.Restart
		record.ShutdownTimeoutMS = process.ShutdownTimeoutMS
		switch artifact.Kind {
		case manifest.ArtifactExecutable:
			name := filepath.Base(artifact.Path)
			if err := fsutil.CopyFile(artifact.Path, filepath.Join(destDir, name)); err != nil {
				return "", err
			}
			if err := os.Chmod(filepath.Join(destDir, name), 0o755); err != nil {
				return "", err
			}
			record.Command = filepath.ToSlash(filepath.Join("components", process.Name, name))
			record.Args = append([]string{}, process.Args...)
			record.WorkingDirectory = process.WorkingDirectory
		case manifest.ArtifactPythonSource:
			if err := fsutil.CopyTree(artifact.Path, destDir); err != nil {
				return "", err
			}
			hasPython = true
			record.Command = artifact.Run.Command
			record.Args = append(append([]string{}, artifact.Run.Args...), process.Args...)
			workdir := filepath.ToSlash(filepath.Join("components", process.Name))
			if artifact.Run.WorkingDirectory != "" {
				workdir = filepath.ToSlash(filepath.Join(workdir, artifact.Run.WorkingDirectory))
			}
			if process.WorkingDirectory != "" {
				workdir = process.WorkingDirectory
			}
			record.WorkingDirectory = workdir
		default:
			return "", fmt.Errorf("process %q: unsupported artifact kind %q", process.Name, artifact.Kind)
		}
		if requirement, ok := requirementFor(component, runtimeDir != ""); ok {
			deployment.Requirements = append(deployment.Requirements, requirement)
		}
		deployment.Processes = append(deployment.Processes, record)
	}
	if err := copyGenerated(ws, staging, hasPython, runtimeDir); err != nil {
		return "", err
	}
	deploymentFile, err := os.Create(filepath.Join(staging, "deployment.toml"))
	if err != nil {
		return "", err
	}
	if err := toml.NewEncoder(deploymentFile).Encode(deployment); err != nil {
		deploymentFile.Close()
		return "", err
	}
	if err := deploymentFile.Close(); err != nil {
		return "", err
	}
	launcherPath := os.Getenv("MICA_LAUNCHER")
	if launcherPath == "" {
		launcherPath, err = siblingLauncher()
		if err != nil {
			return "", err
		}
	}
	if !manifest.Exists(launcherPath) {
		return "", fmt.Errorf("mica-launcher not found at %s; build the CLI and launcher or set MICA_LAUNCHER", launcherPath)
	}
	if err := fsutil.CopyFile(launcherPath, filepath.Join(staging, "mica-launcher")); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Join(staging, "mica-launcher"), 0o755); err != nil {
		return "", err
	}
	startup := []byte("#!/bin/sh\nset -e\ncd \"$(dirname \"$0\")\"\nexec ./mica-launcher deployment.toml\n")
	if err := os.WriteFile(filepath.Join(staging, "startup.sh"), startup, 0o755); err != nil {
		return "", err
	}
	return staging, nil
}

func siblingLauncher() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(executable), "mica-launcher"), nil
}

func copyGenerated(ws *workspace.Workspace, staging string, hasPython bool, runtimeDir string) error {
	image := filepath.Join(ws.App.GeneratedDir, "image.binpb")
	if err := fsutil.CopyFile(image, filepath.Join(staging, "contracts", "image.binpb")); err != nil {
		return err
	}
	if !hasPython {
		return nil
	}
	pythonGenerated := filepath.Join(ws.App.GeneratedDir, "python")
	if info, err := os.Stat(pythonGenerated); err == nil && info.IsDir() {
		if err := fsutil.CopyTree(pythonGenerated, filepath.Join(staging, "generated", "python")); err != nil {
			return err
		}
	}
	if runtimeDir != "" {
		if err := fsutil.CopyTree(runtimeDir, filepath.Join(staging, "python", "mica")); err != nil {
			return err
		}
	}
	return nil
}

func requirementFor(component *manifest.Component, pythonRuntimeBundled bool) (spec.Requirement, bool) {
	requirement := spec.Requirement{Component: component.Name}
	requirement.Executables = append(requirement.Executables, component.Requirements.Executables...)
	requirement.Libraries = append(requirement.Libraries, component.Requirements.Libraries...)
	if component.Requirements.Python != nil {
		requirement.PythonVersion = component.Requirements.Python.Version
		requirement.PythonPackages = append(requirement.PythonPackages, component.Requirements.Python.Packages...)
	}
	if component.Artifact.Kind == manifest.ArtifactPythonSource && !pythonRuntimeBundled && !hasPythonPackage(requirement.PythonPackages, "mica") {
		requirement.PythonPackages = append(requirement.PythonPackages, "mica")
	}
	if len(requirement.Executables) == 0 && len(requirement.Libraries) == 0 && requirement.PythonVersion == "" && len(requirement.PythonPackages) == 0 {
		return requirement, false
	}
	return requirement, true
}

func hasPythonPackage(packages []string, name string) bool {
	for _, item := range packages {
		item = strings.TrimSpace(item)
		for _, operator := range []string{"!=", ">=", "<=", "==", ">", "<"} {
			if index := strings.Index(item, operator); index > 0 {
				item = strings.TrimSpace(item[:index])
				break
			}
		}
		if item == name {
			return true
		}
	}
	return false
}

func runLocal(ws *workspace.Workspace, runPath string, startNATS bool) error {
	var nats *exec.Cmd
	if startNATS {
		var err error
		nats, err = startNATSServer(ws.App.TransportURL)
		if err != nil {
			return err
		}
		fmt.Println("[mica] started nats-server")
	}
	cmd := exec.Command(runPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if nats != nil && nats.Process != nil {
		_ = nats.Process.Kill()
		_ = nats.Wait()
	}
	if err != nil {
		return fmt.Errorf("local deploy failed: %w", err)
	}
	return nil
}
