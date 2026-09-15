package spec

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

const RestartOnFailure = "on-failure"

type Deployment struct {
	App          App           `toml:"app"`
	Transport    Transport     `toml:"transport"`
	Platform     Platform      `toml:"platform"`
	Environment  Environment   `toml:"environment"`
	Processes    []Process     `toml:"process"`
	Requirements []Requirement `toml:"requirements"`
}

type App struct {
	Name              string `toml:"name"`
	ShutdownTimeoutMS int    `toml:"shutdown_timeout_ms"`
}

type Transport struct {
	Kind string `toml:"kind"`
	URL  string `toml:"url"`
}

type Platform struct {
	OS   string `toml:"os"`
	Arch string `toml:"arch"`
}

type Environment struct {
	PythonPath []string `toml:"python_path"`
}

type Process struct {
	Name              string            `toml:"name"`
	Component         string            `toml:"component"`
	Command           string            `toml:"command"`
	Args              []string          `toml:"args"`
	Env               map[string]string `toml:"env"`
	WorkingDirectory  string            `toml:"working_directory"`
	Restart           string            `toml:"restart"`
	ShutdownTimeoutMS int               `toml:"shutdown_timeout_ms"`
	Publishes         []string          `toml:"publishes"`
	Subscribes        []string          `toml:"subscribes"`
	Calls             []string          `toml:"calls"`
	Provides          []string          `toml:"provides"`
}

type Requirement struct {
	Component      string   `toml:"component"`
	Executables    []string `toml:"executables"`
	Libraries      []string `toml:"libraries"`
	PythonVersion  string   `toml:"python_version"`
	PythonPackages []string `toml:"python_packages"`
}

func Load(path string) (*Deployment, error) {
	var deployment Deployment
	if _, err := toml.DecodeFile(path, &deployment); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &deployment, nil
}
