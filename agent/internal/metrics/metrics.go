// Package metrics stores network probe results and formats them
// in the Prometheus text exposition format (no external libs).
package metrics

import (
	"fmt"
	"strings"
	"sync"
)

// gauge stores a float64 value per label set.
type gauge struct {
	name   string
	help   string
	mu     sync.RWMutex
	values map[string]float64 // key: target label value
}

func newGauge(name, help string) *gauge {
	return &gauge{name: name, help: help, values: make(map[string]float64)}
}

func (g *gauge) set(target string, v float64) {
	g.mu.Lock()
	g.values[target] = v
	g.mu.Unlock()
}

func (g *gauge) exposition() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var sb strings.Builder
	fmt.Fprintf(&sb, "# HELP %s %s\n", g.name, g.help)
	fmt.Fprintf(&sb, "# TYPE %s gauge\n", g.name)
	for target, val := range g.values {
		fmt.Fprintf(&sb, "%s{target=%q} %g\n", g.name, target, val)
	}
	return sb.String()
}

// Metrics holds all Prometheus-format gauges for the agent.
type Metrics struct {
	up         *gauge
	latencyMs  *gauge
	packetLoss *gauge
}

// New creates and initialises all agent metrics.
func New() *Metrics {
	return &Metrics{
		up:         newGauge("network_up", "Whether the target host is reachable (1 = up, 0 = down)."),
		latencyMs:  newGauge("network_latency_ms", "Average round-trip time to the target in milliseconds."),
		packetLoss: newGauge("network_packet_loss", "Percentage of packets lost (0-100)."),
	}
}

// Update records the latest probe result for a target.
func (m *Metrics) Update(target string, up bool, latencyMs, packetLoss float64) {
	upVal := 0.0
	if up {
		upVal = 1.0
	}
	m.up.set(target, upVal)
	m.latencyMs.set(target, latencyMs)
	m.packetLoss.set(target, packetLoss)
}

// Exposition returns the full Prometheus text format output.
func (m *Metrics) Exposition() string {
	return m.up.exposition() +
		m.latencyMs.exposition() +
		m.packetLoss.exposition()
}
