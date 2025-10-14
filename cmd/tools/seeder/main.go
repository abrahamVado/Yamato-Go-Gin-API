package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/example/Yamato-Go-Gin-API/seeds"
)

func main() {
	//1.- Build the database connection string from environment variables.
	dsn, err := buildPostgresDSNFromEnv()
	if err != nil {
		log.Fatalf("failed to build postgres dsn: %v", err)
	}

	//2.- Open a database connection using the pq driver.
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		//3.- Close the connection on exit to release resources.
		_ = db.Close()
	}()

	//4.- Verify the database is reachable before running the seeds.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	//5.- Construct the seeder instance that knows how to populate baseline data.
	seeder, err := seeds.NewSeeder(db)
	if err != nil {
		log.Fatalf("failed to initialize seeder: %v", err)
	}

	//6.- Execute the seeding workflow and surface any error to the operator.
	if err := seeder.Run(ctx); err != nil {
		log.Fatalf("seeding failed: %v", err)
	}

	//7.- Inform the operator that the bootstrap process finished without issues.
	log.Println("Database seed completed successfully")
}

// buildPostgresDSNFromEnv assembles a PostgreSQL DSN using environment variables.
func buildPostgresDSNFromEnv() (string, error) {
	//1.- Prefer a fully specified DATABASE_URL when provided.
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return raw, nil
	}

	//2.- Gather individual connection components using sensible defaults.
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "postgres")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbName := getEnv("POSTGRES_DB", "postgres")
	sslMode := getEnv("POSTGRES_SSLMODE", "disable")

	//3.- Ensure the provided port is numeric to avoid invalid DSNs.
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid POSTGRES_PORT value %q: %w", port, err)
	}

	//4.- Construct the DSN using the net/url package to handle escaping.
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + dbName,
	}
	if password != "" {
		u.User = url.UserPassword(user, password)
	} else {
		u.User = url.User(user)
	}
	query := url.Values{}
	query.Set("sslmode", sslMode)
	u.RawQuery = query.Encode()

	//5.- Return the composed DSN so callers can open a database connection.
	return u.String(), nil
}

// getEnv reads an environment variable with fallback to a default value.
func getEnv(key, def string) string {
	//1.- Read the environment variable and trim surrounding whitespace.
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}

	//2.- Return the provided default when the environment variable is not set.
	return def
}
