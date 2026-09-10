package mica

import (
	"os"
	"strconv"
	"time"
)

const (
	ProtocolVersion     = 1
	DefaultTransportURL = "nats://127.0.0.1:4222"
	DefaultRPCTimeout   = 5 * time.Second
)

type AppConfig struct {
	Name         string
	TransportURL string
	RPCTimeout   time.Duration
}

func ConfigFromEnv(name string) AppConfig {
	cfg := AppConfig{
		Name:         name,
		TransportURL: DefaultTransportURL,
		RPCTimeout:   DefaultRPCTimeout,
	}
	if v := os.Getenv("MICA_COMPONENT_NAME"); v != "" {
		cfg.Name = v
	}
	if v := os.Getenv("MICA_NATS_URL"); v != "" {
		cfg.TransportURL = v
	}
	if v := os.Getenv("MICA_RPC_TIMEOUT_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil {
			cfg.RPCTimeout = time.Duration(ms) * time.Millisecond
		}
	}
	return cfg
}
