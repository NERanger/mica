package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func RunSSH(runPath, host string, port int) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("ssh is required for --host")
	}
	if _, err := exec.LookPath("scp"); err != nil {
		return fmt.Errorf("scp is required for --host")
	}
	targetOS, targetArch, err := remotePlatform(host, port)
	if err != nil {
		return err
	}
	if targetOS != runtime.GOOS || targetArch != runtime.GOARCH {
		return fmt.Errorf("target %s is %s/%s but this artifact is %s/%s", host, targetOS, targetArch, runtime.GOOS, runtime.GOARCH)
	}
	remoteDir, err := sshOutput(host, port, "mktemp -d /tmp/mica-deploy.XXXXXX")
	if err != nil {
		return err
	}
	remoteRun := filepath.ToSlash(filepath.Join(remoteDir, filepath.Base(runPath)))
	defer func() {
		_ = sshRun(host, port, false, "rm -rf "+shellQuote(remoteDir))
	}()
	scp := []string{}
	if port > 0 {
		scp = append(scp, "-P", strconv.Itoa(port))
	}
	scp = append(scp, runPath, host+":"+remoteRun)
	if err := runCommand("scp", scp...); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Printf("[mica] executing %s on %s\n", filepath.Base(runPath), host)
	if err := sshRun(host, port, true, "chmod +x "+shellQuote(remoteRun)+" && "+shellQuote(remoteRun)); err != nil {
		return fmt.Errorf("remote execution failed: %w", err)
	}
	return nil
}

func remotePlatform(host string, port int) (string, string, error) {
	out, err := sshOutput(host, port, "uname -s; uname -m")
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("could not detect platform on %s", host)
	}
	return normalizeOS(strings.TrimSpace(lines[0])), normalizeArch(strings.TrimSpace(lines[1])), nil
}

func normalizeOS(value string) string {
	switch strings.ToLower(value) {
	case "linux":
		return "linux"
	case "darwin":
		return "darwin"
	}
	return strings.ToLower(value)
}

func normalizeArch(value string) string {
	switch strings.ToLower(value) {
	case "x86_64", "amd64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	}
	return strings.ToLower(value)
}

func sshArgs(host string, port int, command string) []string {
	args := []string{}
	if port > 0 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	return append(args, host, command)
}

func sshOutput(host string, port int, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", sshArgs(host, port, command)...)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("ssh %s timed out", host)
	}
	if err != nil {
		return "", fmt.Errorf("ssh %s: %w", host, err)
	}
	return string(out), nil
}

func sshRun(host string, port int, interactive bool, command string) error {
	args := sshArgs(host, port, command)
	if interactive {
		args = append([]string{"-tt"}, args...)
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if interactive {
		cmd.Stdin = os.Stdin
	}
	return cmd.Run()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
