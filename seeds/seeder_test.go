package seeds_test

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

	"github.com/example/Yamato-Go-Gin-API/internal/storage"
	"github.com/example/Yamato-Go-Gin-API/seeds"
)

func TestSeederRunIsIdempotent(t *testing.T) {
	//1.- Ensure Docker is available before attempting to run the integration test.
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("docker socket not available; skipping seeder integration test")
		}
		t.Fatalf("failed to stat docker socket: %v", err)
	}

	//2.- Start a disposable PostgreSQL container for exercising the seeds end-to-end.
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
		//3.- Configure the container lifecycle to auto remove after the test completes.
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		//4.- Ensure the container is purged so tests do not leak resources.
		if cleanupErr := pool.Purge(resource); cleanupErr != nil {
			t.Fatalf("failed to purge resource: %v", cleanupErr)
		}
	})

	//5.- Build the database URL and wait for PostgreSQL to accept connections.
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
		//6.- Close the database connection once the test is finished.
		if db != nil {
			_ = db.Close()
		}
	})

	//7.- Apply the SQL migrations so the schema exists before seeding.
	migrator, err := storage.NewMigrator(db)
	if err != nil {
		t.Fatalf("failed to build migrator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := migrator.Apply(ctx); err != nil {
		t.Fatalf("failed to apply migrations: %v", err)
	}

	//8.- Construct the seeder and execute it twice to verify idempotency.
	seeder, err := seeds.NewSeeder(db)
	if err != nil {
		t.Fatalf("failed to build seeder: %v", err)
	}

	if err := seeder.Run(ctx); err != nil {
		t.Fatalf("failed to run seeder: %v", err)
	}

	//9.- Capture baseline counts and key identifiers after the first execution.
	adminRoleID := queryInt64(t, ctx, db, "SELECT id FROM roles WHERE name = $1", "admin")
	adminUser := queryAdminUser(t, ctx, db)
	permissionsCount := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM permissions")
	rolePermissionsCount := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM role_permissions WHERE role_id = $1", adminRoleID)
	settingsCount := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM settings")
	userRoleCount := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM user_roles WHERE user_id = $1", adminUser.id)

	if permissionsCount == 0 {
		t.Fatalf("expected permissions to be seeded")
	}
	if settingsCount == 0 {
		t.Fatalf("expected settings to be seeded")
	}
	if adminUser.firstName != "System" || adminUser.lastName != "Administrator" {
		t.Fatalf("unexpected admin user name: %+v", adminUser)
	}
	if adminUser.status != "active" {
		t.Fatalf("unexpected admin user status: %s", adminUser.status)
	}
	if adminUser.passwordHash == "" {
		t.Fatalf("expected admin user password hash to be set")
	}
	if rolePermissionsCount != permissionsCount {
		t.Fatalf("expected role permissions to match permissions count: got %d want %d", rolePermissionsCount, permissionsCount)
	}
	if userRoleCount != 1 {
		t.Fatalf("expected exactly one admin user role assignment, got %d", userRoleCount)
	}

	//10.- Run the seeder a second time and ensure counts remain stable.
	if err := seeder.Run(ctx); err != nil {
		t.Fatalf("failed to rerun seeder: %v", err)
	}

	permissionsCountAgain := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM permissions")
	rolePermissionsCountAgain := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM role_permissions WHERE role_id = $1", adminRoleID)
	settingsCountAgain := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM settings")
	userRoleCountAgain := queryInt64(t, ctx, db, "SELECT COUNT(*) FROM user_roles WHERE user_id = $1", adminUser.id)

	if permissionsCountAgain != permissionsCount {
		t.Fatalf("permissions count changed after rerun: got %d want %d", permissionsCountAgain, permissionsCount)
	}
	if rolePermissionsCountAgain != rolePermissionsCount {
		t.Fatalf("role_permissions count changed after rerun: got %d want %d", rolePermissionsCountAgain, rolePermissionsCount)
	}
	if settingsCountAgain != settingsCount {
		t.Fatalf("settings count changed after rerun: got %d want %d", settingsCountAgain, settingsCount)
	}
	if userRoleCountAgain != userRoleCount {
		t.Fatalf("user_roles count changed after rerun: got %d want %d", userRoleCountAgain, userRoleCount)
	}

	//11.- Re-query the administrator user to ensure key fields remain intact.
	adminUserRepeat := queryAdminUser(t, ctx, db)
	if adminUserRepeat.id != adminUser.id {
		t.Fatalf("admin user ID changed between runs: got %d want %d", adminUserRepeat.id, adminUser.id)
	}
	if adminUserRepeat.firstName != adminUser.firstName || adminUserRepeat.lastName != adminUser.lastName {
		t.Fatalf("admin user name changed between runs: %#v vs %#v", adminUserRepeat, adminUser)
	}
	if adminUserRepeat.status != adminUser.status {
		t.Fatalf("admin user status changed between runs: got %s want %s", adminUserRepeat.status, adminUser.status)
	}
	if adminUserRepeat.passwordHash == "" {
		t.Fatalf("admin user password hash missing after rerun")
	}
}

type adminUserRecord struct {
	id           int64
	firstName    string
	lastName     string
	status       string
	passwordHash string
}

// queryAdminUser retrieves the administrator user details from the database.
func queryAdminUser(t *testing.T, ctx context.Context, db *sql.DB) adminUserRecord {
	//1.- Query the administrator account using its canonical email address.
	const email = "admin@example.com"

	var record adminUserRecord
	err := db.QueryRowContext(ctx, "SELECT id, first_name, last_name, status, password_hash FROM users WHERE email = $1", email).
		Scan(&record.id, &record.firstName, &record.lastName, &record.status, &record.passwordHash)
	if err != nil {
		t.Fatalf("failed to query admin user: %v", err)
	}

	//2.- Return the populated record to callers.
	return record
}

// queryInt64 retrieves a single int64 value from the database.
func queryInt64(t *testing.T, ctx context.Context, db *sql.DB, query string, args ...any) int64 {
	//1.- Execute the scalar query and surface failures to the test harness.
	var value int64
	if err := db.QueryRowContext(ctx, query, args...).Scan(&value); err != nil {
		t.Fatalf("failed to execute query %s: %v", query, err)
	}

	//2.- Return the extracted value to the caller.
	return value
}
