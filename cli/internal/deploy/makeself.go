package deploy

import (
	"fmt"
	"os"
	"os/exec"
)

func Package(staging, output, label string) error {
	makeself := os.Getenv("MICA_MAKESELF")
	if makeself == "" {
		makeself = "makeself"
	}
	if _, err := exec.LookPath(makeself); err != nil {
		return fmt.Errorf("makeself is required for deploy; install it or set MICA_MAKESELF")
	}
	cmd := exec.Command(makeself, "--gzip", "--sha256", "--nox11", staging, output, label, "./startup.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("makeself failed: %w", err)
	}
	return nil
}
