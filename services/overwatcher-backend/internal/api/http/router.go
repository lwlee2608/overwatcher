package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/handler"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
	"github.com/lwlee2608/overwatcher/internal/service/project"
	"github.com/lwlee2608/overwatcher/internal/service/user"
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
	EventLogService   *eventlog.Service
	UserService       *user.Service
	ProjectService    *project.Service
	WebhookSecret     string
	AgentSharedSecret string
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())
	engine.Use(middleware.ErrorHandler())

	healthHandler := handler.NewHealthHandler()
	webhookHandler := handler.NewWebhookHandler(srvs.WebhookService)
	deployHandler := handler.NewDeployHandler(srvs.DispatchService, srvs.WebhookService)
	agentHandler := handler.NewAgentHandler(srvs.AgentService)
	mappingHandler := handler.NewMappingHandler(srvs.MappingService)
	eventLogHandler := handler.NewEventLogHandler(srvs.EventLogService)
	userHandler := handler.NewUserHandler(srvs.UserService)
	projectHandler := handler.NewProjectHandler(srvs.ProjectService)

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
		apis.PUT("/agents/:id/project", agentHandler.BindProject)

		apis.GET("/mappings", mappingHandler.List)
		apis.POST("/mappings", mappingHandler.Create)
		apis.PUT("/mappings/:id", mappingHandler.Update)
		apis.DELETE("/mappings/:id", mappingHandler.Delete)

		apis.GET("/users", userHandler.List)
		apis.POST("/users", userHandler.Create)
		apis.GET("/users/:id", userHandler.Get)
		apis.PUT("/users/:id", userHandler.Update)
		apis.DELETE("/users/:id", userHandler.Delete)

		apis.GET("/projects", projectHandler.List)
		apis.POST("/projects", projectHandler.Create)
		apis.GET("/projects/:id", projectHandler.Get)
		apis.PUT("/projects/:id", projectHandler.Update)
		apis.DELETE("/projects/:id", projectHandler.Delete)
		apis.GET("/projects/:id/services", projectHandler.ListServices)
		apis.POST("/projects/:id/services", projectHandler.CreateService)
		apis.PUT("/projects/:id/services", projectHandler.ReplaceServices)
		apis.DELETE("/projects/:id/services/:serviceID", projectHandler.DeleteService)

		apis.GET("/events", eventLogHandler.List)
		apis.GET("/deployments", deployHandler.ListDeployments)
		apis.POST("/deployments/:id/redeploy", deployHandler.Redeploy)
	}
}
