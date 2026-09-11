package deploy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

func startNATSServer(url string) (*exec.Cmd, error) {
	if !strings.HasPrefix(url, "nats://") {
		return nil, fmt.Errorf("unsupported transport url: %s", url)
	}
	hostport := strings.TrimPrefix(url, "nats://")
	if index := strings.Index(hostport, "@"); index >= 0 {
		hostport = hostport[index+1:]
	}
	host, port := hostport, "4222"
	if index := strings.LastIndex(hostport, ":"); index >= 0 {
		host = hostport[:index]
		port = hostport[index+1:]
	}
	if host == "" {
		host = "127.0.0.1"
	}
	binary := os.Getenv("NATS_SERVER")
	if binary == "" {
		binary = "nats-server"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return nil, fmt.Errorf("nats-server is required for --start-nats; install it or set NATS_SERVER")
	}
	cmd := exec.Command(binary, "-a", host, "-p", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	address := net.JoinHostPort(host, port)
	for i := 0; i < 50; i++ {
		if cmd.ProcessState != nil {
			return nil, fmt.Errorf("nats-server exited before becoming ready")
		}
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return cmd, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	return nil, fmt.Errorf("nats-server did not become ready at %s", address)
}
