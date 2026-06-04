package http

import (
	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/handler"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
	"github.com/lwlee2608/overwatcher/internal/service/auth"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
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
	AgentService      *agentregistry.Service
	EventLogService   *eventlog.Service
	UserService       *user.Service
	ProjectService    *project.Service
	AuthService       *auth.Service
	WebhookSecret   string
	AppVersion      string
	AgentReleaseTag string
	AgentPublicURL  string
	CookieConfig    middleware.CookieConfig
}

func SetupRoute(engine *gin.Engine, srvs *Services) {
	engine.Use(middleware.RequestLogger())
	engine.Use(middleware.ErrorHandler())

	healthHandler := handler.NewHealthHandler()
	versionHandler := handler.NewVersionHandler(srvs.AppVersion, srvs.AgentReleaseTag)
	installHandler := handler.NewInstallHandler(srvs.AgentReleaseTag, srvs.AgentPublicURL)
	webhookHandler := handler.NewWebhookHandler(srvs.WebhookService)
	deployHandler := handler.NewDeployHandler(srvs.DispatchService, srvs.WebhookService, srvs.ProjectService)
	agentHandler := handler.NewAgentHandler(srvs.AgentService, srvs.ProjectService)
	eventLogHandler := handler.NewEventLogHandler(srvs.EventLogService)
	userHandler := handler.NewUserHandler(srvs.UserService)
	projectHandler := handler.NewProjectHandler(srvs.ProjectService)
	authHandler := handler.NewAuthHandler(srvs.AuthService, srvs.CookieConfig)

	engine.GET("/health", healthHandler.Check)
	// Public: piped into bash before any credentials exist on the VM.
	engine.GET("/install.sh", installHandler.Serve)

	apis := engine.Group("/api/v1")
	{
		githubGroup := apis.Group("/github")
		githubGroup.Use(middleware.WebhookSignatureVerifier(srvs.WebhookSecret))
		{
			githubGroup.POST("/webhook", webhookHandler.HandleWebhook)
		}

		deployGroup := apis.Group("/deploy")
		deployGroup.Use(middleware.AgentTokenAuth(srvs.AgentService))
		deployGroup.Use(middleware.AgentHeartbeat(srvs.AgentService))
		{
			deployGroup.GET("/next", deployHandler.Next)
			deployGroup.POST("/:id/result", deployHandler.Result)
		}

		// /auth/login and /auth/logout are public (no session required).
		// Other /auth/* routes are session-protected via the group below.
		apis.POST("/auth/login", authHandler.Login)
		apis.POST("/auth/logout", authHandler.Logout)

		ui := apis.Group("")
		ui.Use(middleware.SessionAuth(srvs.AuthService, srvs.CookieConfig))
		{
			ui.GET("/auth/me", authHandler.Me)
			ui.PUT("/auth/password", authHandler.ChangePassword)

			ui.GET("/version", versionHandler.Get)

			ui.POST("/agents", agentHandler.Create)
			ui.GET("/agents", agentHandler.List)
			ui.GET("/agents/:id", agentHandler.Get)
			ui.POST("/agents/:id/token", agentHandler.ReissueToken)
			ui.PUT("/agents/:id/project", agentHandler.BindProject)
			ui.DELETE("/agents/:id", agentHandler.Delete)

			ui.GET("/users", userHandler.List)
			ui.POST("/users", userHandler.Create)
			ui.GET("/users/:id", userHandler.Get)
			ui.PUT("/users/:id", userHandler.Update)
			ui.DELETE("/users/:id", userHandler.Delete)

			ui.GET("/projects", projectHandler.List)
			ui.POST("/projects", projectHandler.Create)
			ui.GET("/projects/:id", projectHandler.Get)
			ui.PUT("/projects/:id", projectHandler.Update)
			ui.DELETE("/projects/:id", projectHandler.Delete)
			ui.GET("/projects/:id/services", projectHandler.ListServices)
			ui.POST("/projects/:id/services", projectHandler.CreateService)
			ui.PUT("/projects/:id/services", projectHandler.ReplaceServices)
			ui.DELETE("/projects/:id/services/:serviceID", projectHandler.DeleteService)
			ui.GET("/projects/:id/members", projectHandler.ListMembers)
			ui.POST("/projects/:id/members", projectHandler.AddMember)
			ui.DELETE("/projects/:id/members/:userID", projectHandler.RemoveMember)

			ui.GET("/events", eventLogHandler.List)
			ui.GET("/deployments", deployHandler.ListDeployments)
			ui.POST("/deployments/:id/redeploy", deployHandler.Redeploy)
		}
	}
}
