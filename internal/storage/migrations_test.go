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
                "join_requests",
                "tasks",
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

	//6.- Confirm the join request status enum is present with the expected values.
	rows, err := db.QueryContext(ctx, "SELECT enumlabel FROM pg_enum WHERE enumtypid = 'join_request_status'::regtype ORDER BY enumsortorder")
	if err != nil {
		t.Fatalf("failed to query join_request_status enum: %v", err)
	}
	defer rows.Close()

	var labels []string
	for rows.Next() {
		var label string
		if scanErr := rows.Scan(&label); scanErr != nil {
			t.Fatalf("failed to scan enum label: %v", scanErr)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("failed to iterate enum labels: %v", err)
	}

	expectedLabels := []string{"pending", "approved", "declined"}
	if len(labels) != len(expectedLabels) {
		t.Fatalf("unexpected enum label count: got %d want %d", len(labels), len(expectedLabels))
	}
	for i, label := range labels {
		if label != expectedLabels[i] {
			t.Fatalf("unexpected enum label at position %d: got %s want %s", i, label, expectedLabels[i])
		}
	}

	//7.- Verify the join request status column uses the enum type and defaults to pending.
	var statusDataType, statusUDTName, statusDefault string
	statusQuery := "SELECT data_type, udt_name, column_default FROM information_schema.columns WHERE table_name = 'join_requests' AND column_name = 'status'"
	if err := db.QueryRowContext(ctx, statusQuery).Scan(&statusDataType, &statusUDTName, &statusDefault); err != nil {
		t.Fatalf("failed to describe join_requests.status: %v", err)
	}
	if statusDataType != "USER-DEFINED" {
		t.Fatalf("unexpected status data type: %s", statusDataType)
	}
	if statusUDTName != "join_request_status" {
		t.Fatalf("unexpected status udt: %s", statusUDTName)
	}
	if statusDefault != "'pending'::join_request_status" {
		t.Fatalf("unexpected status default: %s", statusDefault)
	}

	//8.- Ensure the payload column stores JSONB with a deterministic default value.
	var payloadDataType, payloadDefault string
	payloadQuery := "SELECT data_type, column_default FROM information_schema.columns WHERE table_name = 'join_requests' AND column_name = 'payload'"
	if err := db.QueryRowContext(ctx, payloadQuery).Scan(&payloadDataType, &payloadDefault); err != nil {
		t.Fatalf("failed to describe join_requests.payload: %v", err)
	}
	if payloadDataType != "jsonb" {
		t.Fatalf("unexpected payload data type: %s", payloadDataType)
	}
	if payloadDefault != "'{}'::jsonb" {
		t.Fatalf("unexpected payload default: %s", payloadDefault)
	}

	//9.- Confirm timestamps default to UTC values so records always contain audit metadata.
	var createdType, createdDefault string
	createdQuery := "SELECT data_type, column_default FROM information_schema.columns WHERE table_name = 'join_requests' AND column_name = 'created_at'"
	if err := db.QueryRowContext(ctx, createdQuery).Scan(&createdType, &createdDefault); err != nil {
		t.Fatalf("failed to describe join_requests.created_at: %v", err)
	}
	if createdType != "timestamp with time zone" {
		t.Fatalf("unexpected created_at type: %s", createdType)
	}
	if createdDefault == "" {
		t.Fatalf("expected created_at default to be set")
	}

	var updatedType, updatedDefault string
	updatedQuery := "SELECT data_type, column_default FROM information_schema.columns WHERE table_name = 'join_requests' AND column_name = 'updated_at'"
	if err := db.QueryRowContext(ctx, updatedQuery).Scan(&updatedType, &updatedDefault); err != nil {
		t.Fatalf("failed to describe join_requests.updated_at: %v", err)
	}
	if updatedType != "timestamp with time zone" {
		t.Fatalf("unexpected updated_at type: %s", updatedType)
	}
	if updatedDefault == "" {
		t.Fatalf("expected updated_at default to be set")
	}

	//10.- Validate that duplicate requests per team and requester are rejected by a unique constraint.
	var constraintName sql.NullString
	constraintQuery := "SELECT conname FROM pg_constraint WHERE conrelid = 'join_requests'::regclass AND contype = 'u' AND conname = 'join_requests_unique_requester'"
	if err := db.QueryRowContext(ctx, constraintQuery).Scan(&constraintName); err != nil {
		t.Fatalf("failed to inspect join_requests unique constraint: %v", err)
	}
	if !constraintName.Valid {
		t.Fatalf("expected join_requests_unique_requester constraint to exist")
	}
}
