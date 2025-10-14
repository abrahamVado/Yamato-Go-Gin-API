package db

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// BuildPostgresDSNFromEnv assembles a PostgreSQL DSN using environment variables.
func BuildPostgresDSNFromEnv() (string, error) {
	// 1.- Prefer a fully specified DATABASE_URL when provided.
	if raw := strings.TrimSpace(os.Getenv("DATABASE_URL")); raw != "" {
		return raw, nil
	}

	// 2.- Gather individual connection components using sensible defaults.
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "postgres")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbName := getEnv("POSTGRES_DB", "postgres")
	sslMode := getEnv("POSTGRES_SSLMODE", "disable")

	// 3.- Ensure the provided port is numeric to avoid invalid DSNs.
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid POSTGRES_PORT value %q: %w", port, err)
	}

	// 4.- Construct the DSN using the net/url package to handle escaping.
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

	// 5.- Return the composed DSN so callers can open a database connection.
	return u.String(), nil
}

// getEnv reads an environment variable with fallback to a default value.
func getEnv(key, def string) string {
	// 1.- Read the environment variable and trim surrounding whitespace.
	if value, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(value)
	}

	// 2.- Return the provided default when the environment variable is not set.
	return def
}
