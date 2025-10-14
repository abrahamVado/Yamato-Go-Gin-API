package testutil

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
	"github.com/redis/go-redis/v9"
)

// PostgresContainer groups connection metadata for a transient PostgreSQL instance.
type PostgresContainer struct {
	DSN      string
	Host     string
	Port     string
	User     string
	Password string
	Database string

	pool     *dockertest.Pool
	resource *dockertest.Resource
}

// RedisContainer groups connection metadata for a transient Redis instance.
type RedisContainer struct {
	Addr string
	DB   int

	pool     *dockertest.Pool
	resource *dockertest.Resource
}

// RunPostgresContainer provisions a dockerized PostgreSQL instance for integration tests.
func RunPostgresContainer(t *testing.T) *PostgresContainer {
	t.Helper()

	//1.- Confirm Docker is reachable before attempting to start a container.
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("docker socket unavailable for postgres container")
			return nil
		}
		t.Fatalf("failed to inspect docker socket: %v", err)
		return nil
	}

	//2.- Open a connection to the local Docker engine.
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Skipf("docker unavailable for postgres container: %v", err)
		return nil
	}
	pool.MaxWait = 15 * time.Second

	//3.- Launch a postgres:16 container with deterministic credentials for tests.
	runOpts := &dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=test",
		},
	}

	resource, err := pool.RunWithOptions(runOpts, func(config *docker.HostConfig) {
		//4.- Enable automatic cleanup in case of abrupt test termination.
		config.AutoRemove = true
	})
	if err != nil {
		t.Skipf("postgres container start failed: %v", err)
		return nil
	}

	//5.- Ensure the container is purged after the test finishes.
	t.Cleanup(func() {
		_ = pool.Purge(resource)
	})

	port := resource.GetPort("5432/tcp")
	container := &PostgresContainer{
		DSN:      fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", "postgres", "postgres", port, "test"),
		Host:     "localhost",
		Port:     port,
		User:     "postgres",
		Password: "postgres",
		Database: "test",
		pool:     pool,
		resource: resource,
	}

	//6.- Wait until PostgreSQL accepts connections using exponential backoff.
	err = pool.Retry(func() error {
		db, openErr := sql.Open("postgres", container.DSN)
		if openErr != nil {
			return openErr
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		return db.PingContext(ctx)
	})
	if err != nil {
		t.Skipf("postgres container ping failed: %v", err)
		return nil
	}

	return container
}

// RunRedisContainer provisions a dockerized Redis instance for integration tests.
func RunRedisContainer(t *testing.T) *RedisContainer {
	t.Helper()

	//1.- Confirm Docker is reachable before attempting to start a container.
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("docker socket unavailable for redis container")
			return nil
		}
		t.Fatalf("failed to inspect docker socket: %v", err)
		return nil
	}

	//2.- Open a connection to the local Docker engine.
	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Skipf("docker unavailable for redis container: %v", err)
		return nil
	}
	pool.MaxWait = 15 * time.Second

	//3.- Launch a redis:7 container suitable for integration testing.
	runOpts := &dockertest.RunOptions{
		Repository: "redis",
		Tag:        "7-alpine",
	}

	resource, err := pool.RunWithOptions(runOpts, func(config *docker.HostConfig) {
		//4.- Ensure the container is automatically removed after stopping.
		config.AutoRemove = true
	})
	if err != nil {
		t.Skipf("redis container start failed: %v", err)
		return nil
	}

	//5.- Guarantee the container is purged once tests conclude.
	t.Cleanup(func() {
		_ = pool.Purge(resource)
	})

	port := resource.GetPort("6379/tcp")
	container := &RedisContainer{
		Addr:     fmt.Sprintf("localhost:%s", port),
		DB:       0,
		pool:     pool,
		resource: resource,
	}

	//6.- Wait until Redis accepts commands via go-redis client.
	err = pool.Retry(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		client := redis.NewClient(&redis.Options{Addr: container.Addr, DB: container.DB})
		defer client.Close()

		return client.Ping(ctx).Err()
	})
	if err != nil {
		t.Skipf("redis container ping failed: %v", err)
		return nil
	}

	return container
}
