// Package config loads and validates the agent YAML configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// Config holds the full application configuration.
type Config struct {
	Targets  []string      `yaml:"targets"`
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Port     int           `yaml:"port"`
	Workers  int           `yaml:"workers"`
}

// Load reads and validates a YAML config file using a minimal hand-written parser
// so no external yaml library is required.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	cfg := &Config{
		Interval: 5 * time.Second,
		Timeout:  2 * time.Second,
		Port:     9100,
		Workers:  10,
	}

	if err := parseYAML(data, cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}

	return cfg, cfg.validate()
}

// parseYAML is a simple, purpose-built YAML parser for our config schema.
// It handles a flat key:value structure plus a targets list.
func parseYAML(data []byte, cfg *Config) error {
	lines := splitLines(data)
	inTargets := false

	for _, raw := range lines {
		line := trimComment(raw)
		if len(line) == 0 {
			continue
		}

		// List item under targets:
		if inTargets && len(line) >= 2 && line[0] == '-' {
			target := trim(line[1:])
			if target != "" {
				cfg.Targets = append(cfg.Targets, target)
			}
			continue
		}

		// Key: value pair
		k, v, ok := splitKV(line)
		if !ok {
			inTargets = false
			continue
		}

		inTargets = false
		switch k {
		case "targets":
			// value may be empty; items follow on next lines
			inTargets = true
		case "interval":
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("interval: %w", err)
			}
			cfg.Interval = d
		case "timeout":
			d, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("timeout: %w", err)
			}
			cfg.Timeout = d
		case "port":
			n, err := parseInt(v)
			if err != nil {
				return fmt.Errorf("port: %w", err)
			}
			cfg.Port = n
		case "workers":
			n, err := parseInt(v)
			if err != nil {
				return fmt.Errorf("workers: %w", err)
			}
			cfg.Workers = n
		}
	}
	return nil
}

func (c *Config) validate() error {
	if len(c.Targets) == 0 {
		return errors.New("config: no targets specified")
	}
	if c.Interval <= 0 {
		return errors.New("config: interval must be positive")
	}
	if c.Timeout <= 0 {
		return errors.New("config: timeout must be positive")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("config: port must be between 1 and 65535")
	}
	if c.Workers <= 0 {
		c.Workers = 10
	}
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

func trimComment(s string) string {
	for i, c := range s {
		if c == '#' {
			s = s[:i]
			break
		}
	}
	return trim(s)
}

func trim(s string) string {
	// trim spaces and tabs from both ends
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func splitKV(line string) (string, string, bool) {
	for i := 0; i < len(line); i++ {
		if line[i] == ':' {
			k := trim(line[:i])
			v := ""
			if i+1 < len(line) {
				v = trim(line[i+1:])
			}
			return k, v, true
		}
	}
	return "", "", false
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
