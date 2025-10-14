package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/example/Yamato-Go-Gin-API/internal/storage"
	"github.com/example/Yamato-Go-Gin-API/internal/tooling/db"
)

func main() {
	// 1.- Declare command-line flags that control the migration behaviour.
	direction := flag.String("direction", "up", "migration direction: up or down")
	timeout := flag.Duration("timeout", time.Minute, "maximum time to wait for database operations")
	flag.Parse()

	// 2.- Build the PostgreSQL connection string using shared helpers.
	dsn, err := db.BuildPostgresDSNFromEnv()
	if err != nil {
		log.Fatalf("failed to build postgres dsn: %v", err)
	}

	// 3.- Open the database connection and ensure resources are released on exit.
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("failed to close database connection: %v", closeErr)
		}
	}()

	// 4.- Verify the database is reachable before attempting to run migrations.
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	// 5.- Construct the migrator which applies the embedded SQL bundles.
	migrator, err := storage.NewMigrator(conn)
	if err != nil {
		log.Fatalf("failed to create migrator: %v", err)
	}

	// 6.- Dispatch based on the requested direction.
	switch strings.ToLower(strings.TrimSpace(*direction)) {
	case "", "up":
		if err := migrator.Apply(ctx); err != nil {
			log.Fatalf("failed to apply migrations: %v", err)
		}
		log.Println("database migrations applied successfully")
	case "down":
		log.Println("down migrations are not defined; skipping without changes")
	default:
		log.Fatalf("unknown migration direction %q", *direction)
	}

	// 7.- Exit explicitly so the deferred cleanup runs before the process stops.
	os.Exit(0)
}
