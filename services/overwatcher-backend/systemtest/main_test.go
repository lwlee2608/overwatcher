package systemtest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	internalhttp "github.com/lwlee2608/overwatcher/internal/api/http"
	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/project"
	"github.com/lwlee2608/overwatcher/internal/service/user"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
	"github.com/lwlee2608/overwatcher/systemtest/postgres"
	"github.com/lwlee2608/overwatcher/systemtest/tests"
	"github.com/stretchr/testify/require"
)

func TestSystemIntegration(t *testing.T) {
	dbUser := "lumora"
	dbPassword := "lumora"
	dbName := "lumora"
	dbHost := "localhost"
	schema := "public"

	container, err := postgres.StartPostgres(context.Background(), dbUser, dbPassword, dbName)
	require.NoError(t, err)
	defer func() {
		_ = postgres.TerminatePostgres(context.Background(), container)
	}()

	port, err := container.MappedPort(context.Background(), "5432/tcp")
	require.NoError(t, err)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, port.Port(), dbName)

	err = db.RunMigrations(dbURL, schema)
	require.NoError(t, err)

	pool, err := db.InitDB(context.Background(), db.Config{URL: dbURL, Schema: schema})
	require.NoError(t, err)
	defer pool.Close()

	queries := sqlc.New(pool)

	agentSvc := agent.NewService(queries, 60*time.Second)
	eventLogSvc := eventlog.NewService(queries)
	userSvc := user.NewService(pool)
	projectSvc := project.NewService(pool)
	intentStore := intent.NewDBStore(pool)
	dispatchSvc := dispatch.NewForTest(intentStore)
	webhookSvc := webhook.New(nil, projectSvc, intentStore, eventLogSvc)

	services := &internalhttp.Services{
		WebhookService:    webhookSvc,
		DispatchService:   dispatchSvc,
		AgentService:      agentSvc,
		EventLogService:   eventLogSvc,
		UserService:       userSvc,
		ProjectService:    projectSvc,
		WebhookSecret:     "test-webhook-secret",
		AgentSharedSecret: "test-agent-secret",
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	internalhttp.SetupRoute(engine, services)

	t.Run("HealthCheck", func(t *testing.T) { tests.TestHealthCheck(t, engine) })
	t.Run("Agents", func(t *testing.T) { tests.TestAgents(t, engine, agentSvc) })
	var userID string
	t.Run("Users", func(t *testing.T) { userID = tests.TestUsers(t, engine) })
	t.Run("Projects", func(t *testing.T) { tests.TestProjects(t, engine, userID) })
	t.Run("Deploy", func(t *testing.T) { tests.TestDeploy(t, engine, intentStore, services.AgentSharedSecret) })
}
