package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/handler"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
)

type Config struct {
	Port uint
}

type Services struct {
	WebhookService    *webhook.Service
	DispatchService   *dispatch.Service
	AgentService      *agent.Service
	MappingService    *mapping.Service
	WebhookSecret     string
	AgentSharedSecret string
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())
	engine.Use(middleware.ErrorHandler())

	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(srvs.WebhookService)
	deployHandler := handler.NewDeployHandler(srvs.DispatchService)
	agentHandler := handler.NewAgentHandler(srvs.AgentService)
	mappingHandler := handler.NewMappingHandler(srvs.MappingService)

	engine.GET("/health", healthHandler.Check)

	apis := engine.Group("/api/v1")
	{
		githubGroup := apis.Group("/github")
		githubGroup.Use(middleware.WebhookSignatureVerifier(srvs.WebhookSecret))
		{
			githubGroup.POST("/webhook", webhookHandler.HandleWebhook)
		}

		deployGroup := apis.Group("/deploy")
		deployGroup.Use(middleware.BearerTokenAuth(srvs.AgentSharedSecret))
		deployGroup.Use(middleware.AgentHeartbeat(srvs.AgentService))
		{
			deployGroup.GET("/next", deployHandler.Next)
			deployGroup.POST("/:id/result", deployHandler.Result)
		}

		apis.GET("/agents", agentHandler.List)
		apis.GET("/agents/:id", agentHandler.Get)

		apis.GET("/mappings", mappingHandler.List)
		apis.POST("/mappings", mappingHandler.Create)
		apis.PUT("/mappings/:id", mappingHandler.Update)
		apis.DELETE("/mappings/:id", mappingHandler.Delete)
	}
}
