package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"mica/cli/internal/manifest"
)

type Workspace struct {
	App        *manifest.App
	Components []*manifest.Component
	byPath     map[string]*manifest.Component
}

func Find(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		return dir, nil
	}
	for current := dir; ; current = filepath.Dir(current) {
		candidate := filepath.Join(current, "app.toml")
		if manifest.Exists(candidate) {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("cannot find app.toml from %s", start)
		}
	}
}

func Load(appPath string) (*Workspace, error) {
	app, err := manifest.ParseApp(appPath)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(app.ContractsDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("contracts directory not found: %s", app.ContractsDir)
	}
	ws := &Workspace{App: app, byPath: map[string]*manifest.Component{}}
	byName := map[string]string{}
	for _, process := range app.Processes {
		if !manifest.Exists(process.ComponentPath) {
			return nil, fmt.Errorf("process %q: component manifest not found: %s", process.Name, process.Component)
		}
		component, err := manifest.ParseComponent(process.ComponentPath)
		if err != nil {
			return nil, err
		}
		if previous, ok := byName[component.Name]; ok && previous != component.Path {
			return nil, fmt.Errorf("duplicate component name %q in %s and %s", component.Name, previous, component.Path)
		}
		byName[component.Name] = component.Path
		ws.byPath[component.Path] = component
	}
	for _, component := range ws.byPath {
		ws.Components = append(ws.Components, component)
	}
	sortComponents(ws.Components)
	return ws, nil
}

func (w *Workspace) Component(path string) (*manifest.Component, bool) {
	component, ok := w.byPath[path]
	return component, ok
}

func (w *Workspace) ProcessComponent(process manifest.Process) (*manifest.Component, bool) {
	return w.Component(process.ComponentPath)
}

func sortComponents(components []*manifest.Component) {
	for i := 1; i < len(components); i++ {
		for j := i; j > 0 && components[j-1].Name > components[j].Name; j-- {
			components[j-1], components[j] = components[j], components[j-1]
		}
	}
}
