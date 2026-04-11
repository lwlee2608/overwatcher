package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	internalhttp "github.com/lwlee2608/overwatcher/internal/api/http"
	internalgithub "github.com/lwlee2608/overwatcher/internal/github"
	"github.com/lwlee2608/overwatcher/internal/service"
)

var AppVersion = "dev"

func main() {
	InitConfig()

	slog.Info("overwatcher", "version", AppVersion)

	ghClient := internalgithub.NewClient(config.GitHub.AppID, []byte(config.GitHub.PrivateKey))
	mapping := service.NewMapping(config.Deployments.Mappings)
	intentStore := service.NewIntentStore()
	webhookSvc := service.NewWebhookService(ghClient, mapping, intentStore)
	dispatchSvc := service.NewDispatchService(ghClient, intentStore)

	services := &internalhttp.Services{
		WebhookService:    webhookSvc,
		DispatchService:   dispatchSvc,
		WebhookSecret:     config.GitHub.WebhookSecret,
		AgentSharedSecret: config.Agent.SharedSecret,
	}

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

	slog.Info("Starting HTTP server", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("HTTP server error", "error", err)
	}
}
