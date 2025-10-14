package testutil_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"github.com/example/Yamato-Go-Gin-API/internal/testutil"
)

func TestRunPostgresContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres container test in short mode")
	}

	//1.- Boot the PostgreSQL container via the helper.
	pg := testutil.RunPostgresContainer(t)
	if pg == nil {
		t.Skip("postgres container helper unavailable")
	}

	//2.- Connect to the database and ensure basic queries succeed.
	db, err := sql.Open("postgres", pg.DSN)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("postgres ping: %v", err)
	}
}

func TestRunRedisContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping redis container test in short mode")
	}

	//1.- Boot the Redis container via the helper.
	redisContainer := testutil.RunRedisContainer(t)
	if redisContainer == nil {
		t.Skip("redis container helper unavailable")
	}

	//2.- Interact with Redis and confirm ping succeeds.
	client := redis.NewClient(&redis.Options{Addr: redisContainer.Addr, DB: redisContainer.DB})
	t.Cleanup(func() {
		_ = client.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}
}
