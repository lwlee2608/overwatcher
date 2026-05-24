package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lwlee2608/overwatcher/internal/agent"
)

var AppVersion = "dev"

func main() {
	cfg, err := agent.InitConfig()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	slog.Info("overwatcher-agent",
		"version", AppVersion,
		"coordinator", cfg.Agent.CoordinatorURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := agent.NewRunner()
	poller, err := agent.NewPoller(cfg.Agent, runner, AppVersion)
	if err != nil {
		slog.Error("failed to construct poller", "error", err)
		os.Exit(1)
	}

	slog.Info("agent started, polling for intents")
	poller.Run(ctx)
	slog.Info("agent stopped")
}
