package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

//1.- unsetEnv removes an environment variable for the duration of the test.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, existed := os.LookupEnv(key)
	if existed {
		t.Cleanup(func() { _ = os.Setenv(key, prev) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset %s: %v", key, err)
	}
}

//1.- TestLoadUsesEnvFile ensures .env values populate the configuration.
func TestLoadUsesEnvFile(t *testing.T) {
	unsetEnv(t, "JWT_SECRET")
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	envContent := "" +
		"JWT_SECRET=from-file\n" +
		"JWT_ISSUER=test-service\n" +
		"JWT_EXPIRATION=2h\n" +
		"REDIS_PORT=6380\n" +
		"REDIS_TLS=true\n" +
		"POSTGRES_CONN_TIMEOUT=7s\n" +
		"RATE_LIMIT_DURATION=2m\n" +
		"LOCALE_SUPPORTED=en, ja, pt\n" +
		"CORS_ALLOW_ORIGINS=https://example.com, https://api.example.com\n" +
		"STORAGE_PROVIDER=s3\n" +
		"STORAGE_S3_USE_SSL=false\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("failed to create env file: %v", err)
	}

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.JWT.Secret != "from-file" {
		t.Fatalf("expected jwt secret from file, got %q", cfg.JWT.Secret)
	}
	if cfg.JWT.Expiration != 2*time.Hour {
		t.Fatalf("unexpected jwt expiration: %v", cfg.JWT.Expiration)
	}
	if !cfg.Redis.TLS {
		t.Fatalf("expected redis TLS true")
	}
	if cfg.Redis.Port != 6380 {
		t.Fatalf("expected redis port 6380, got %d", cfg.Redis.Port)
	}
	if cfg.Postgres.ConnTimeout != 7*time.Second {
		t.Fatalf("expected postgres timeout 7s, got %v", cfg.Postgres.ConnTimeout)
	}
	if cfg.Rate.Duration != 2*time.Minute {
		t.Fatalf("expected rate duration 2m, got %v", cfg.Rate.Duration)
	}
	expectedLocales := []string{"en", "ja", "pt"}
	if len(cfg.Locale.Supported) != len(expectedLocales) {
		t.Fatalf("expected %d locales, got %d", len(expectedLocales), len(cfg.Locale.Supported))
	}
	for i, locale := range expectedLocales {
		if cfg.Locale.Supported[i] != locale {
			t.Fatalf("expected locale %q at position %d, got %q", locale, i, cfg.Locale.Supported[i])
		}
	}
	if len(cfg.CORS.AllowOrigins) != 2 {
		t.Fatalf("expected two cors origins, got %d", len(cfg.CORS.AllowOrigins))
	}
	if cfg.CORS.AllowOrigins[0] != "https://example.com" || cfg.CORS.AllowOrigins[1] != "https://api.example.com" {
		t.Fatalf("unexpected cors allow origins: %#v", cfg.CORS.AllowOrigins)
	}
	if cfg.Storage.Provider != "s3" {
		t.Fatalf("expected storage provider s3, got %q", cfg.Storage.Provider)
	}
	if cfg.Storage.S3UseSSL {
		t.Fatalf("expected storage s3 ssl false")
	}
}

//1.- TestLoadEnvironmentOverrides verifies environment variables take priority over .env values.
func TestLoadEnvironmentOverrides(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	envContent := "" +
		"JWT_SECRET=file-secret\n" +
		"RATE_LIMIT_ENABLED=false\n" +
		"REDIS_HOST=redis-file\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("failed to create env file: %v", err)
	}

	t.Setenv("JWT_SECRET", "env-secret")
	t.Setenv("RATE_LIMIT_ENABLED", "true")
	t.Setenv("REDIS_HOST", "redis-env")

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.JWT.Secret != "env-secret" {
		t.Fatalf("expected jwt secret from env, got %q", cfg.JWT.Secret)
	}
	if !cfg.Rate.Enabled {
		t.Fatalf("expected rate limit enabled true")
	}
	if cfg.Redis.Host != "redis-env" {
		t.Fatalf("expected redis host redis-env, got %q", cfg.Redis.Host)
	}
}

//1.- TestLoadMissingEnvFile ensures defaults work when no file is present.
func TestLoadMissingEnvFile(t *testing.T) {
	t.Setenv("JWT_SECRET", "only-env")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.JWT.Secret != "only-env" {
		t.Fatalf("expected jwt secret only-env, got %q", cfg.JWT.Secret)
	}
	if cfg.Redis.Port != 6379 {
		t.Fatalf("expected default redis port 6379, got %d", cfg.Redis.Port)
	}
}

//1.- TestLoadWithoutJWTSecretFails ensures the loader enforces a required JWT secret.
func TestLoadWithoutJWTSecretFails(t *testing.T) {
	//1.- Create an env file without the JWT secret to simulate misconfiguration.
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	envContent := "JWT_ISSUER=missing-secret-service\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	unsetEnv(t, "JWT_SECRET")

	//1.- Attempting to load should return an error because the secret is mandatory.
	if _, err := Load(envPath); err == nil {
		t.Fatal("expected Load to fail when JWT secret is absent")
	}
}
