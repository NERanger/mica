package testrun

import (
	"fmt"
	"os"
	"os/exec"

	"mica/cli/internal/adapters"
	"mica/cli/internal/build"
	"mica/cli/internal/workspace"
)

func Run(ws *workspace.Workspace, generate bool) error {
	if _, err := build.Run(ws, generate); err != nil {
		return err
	}
	for _, component := range ws.Components {
		fmt.Printf("[mica] testing %s\n", component.Name)
		if err := adapters.Test(ws, component); err != nil {
			return err
		}
	}
	if ws.App.Test == nil || ws.App.Test.Command == "" {
		fmt.Println("[mica] no application tests")
		return nil
	}
	fmt.Printf("[mica] application tests: %s\n", ws.App.Test.Command)
	cmd := exec.Command(ws.App.Test.Command, ws.App.Test.Args...)
	cmd.Dir = ws.App.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("application tests failed: %w", err)
	}
	return nil
}
