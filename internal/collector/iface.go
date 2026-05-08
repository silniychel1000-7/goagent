// Package collector provides network interface metrics via /proc/net/dev.
// No external libraries required — pure stdlib.
package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// InterfaceStats holds a snapshot of interface counters.
type InterfaceStats struct {
	Name      string
	RxBytes   uint64
	TxBytes   uint64
	RxPackets uint64
	TxPackets uint64
	RxErrors  uint64
	TxErrors  uint64
	RxDrops   uint64
	TxDrops   uint64
}

// InterfaceCollector reads /proc/net/dev and exposes interface metrics.
type InterfaceCollector struct {
	mu      sync.RWMutex
	current map[string]InterfaceStats
	prev    map[string]InterfaceStats
}

// NewInterfaceCollector creates a ready-to-use collector.
func NewInterfaceCollector() *InterfaceCollector {
	return &InterfaceCollector{
		current: make(map[string]InterfaceStats),
		prev:    make(map[string]InterfaceStats),
	}
}

// Collect reads /proc/net/dev and stores the latest counters.
func (c *InterfaceCollector) Collect() error {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	newStats := make(map[string]InterfaceStats)
	scanner := bufio.NewScanner(f)

	// Skip two header lines.
	for i := 0; i < 2 && scanner.Scan(); i++ {
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colonIdx])
		fields := strings.Fields(line[colonIdx+1:])
		if len(fields) < 16 {
			continue
		}

		st := InterfaceStats{Name: name}
		vals := []*uint64{
			&st.RxBytes, &st.RxPackets, &st.RxErrors, &st.RxDrops,
			nil, nil, nil, nil, // fifo, frame, compressed, multicast
			&st.TxBytes, &st.TxPackets, &st.TxErrors, &st.TxDrops,
		}
		for i, ptr := range vals {
			if ptr == nil {
				continue
			}
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err == nil {
				*ptr = v
			}
		}
		newStats[name] = st
	}

	c.mu.Lock()
	c.prev = c.current
	c.current = newStats
	c.mu.Unlock()

	return scanner.Err()
}

// Stats returns a snapshot of current interface statistics.
func (c *InterfaceCollector) Stats() []InterfaceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]InterfaceStats, 0, len(c.current))
	for _, s := range c.current {
		out = append(out, s)
	}
	return out
}

// RxBytesRate returns bytes/s for the given interface since last Collect call.
// Returns 0 if the interface was not seen in the previous snapshot.
func (c *InterfaceCollector) RxBytesRate(iface string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cur, ok1 := c.current[iface]
	prev, ok2 := c.prev[iface]
	if !ok1 || !ok2 {
		return 0
	}
	if cur.RxBytes < prev.RxBytes {
		return 0 // counter wrap
	}
	return float64(cur.RxBytes - prev.RxBytes)
}

// TxBytesRate returns bytes/s for the given interface since last Collect call.
func (c *InterfaceCollector) TxBytesRate(iface string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cur, ok1 := c.current[iface]
	prev, ok2 := c.prev[iface]
	if !ok1 || !ok2 {
		return 0
	}
	if cur.TxBytes < prev.TxBytes {
		return 0
	}
	return float64(cur.TxBytes - prev.TxBytes)
}

// Exposition returns Prometheus text format for all interface metrics.
func (c *InterfaceCollector) Exposition() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sb strings.Builder
	metrics := []struct {
		name   string
		help   string
		getter func(InterfaceStats) float64
	}{
		{"iface_rx_bytes_total", "Total bytes received.", func(s InterfaceStats) float64 { return float64(s.RxBytes) }},
		{"iface_tx_bytes_total", "Total bytes transmitted.", func(s InterfaceStats) float64 { return float64(s.TxBytes) }},
		{"iface_rx_errors_total", "Total receive errors.", func(s InterfaceStats) float64 { return float64(s.RxErrors) }},
		{"iface_tx_errors_total", "Total transmit errors.", func(s InterfaceStats) float64 { return float64(s.TxErrors) }},
		{"iface_rx_drops_total", "Total receive drops.", func(s InterfaceStats) float64 { return float64(s.RxDrops) }},
		{"iface_tx_drops_total", "Total transmit drops.", func(s InterfaceStats) float64 { return float64(s.TxDrops) }},
	}

	for _, m := range metrics {
		fmt.Fprintf(&sb, "# HELP %s %s\n# TYPE %s counter\n", m.name, m.help, m.name)
		for _, s := range c.current {
			if s.Name == "lo" {
				continue // skip loopback
			}
			fmt.Fprintf(&sb, "%s{iface=%q} %g\n", m.name, s.Name, m.getter(s))
		}
	}
	return sb.String()
}
