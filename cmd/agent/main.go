package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

var AppVersion = "dev"

func main() {
	InitConfig()

	slog.Info("overwatcher-agent",
		"version", AppVersion,
		"coordinator", config.Agent.CoordinatorURL,
		"stacks", len(config.Agent.Stacks),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := NewRunner(config.Agent.Stacks)
	poller, err := NewPoller(config.Agent, runner)
	if err != nil {
		slog.Error("failed to construct poller", "error", err)
		os.Exit(1)
	}

	slog.Info("agent started, polling for intents")
	poller.Run(ctx)
	slog.Info("agent stopped")
}
