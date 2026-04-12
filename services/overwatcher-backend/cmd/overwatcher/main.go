package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	internalhttp "github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/db"
	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
)

var AppVersion = "dev"

func main() {
	if err := InitConfig(); err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	slog.Info("overwatcher", "version", AppVersion)

	ghClient := internalgithub.NewClient(config.GitHub.AppID, []byte(config.GitHub.PrivateKey))
	mappingIdx := mapping.New(config.Deployments.Mappings)

	if err := db.RunMigrations(config.Database.URL, config.Database.Schema); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	pool, err := db.InitDB(context.Background(), config.Database)
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	intentStore := intent.NewDBStore(pool)

	webhookSvc := webhook.New(ghClient, mappingIdx, intentStore)
	dispatchSvc := dispatch.New(ghClient, intentStore)

	services := &internalhttp.Services{
		WebhookService:    webhookSvc,
		DispatchService:   dispatchSvc,
		WebhookSecret:     config.GitHub.WebhookSecret,
		AgentSharedSecret: config.Agent.SharedSecret,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reaper := dispatchSvc.NewReaper(
		config.Dispatch.InFlightTimeout,
		config.Dispatch.MaxAttempts,
		config.Dispatch.SweepInterval,
	)
	go reaper.Run(ctx)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	engine.Use(gin.Recovery())
	internalhttp.SetupRoute(engine, services)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Http.Port),
		Handler: engine,
	}

	go func() {
		slog.Info("Starting HTTP server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop()

	slog.Info("Shutting down gracefully", "timeout", config.Dispatch.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), config.Dispatch.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}
	slog.Info("Shutdown complete")
}
