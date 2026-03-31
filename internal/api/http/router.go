package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/handler"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service"
)

type Config struct {
	Port uint
}

type Services struct {
	WebhookService *service.WebhookService
	WebhookSecret  string
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())
	engine.Use(middleware.ErrorHandler())

	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(srvs.WebhookService)

	engine.GET("/health", healthHandler.Check)

	apis := engine.Group("/api/v1")
	{
		githubGroup := apis.Group("/github")
		githubGroup.Use(middleware.WebhookSignatureVerifier(srvs.WebhookSecret))
		{
			githubGroup.POST("/webhook", webhookHandler.HandleWebhook)
		}
	}
}
