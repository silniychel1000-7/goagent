// Package exporter serves the Prometheus /metrics HTTP endpoint.
package exporter

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/netmon/agent/internal/logger"
	"github.com/netmon/agent/internal/metrics"
)

// Exporter serves Prometheus metrics over HTTP.
type Exporter struct {
	port    int
	metrics *metrics.Metrics
	log     *logger.Logger
	server  *http.Server
}

// New creates an Exporter that will listen on the given port.
func New(port int, m *metrics.Metrics, log *logger.Logger) *Exporter {
	e := &Exporter{port: port, metrics: m, log: log}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", e.handleMetrics)
	mux.HandleFunc("/healthz", e.handleHealth)

	e.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return e
}

func (e *Exporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprint(w, e.metrics.Exposition())
}

func (e *Exporter) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

// Start begins serving HTTP. Blocks until ctx is cancelled.
func (e *Exporter) Start(ctx context.Context) error {
	e.log.Info("exporter started", "addr", e.server.Addr)

	errCh := make(chan error, 1)
	go func() {
		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("exporter: %w", err)
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e.log.Info("exporter shutting down")
		return e.server.Shutdown(shutCtx)
	}
}
