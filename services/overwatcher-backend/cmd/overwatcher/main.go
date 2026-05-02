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

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	internalhttp "github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
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

	agentSvc := agent.NewService(pool, queries, 60*time.Second)
	eventLogSvc := eventlog.NewService(queries)
	userSvc := user.NewService(pool)
	projectSvc := project.NewService(pool)
	authSvc := auth.NewService(pool, config.Auth.SessionTTL)
	webhookSvc := webhook.New(ghClient, projectSvc, intentStore, eventLogSvc)
	dispatchSvc := dispatch.New(ghClient, intentStore)

	if config.Auth.BootstrapEmail != "" && config.Auth.BootstrapPassword != "" {
		bootstrapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := authSvc.EnsureUserPassword(bootstrapCtx, config.Auth.BootstrapEmail, config.Auth.BootstrapPassword, config.Auth.BootstrapName); err != nil {
			cancel()
			slog.Error("auth bootstrap failed", "error", err)
			os.Exit(1)
		}
		cancel()
		slog.Info("auth bootstrap user ensured", "email", config.Auth.BootstrapEmail)
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
		CookieConfig: middleware.CookieConfig{
			Secure: config.Auth.CookieSecure,
			Domain: config.Auth.CookieDomain,
		},
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
