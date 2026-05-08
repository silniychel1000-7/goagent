// Package collector provides TCP port reachability probes.
package collector

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// TCPResult is the outcome of a single TCP probe.
type TCPResult struct {
	Target  string
	Port    string
	Service string
	Up      bool
	RTT     float64 // ms
}

// TCPProbe checks whether a TCP port is reachable on a target.
type TCPProbe struct {
	timeout time.Duration
	mu      sync.RWMutex
	results map[string]TCPResult // key: "target:port"
}

// WellKnownPorts maps service names to ports we probe by default.
var WellKnownPorts = map[string]string{
	"ssh":  "22",
	"http": "80",
	"dns":  "53",
}

// NewTCPProbe creates a TCPProbe with the given per-dial timeout.
func NewTCPProbe(timeout time.Duration) *TCPProbe {
	return &TCPProbe{
		timeout: timeout,
		results: make(map[string]TCPResult),
	}
}

// Probe checks all well-known ports on the given targets concurrently.
func (p *TCPProbe) Probe(targets []string) {
	type job struct {
		target  string
		port    string
		service string
	}

	jobs := make([]job, 0, len(targets)*len(WellKnownPorts))
	for _, t := range targets {
		for svc, port := range WellKnownPorts {
			jobs = append(jobs, job{t, port, svc})
		}
	}

	var wg sync.WaitGroup
	resultCh := make(chan TCPResult, len(jobs))

	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			resultCh <- p.dialOne(j.target, j.port, j.service)
		}(j)
	}

	wg.Wait()
	close(resultCh)

	p.mu.Lock()
	for r := range resultCh {
		key := r.Target + ":" + r.Port
		p.results[key] = r
	}
	p.mu.Unlock()
}

func (p *TCPProbe) dialOne(target, port, service string) TCPResult {
	addr := net.JoinHostPort(target, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, p.timeout)
	rtt := float64(time.Since(start)) / float64(time.Millisecond)

	if err != nil {
		return TCPResult{Target: target, Port: port, Service: service, Up: false}
	}
	conn.Close()
	return TCPResult{Target: target, Port: port, Service: service, Up: true, RTT: rtt}
}

// Exposition returns Prometheus text format for all TCP probe results.
func (p *TCPProbe) Exposition() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var sb strings.Builder

	fmt.Fprintf(&sb, "# HELP tcp_up Whether the TCP port is reachable (1=up, 0=down).\n")
	fmt.Fprintf(&sb, "# TYPE tcp_up gauge\n")
	for _, r := range p.results {
		up := 0.0
		if r.Up {
			up = 1.0
		}
		fmt.Fprintf(&sb, "tcp_up{target=%q,port=%q,service=%q} %g\n", r.Target, r.Port, r.Service, up)
	}

	fmt.Fprintf(&sb, "# HELP tcp_rtt_ms TCP dial round-trip time in milliseconds.\n")
	fmt.Fprintf(&sb, "# TYPE tcp_rtt_ms gauge\n")
	for _, r := range p.results {
		if r.Up {
			fmt.Fprintf(&sb, "tcp_rtt_ms{target=%q,port=%q,service=%q} %g\n", r.Target, r.Port, r.Service, r.RTT)
		}
	}

	return sb.String()
}
