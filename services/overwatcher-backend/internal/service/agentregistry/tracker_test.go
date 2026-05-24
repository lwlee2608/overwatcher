package agentregistry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
	"github.com/lwlee2608/overwatcher/systemtest/postgres"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*db.Config, func()) {
	t.Helper()
	ctx := context.Background()
	dbUser := "lumora"
	dbPassword := "lumora"
	dbName := "lumora"
	dbHost := "localhost"
	schema := "public"

	container, err := postgres.StartPostgres(ctx, dbUser, dbPassword, dbName)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, port.Port(), dbName)
	err = db.RunMigrations(dbURL, schema)
	require.NoError(t, err)

	cfg := &db.Config{URL: dbURL, Schema: schema}
	cleanup := func() {
		_ = postgres.TerminatePostgres(ctx, container)
	}
	return cfg, cleanup
}

func TestBindProject_OneToOneConstraint(t *testing.T) {
	cfg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pool, err := db.InitDB(ctx, *cfg)
	require.NoError(t, err)
	defer pool.Close()

	queries := sqlc.New(pool)
	svc := NewService(pool, queries, 60*time.Second)

	// Seed two agents.
	require.NoError(t, svc.Record(ctx, "agent-a", "10.0.0.1", ""))
	require.NoError(t, svc.Record(ctx, "agent-b", "10.0.0.2", ""))

	agents, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 2)
	byName := map[string]string{}
	for _, a := range agents {
		byName[a.Name] = a.ID
	}
	agentA := byName["agent-a"]
	agentB := byName["agent-b"]
	require.NotEmpty(t, agentA)
	require.NotEmpty(t, agentB)

	// Create a project to satisfy the FK constraint.
	user, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "test@example.com",
		Name:  "Test User",
	})
	require.NoError(t, err)
	project, err := queries.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:       user.ID,
		Name:         "test-project",
		Description:  "",
		ComposeFile:  "docker-compose.yml",
		Environment:  "production",
	})
	require.NoError(t, err)
	projectID := util.UUIDToString(project.ID)

	// Bind agent A to project.
	statusA, err := svc.BindProject(ctx, agentA, projectID)
	require.NoError(t, err)
	require.Equal(t, projectID, statusA.ProjectID)

	// Bind agent B to the same project — should evict agent A.
	statusB, err := svc.BindProject(ctx, agentB, projectID)
	require.NoError(t, err)
	require.Equal(t, projectID, statusB.ProjectID)

	// Verify agent A is now unbound.
	agentAStatus, err := svc.GetByID(ctx, agentA)
	require.NoError(t, err)
	require.Empty(t, agentAStatus.ProjectID)

	// Verify agent B is bound.
	agentBStatus, err := svc.GetByID(ctx, agentB)
	require.NoError(t, err)
	require.Equal(t, projectID, agentBStatus.ProjectID)
}

func TestBindProject_Unbind(t *testing.T) {
	cfg, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	pool, err := db.InitDB(ctx, *cfg)
	require.NoError(t, err)
	defer pool.Close()

	queries := sqlc.New(pool)
	svc := NewService(pool, queries, 60*time.Second)

	require.NoError(t, svc.Record(ctx, "agent-a", "10.0.0.1", ""))

	agents, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	agentID := agents[0].ID

	user, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email: "test@example.com",
		Name:  "Test User",
	})
	require.NoError(t, err)
	project, err := queries.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:       user.ID,
		Name:         "test-project",
		Description:  "",
		ComposeFile:  "docker-compose.yml",
		Environment:  "production",
	})
	require.NoError(t, err)
	projectID := util.UUIDToString(project.ID)

	// Bind.
	_, err = svc.BindProject(ctx, agentID, projectID)
	require.NoError(t, err)

	// Unbind.
	status, err := svc.BindProject(ctx, agentID, "")
	require.NoError(t, err)
	require.Empty(t, status.ProjectID)

	// Verify.
	agentStatus, err := svc.GetByID(ctx, agentID)
	require.NoError(t, err)
	require.Empty(t, agentStatus.ProjectID)
}
