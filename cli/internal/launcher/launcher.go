package launcher

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"mica/cli/internal/spec"
)

func Run(deploymentPath string) int {
	abs, err := filepath.Abs(deploymentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	dep, err := spec.Load(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	root := filepath.Dir(abs)
	if failures := checkRequirements(dep, root); len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "mica: target prerequisites are not satisfied:")
		for _, failure := range failures {
			fmt.Fprintf(os.Stderr, "  - %s\n", failure)
		}
		return 1
	}
	return supervise(dep, root)
}

type child struct {
	spec   spec.Process
	cmd    *exec.Cmd
	exited chan struct{}
	code   int
}

func supervise(dep *spec.Deployment, root string) int {
	var children []*child
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	var stopping bool
	var stopMu sync.Mutex
	setStopping := func() {
		stopMu.Lock()
		stopping = true
		stopMu.Unlock()
	}
	isStopping := func() bool {
		stopMu.Lock()
		defer stopMu.Unlock()
		return stopping
	}

	exitCode := 0
	spawn := func(process spec.Process) (*child, error) {
		command := resolve(root, process.Command)
		cmd := exec.Command(command, process.Args...)
		cmd.Dir = resolveDir(root, process.WorkingDirectory)
		cmd.Env = childEnv(dep, process, root)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		item := &child{spec: process, cmd: cmd, exited: make(chan struct{})}
		fmt.Printf("[%s] started\n", process.Name)
		go func() {
			prefixStream(process.Name, stdout, os.Stdout)
		}()
		go func() {
			err := cmd.Wait()
			item.code = exitCodeOf(err)
			close(item.exited)
		}()
		return item, nil
	}

	for _, process := range dep.Processes {
		item, err := spawn(process)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start process: %v\n", err)
			exitCode = 1
			setStopping()
			break
		}
		children = append(children, item)
		time.Sleep(200 * time.Millisecond)
	}

	for !isStopping() {
		select {
		case <-stop:
			setStopping()
		default:
		}
		if isStopping() {
			break
		}
		restarted := false
		remaining := children[:0]
		for _, item := range children {
			select {
			case <-item.exited:
				fmt.Printf("[%s] exited with %d\n", item.spec.Name, item.code)
				if item.spec.Restart == spec.RestartOnFailure && item.code != 0 && !isStopping() {
					fmt.Printf("[%s] restarting\n", item.spec.Name)
					replacement, err := spawn(item.spec)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to restart process: %v\n", err)
						exitCode = 1
						setStopping()
						continue
					}
					remaining = append(remaining, replacement)
					restarted = true
					continue
				}
				if item.code != 0 {
					exitCode = 1
				}
				setStopping()
			default:
				remaining = append(remaining, item)
			}
		}
		children = remaining
		if restarted {
			continue
		}
		if !isStopping() {
			time.Sleep(100 * time.Millisecond)
		}
	}

	timeout := time.Duration(dep.App.ShutdownTimeoutMS) * time.Millisecond
	for _, item := range children {
		select {
		case <-item.exited:
			continue
		default:
		}
		terminate(item)
	}
	deadline := time.Now().Add(timeout)
	for _, item := range children {
		select {
		case <-item.exited:
			fmt.Printf("[%s] stopped\n", item.spec.Name)
		case <-time.After(time.Until(deadline)):
			kill(item)
			<-item.exited
			fmt.Printf("[%s] killed\n", item.spec.Name)
		}
	}
	return exitCode
}

func childEnv(dep *spec.Deployment, process spec.Process, root string) []string {
	env := os.Environ()
	for key, value := range process.Env {
		env = append(env, key+"="+value)
	}
	env = append(env,
		"MICA_NATS_URL="+dep.Transport.URL,
		"MICA_APP_NAME="+dep.App.Name,
		"MICA_COMPONENT_NAME="+process.Name,
	)
	var pythonPath []string
	for _, entry := range dep.Environment.PythonPath {
		pythonPath = append(pythonPath, resolveDir(root, entry))
	}
	if existing := os.Getenv("PYTHONPATH"); existing != "" {
		pythonPath = append(pythonPath, existing)
	}
	if len(pythonPath) > 0 {
		env = append(env, "PYTHONPATH="+strings.Join(pythonPath, string(os.PathListSeparator)))
	}
	return env
}

func prefixStream(name string, source io.Reader, dest io.Writer) {
	reader := bufio.NewReader(source)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			fmt.Fprintf(dest, "[%s] %s", name, line)
		}
		if err != nil {
			return
		}
	}
}

func terminate(item *child) {
	if item.cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-item.cmd.Process.Pid, syscall.SIGTERM); err != nil {
		_ = item.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func kill(item *child) {
	if item.cmd.Process == nil {
		return
	}
	if err := syscall.Kill(-item.cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = item.cmd.Process.Kill()
	}
}

func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}
