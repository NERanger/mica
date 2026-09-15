package mica

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type contractSurface struct {
	mu         sync.Mutex
	publishes  map[string]bool
	subscribes map[string]bool
	calls      map[string]bool
	provides   map[string]bool
}

func newContractSurface() *contractSurface {
	return &contractSurface{
		publishes:  map[string]bool{},
		subscribes: map[string]bool{},
		calls:      map[string]bool{},
		provides:   map[string]bool{},
	}
}

func (s *contractSurface) record(kind string, contractID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ids := s.kind(kind); ids != nil {
		ids[contractID] = true
	}
}

func (s *contractSurface) kind(name string) map[string]bool {
	switch name {
	case "publishes":
		return s.publishes
	case "subscribes":
		return s.subscribes
	case "calls":
		return s.calls
	case "provides":
		return s.provides
	}
	return nil
}

func (s *contractSurface) write(path, component string) error {
	s.mu.Lock()
	report := map[string]any{"component": component}
	for _, kind := range []string{"publishes", "subscribes", "calls", "provides"} {
		ids := []string{}
		for id := range s.kind(kind) {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		report[kind] = ids
	}
	s.mu.Unlock()
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
