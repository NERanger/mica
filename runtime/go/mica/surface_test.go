package mica

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSurfaceWrite(t *testing.T) {
	surface := newContractSurface()
	surface.record("subscribes", "jobs.v1.JobCompleted")
	surface.record("provides", "jobs.v1.Worker.Run")
	surface.record("publishes", "audit.v1.JobRecorded")
	surface.record("calls", "jobs.v1.Worker.Run")
	surface.record("calls", "jobs.v1.Worker.Run")

	path := filepath.Join(t.TempDir(), "nested", "client.json")
	if err := surface.write(path, "client"); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	var report struct {
		Component  string   `json:"component"`
		Publishes  []string `json:"publishes"`
		Subscribes []string `json:"subscribes"`
		Calls      []string `json:"calls"`
		Provides   []string `json:"provides"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("malformed surface file: %v", err)
	}
	if report.Component != "client" {
		t.Fatalf("component = %q", report.Component)
	}
	if len(report.Publishes) != 1 || report.Publishes[0] != "audit.v1.JobRecorded" {
		t.Fatalf("publishes = %v", report.Publishes)
	}
	if len(report.Subscribes) != 1 || report.Subscribes[0] != "jobs.v1.JobCompleted" {
		t.Fatalf("subscribes = %v", report.Subscribes)
	}
	if len(report.Calls) != 1 || report.Calls[0] != "jobs.v1.Worker.Run" {
		t.Fatalf("calls = %v", report.Calls)
	}
	if len(report.Provides) != 1 || report.Provides[0] != "jobs.v1.Worker.Run" {
		t.Fatalf("provides = %v", report.Provides)
	}
}
