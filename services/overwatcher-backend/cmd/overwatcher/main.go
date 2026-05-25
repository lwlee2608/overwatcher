package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	internalhttp "github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
	"github.com/lwlee2608/overwatcher/internal/service/auth"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/project"
	"github.com/lwlee2608/overwatcher/internal/service/user"
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

	queries := sqlc.New(pool)
	intentStore := intent.NewDBStore(pool)

	agentSvc := agentregistry.NewService(pool, queries, 30*time.Second, time.Hour, 12*time.Hour)
	eventLogSvc := eventlog.NewService(queries)
	userSvc := user.NewService(pool)
	projectSvc := project.NewService(pool)
	authSvc := auth.NewService(pool, config.Auth.SessionTTL)
	webhookSvc := webhook.New(ghClient, projectSvc, intentStore, eventLogSvc)
	dispatchSvc := dispatch.New(ghClient, intentStore)

	if config.Auth.Bootstrap.Enabled() {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := authSvc.EnsureUserPassword(bootstrapCtx, config.Auth.Bootstrap); err != nil {
			cancel()
			slog.Error("auth bootstrap failed", "error", err)
			os.Exit(1)
		}
		cancel()
		slog.Info("auth bootstrap user ensured", "email", config.Auth.Bootstrap.Email)
	}

	services := &internalhttp.Services{
		WebhookService:    webhookSvc,
		DispatchService:   dispatchSvc,
		AgentService:      agentSvc,
		EventLogService:   eventLogSvc,
		UserService:       userSvc,
		ProjectService:    projectSvc,
		AuthService:       authSvc,
		WebhookSecret:     config.GitHub.WebhookSecret,
		AgentSharedSecret: config.Agent.SharedSecret,
		AgentReleaseTag:   config.Agent.ReleaseTag,
		AgentPublicURL:    config.Agent.PublicURL,
		CookieConfig:      config.Auth.Cookie,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reaper := dispatchSvc.NewReaper(
		config.Dispatch.InFlightTimeout,
		config.Dispatch.MaxAttempts,
		config.Dispatch.SweepInterval,
	)
	go reaper.Run(ctx)
	go runSessionReaper(ctx, authSvc, time.Hour)

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
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

func runSessionReaper(ctx context.Context, svc *auth.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.ReapExpiredSessions(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("session reaper failed", "error", err)
			}
		}
	}
}
