package intent

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/db"
	"github.com/lwlee2608/overwatcher/systemtest/postgres"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	user, pass, name := "intent", "intent", "intent"

	container, err := postgres.StartPostgres(ctx, user, pass, name)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}

	teardown := func() { _ = postgres.TerminatePostgres(ctx, container) }

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		teardown()
		log.Fatalf("mapped port: %v", err)
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", user, pass, port.Port(), name)

	if err := db.RunMigrations(dbURL, "public"); err != nil {
		teardown()
		log.Fatalf("migrate: %v", err)
	}

	pool, err := db.InitDB(ctx, db.Config{URL: dbURL, Schema: "public"})
	if err != nil {
		teardown()
		log.Fatalf("init db: %v", err)
	}
	testPool = pool

	code := m.Run()

	pool.Close()
	teardown()
	os.Exit(code)
}
