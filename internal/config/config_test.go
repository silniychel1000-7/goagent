package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadValid(t *testing.T) {
	f, _ := os.CreateTemp("", "cfg*.yaml")
	f.WriteString("targets:\n  - 8.8.8.8\ninterval: 10s\ntimeout: 3s\nport: 9100\nworkers: 5\n")
	f.Close()
	defer os.Remove(f.Name())

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Targets) != 1 || cfg.Targets[0] != "8.8.8.8" {
		t.Errorf("targets mismatch: %v", cfg.Targets)
	}
	if cfg.Interval != 10*time.Second {
		t.Errorf("interval mismatch: %v", cfg.Interval)
	}
}

func TestLoadNoTargets(t *testing.T) {
	f, _ := os.CreateTemp("", "cfg*.yaml")
	f.WriteString("interval: 5s\ntimeout: 2s\nport: 9100\n")
	f.Close()
	defer os.Remove(f.Name())

	_, err := Load(f.Name())
	if err == nil {
		t.Fatal("expected error for missing targets")
	}
}
