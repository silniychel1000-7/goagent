// Package jitter computes network jitter as the mean absolute deviation
// of successive RTT samples (RFC 3550 definition).
package jitter

import (
	"math"
	"sync"
)

const windowSize = 20

// Calculator maintains a rolling window of RTT samples per target.
type Calculator struct {
	mu      sync.Mutex
	windows map[string][]float64 // target → last N RTT values
}

// New creates a Calculator.
func New() *Calculator {
	return &Calculator{windows: make(map[string][]float64)}
}

// Add records a new RTT sample for target (milliseconds).
func (c *Calculator) Add(target string, rttMs float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.windows[target]
	w = append(w, rttMs)
	if len(w) > windowSize {
		w = w[len(w)-windowSize:]
	}
	c.windows[target] = w
}

// Jitter returns the mean absolute deviation of RTTs for target.
// Returns 0 if fewer than 2 samples are available.
func (c *Calculator) Jitter(target string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.windows[target]
	if len(w) < 2 {
		return 0
	}

	// Mean
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	mean := sum / float64(len(w))

	// Mean absolute deviation
	dev := 0.0
	for _, v := range w {
		dev += math.Abs(v - mean)
	}
	return dev / float64(len(w))
}

// Targets returns all targets that have samples.
func (c *Calculator) Targets() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.windows))
	for k := range c.windows {
		out = append(out, k)
	}
	return out
}
