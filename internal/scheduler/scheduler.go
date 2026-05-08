// Package scheduler manages periodic, parallel monitoring of all targets.
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/netmon/agent/internal/jitter"
	"github.com/netmon/agent/internal/logger"
	"github.com/netmon/agent/internal/metrics"
	"github.com/netmon/agent/internal/ping"
)

// Scheduler manages periodic, parallel monitoring of all targets.
type Scheduler struct {
	targets  []string
	interval time.Duration
	workers  int
	pinger   *ping.Pinger
	metrics  *metrics.Metrics
	jitter   *jitter.Calculator
	log      *logger.Logger
}

// New creates a Scheduler.
func New(
	targets []string,
	interval time.Duration,
	workers int,
	pinger *ping.Pinger,
	m *metrics.Metrics,
	j *jitter.Calculator,
	log *logger.Logger,
) *Scheduler {
	return &Scheduler{
		targets:  targets,
		interval: interval,
		workers:  workers,
		pinger:   pinger,
		metrics:  m,
		jitter:   j,
		log:      log,
	}
}

// Run starts the monitoring loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info("scheduler started",
		"targets", len(s.targets),
		"interval", s.interval,
		"workers", s.workers,
	)

	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce fans out one monitoring round across all targets using a worker pool.
func (s *Scheduler) runOnce(ctx context.Context) {
	taskCh := make(chan string, len(s.targets))
	for _, t := range s.targets {
		taskCh <- t
	}
	close(taskCh)

	workerCount := s.workers
	if workerCount > len(s.targets) {
		workerCount = len(s.targets)
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range taskCh {
				select {
				case <-ctx.Done():
					return
				default:
					s.check(target)
				}
			}
		}()
	}
	wg.Wait()
}

// check pings a single target, updates jitter and metrics.
func (s *Scheduler) check(target string) {
	result := s.pinger.Ping(target)

	if result.Up && result.LatencyMs > 0 {
		s.jitter.Add(target, result.LatencyMs)
	}
	j := s.jitter.Jitter(target)

	s.metrics.Update(target, result.Up, result.LatencyMs, result.PacketLoss, j)

	switch {
	case result.Error != nil:
		s.log.Error("ping error", "target", target, "error", result.Error)
	case !result.Up:
		s.log.Warn("host unreachable", "target", target, "packet_loss", result.PacketLoss)
	default:
		s.log.Debug("ping ok", "target", target,
			"latency_ms", result.LatencyMs,
			"packet_loss", result.PacketLoss,
			"jitter_ms", j,
		)
	}
}
