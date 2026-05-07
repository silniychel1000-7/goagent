package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/netmon/agent/internal/config"
	"github.com/netmon/agent/internal/exporter"
	"github.com/netmon/agent/internal/logger"
	"github.com/netmon/agent/internal/metrics"
	"github.com/netmon/agent/internal/ping"
	"github.com/netmon/agent/internal/scheduler"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "Path to YAML config file")
	flag.Parse()

	log := logger.New(logger.INFO)
	log.Info("network monitoring agent starting")

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	log.Info("config loaded",
		"targets", len(cfg.Targets),
		"interval", cfg.Interval,
		"timeout", cfg.Timeout,
	)

	m := metrics.New()
	pinger := ping.New(cfg.Timeout)
	sched := scheduler.New(cfg.Targets, cfg.Interval, cfg.Workers, pinger, m, log)
	exp := exporter.New(cfg.Port, m, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	go sched.Run(ctx)

	if err := exp.Start(ctx); err != nil {
		log.Error("exporter error", "error", err)
		os.Exit(1)
	}

	log.Info("agent stopped")
}
