package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

func TestMigratorApply(t *testing.T) {
	//1.- Ensure Docker is reachable; skip the integration test when the engine is not available.
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("docker socket not available; skipping migration integration test")
		}
		t.Fatalf("failed to stat docker socket: %v", err)
	}

	//2.- Boot a temporary PostgreSQL container so we can execute the migrations end-to-end.
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("failed to connect to Docker: %v", err)
	}

	runOptions := &dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_DB=testdb",
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"TZ=UTC",
		},
	}

	resource, err := pool.RunWithOptions(runOptions, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if cleanupErr := pool.Purge(resource); cleanupErr != nil {
			t.Fatalf("failed to purge resource: %v", cleanupErr)
		}
	})

	//3.- Build the database connection string and wait until PostgreSQL is ready to accept traffic.
	dbURL := fmt.Sprintf("postgres://postgres:postgres@localhost:%s/testdb?sslmode=disable", resource.GetPort("5432/tcp"))

	var db *sql.DB
	if retryErr := pool.Retry(func() error {
		var openErr error
		db, openErr = sql.Open("postgres", dbURL)
		if openErr != nil {
			return openErr
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.PingContext(ctx)
	}); retryErr != nil {
		t.Fatalf("failed to connect to postgres: %v", retryErr)
	}
	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
		}
	})

	//4.- Execute the migrations and assert that no errors occur.
	migrator, err := NewMigrator(db)
	if err != nil {
		t.Fatalf("failed to build migrator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := migrator.Apply(ctx); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	//5.- Validate that the most critical tables exist by querying PostgreSQL metadata.
	requiredTables := []string{
		"users",
		"roles",
		"permissions",
		"role_permissions",
		"user_roles",
		"teams",
		"team_members",
		"modules",
		"settings",
		"notifications",
	}

	for _, table := range requiredTables {
		var result sql.NullString
		query := fmt.Sprintf("SELECT to_regclass('public.%s')", table)
		if err := db.QueryRowContext(ctx, query).Scan(&result); err != nil {
			t.Fatalf("failed to query table %s: %v", table, err)
		}
		if !result.Valid {
			t.Fatalf("expected table %s to exist", table)
		}
	}
}
