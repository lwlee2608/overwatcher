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
	if err := InitConfig(); err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	slog.Info("overwatcher-agent",
		"version", AppVersion,
		"coordinator", config.Agent.CoordinatorURL,
		"compose_file", config.Agent.ComposeFile,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := NewRunner(config.Agent.ComposeFile)
	poller, err := NewPoller(config.Agent, runner)
	if err != nil {
		slog.Error("failed to construct poller", "error", err)
		os.Exit(1)
	}

	slog.Info("agent started, polling for intents")
	poller.Run(ctx)
	slog.Info("agent stopped")
}
