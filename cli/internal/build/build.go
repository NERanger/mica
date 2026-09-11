package build

import (
	"fmt"
	"os"
	"strings"

	"mica/cli/internal/adapters"
	"mica/cli/internal/contracts"
	"mica/cli/internal/workspace"
)

type Result struct {
	Artifacts map[string]*adapters.Artifact
}

func Run(ws *workspace.Workspace, generate bool) (*Result, error) {
	if len(ws.App.Processes) == 0 {
		return nil, fmt.Errorf("application %s has no processes", ws.App.Name)
	}
	if generate {
		if err := contracts.Generate(ws); err != nil {
			return nil, err
		}
	}
	report, err := contracts.Validate(ws)
	if err != nil {
		return nil, err
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	if !report.OK() {
		return nil, fmt.Errorf("%s", strings.Join(report.Errors, "\n"))
	}
	result := &Result{Artifacts: map[string]*adapters.Artifact{}}
	for _, component := range ws.Components {
		fmt.Printf("[mica] building %s (%s)\n", component.Name, component.Build.Adapter)
		artifact, err := adapters.Build(ws, component)
		if err != nil {
			return nil, err
		}
		result.Artifacts[component.Path] = artifact
	}
	return result, nil
}
