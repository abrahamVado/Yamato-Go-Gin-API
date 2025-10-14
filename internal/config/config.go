package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

//1.- Config gathers every individual configuration section for the application.
type Config struct {
	JWT      JWTConfig
	Redis    RedisConfig
	Postgres PostgresConfig
	Rate     RateLimitConfig
	Locale   LocaleConfig
	CORS     CORSConfig
	Storage  StorageConfig
}

//1.- JWTConfig stores token-related configuration.
type JWTConfig struct {
	Secret     string
	Issuer     string
	Audience   string
	Expiration time.Duration
}

//1.- RedisConfig stores cache connection parameters.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	TLS      bool
}

//1.- PostgresConfig keeps the SQL database connection details.
type PostgresConfig struct {
	Host           string
	Port           int
	User           string
	Password       string
	DBName         string
	SSLMode        string
	MaxConnections int
	MinConnections int
	ConnTimeout    time.Duration
}

//1.- RateLimitConfig represents throttling configuration.
type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Duration time.Duration
	Burst    int
}

//1.- LocaleConfig defines locale and timezone defaults.
type LocaleConfig struct {
	Default   string
	Supported []string
	TimeZone  string
}

//1.- CORSConfig defines cross-origin request options.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

//1.- StorageConfig supports local or object storage options.
type StorageConfig struct {
	Provider    string
	LocalPath   string
	S3Endpoint  string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
}

//1.- Load reads configuration from a .env file and environment variables.
func Load(path string) (Config, error) {
        //1.- Read the .env file (if present) without mutating the global environment.
	envMap, err := readEnvFile(path)
	if err != nil {
		return Config{}, err
	}

        //1.- Assemble the final configuration struct by querying helpers.
	cfg := Config{
		JWT: JWTConfig{
			Secret:     getString("JWT_SECRET", envMap, ""),
			Issuer:     getString("JWT_ISSUER", envMap, ""),
			Audience:   getString("JWT_AUDIENCE", envMap, ""),
			Expiration: getDuration("JWT_EXPIRATION", envMap, time.Hour*24),
		},
		Redis: RedisConfig{
			Host:     getString("REDIS_HOST", envMap, "127.0.0.1"),
			Port:     getInt("REDIS_PORT", envMap, 6379),
			Password: getString("REDIS_PASSWORD", envMap, ""),
			DB:       getInt("REDIS_DB", envMap, 0),
			TLS:      getBool("REDIS_TLS", envMap, false),
		},
		Postgres: PostgresConfig{
			Host:           getString("POSTGRES_HOST", envMap, "127.0.0.1"),
			Port:           getInt("POSTGRES_PORT", envMap, 5432),
			User:           getString("POSTGRES_USER", envMap, "postgres"),
			Password:       getString("POSTGRES_PASSWORD", envMap, ""),
			DBName:         getString("POSTGRES_DB", envMap, "postgres"),
			SSLMode:        getString("POSTGRES_SSLMODE", envMap, "disable"),
			MaxConnections: getInt("POSTGRES_MAX_CONNECTIONS", envMap, 10),
			MinConnections: getInt("POSTGRES_MIN_CONNECTIONS", envMap, 1),
			ConnTimeout:    getDuration("POSTGRES_CONN_TIMEOUT", envMap, 5*time.Second),
		},
		Rate: RateLimitConfig{
			Enabled:  getBool("RATE_LIMIT_ENABLED", envMap, true),
			Requests: getInt("RATE_LIMIT_REQUESTS", envMap, 100),
			Duration: getDuration("RATE_LIMIT_DURATION", envMap, time.Minute),
			Burst:    getInt("RATE_LIMIT_BURST", envMap, 20),
		},
		Locale: LocaleConfig{
			Default:   getString("LOCALE_DEFAULT", envMap, "en"),
			Supported: getStringSlice("LOCALE_SUPPORTED", envMap, []string{"en"}),
			TimeZone:  getString("LOCALE_TIMEZONE", envMap, "UTC"),
		},
		CORS: CORSConfig{
			AllowOrigins:     getStringSlice("CORS_ALLOW_ORIGINS", envMap, []string{"*"}),
			AllowMethods:     getStringSlice("CORS_ALLOW_METHODS", envMap, []string{"GET", "POST", "PUT", "PATCH", "DELETE"}),
			AllowHeaders:     getStringSlice("CORS_ALLOW_HEADERS", envMap, []string{"Authorization", "Content-Type"}),
			ExposeHeaders:    getStringSlice("CORS_EXPOSE_HEADERS", envMap, []string{}),
			AllowCredentials: getBool("CORS_ALLOW_CREDENTIALS", envMap, false),
			MaxAge:           getDuration("CORS_MAX_AGE", envMap, 10*time.Minute),
		},
		Storage: StorageConfig{
			Provider:    getString("STORAGE_PROVIDER", envMap, "local"),
			LocalPath:   getString("STORAGE_LOCAL_PATH", envMap, "./storage"),
			S3Endpoint:  getString("STORAGE_S3_ENDPOINT", envMap, ""),
			S3Bucket:    getString("STORAGE_S3_BUCKET", envMap, ""),
			S3Region:    getString("STORAGE_S3_REGION", envMap, ""),
			S3AccessKey: getString("STORAGE_S3_ACCESS_KEY", envMap, ""),
			S3SecretKey: getString("STORAGE_S3_SECRET_KEY", envMap, ""),
			S3UseSSL:    getBool("STORAGE_S3_USE_SSL", envMap, true),
		},
	}

        //1.- Perform sanity checks for required fields that lack sensible defaults.
	if cfg.JWT.Secret == "" {
		return Config{}, errors.New("jwt secret must not be empty")
	}

	return cfg, nil
}

//1.- readEnvFile parses a .env file if it exists and returns the key-value map.
func readEnvFile(path string) (map[string]string, error) {
        //1.- Determine the target path, falling back to the default .env.
	target := path
	if strings.TrimSpace(target) == "" {
		target = ".env"
	}

        //1.- Attempt to read the .env file; missing files are acceptable.
	envMap, err := godotenv.Read(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, nil
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) && pathErr.Err == os.ErrNotExist {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	return envMap, nil
}

//1.- getString retrieves a string configuration value with fallback logic.
func getString(key string, envMap map[string]string, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	if val, ok := envMap[key]; ok {
		return val
	}
	return def
}

//1.- getInt retrieves an integer configuration value with fallback logic.
func getInt(key string, envMap map[string]string, def int) int {
	raw := getString(key, envMap, "")
	if raw == "" {
		return def
	}
	val, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return val
}

//1.- getBool retrieves a boolean configuration value with fallback logic.
func getBool(key string, envMap map[string]string, def bool) bool {
	raw := getString(key, envMap, "")
	if raw == "" {
		return def
	}
	val, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return val
}

//1.- getDuration retrieves a duration value using time.ParseDuration.
func getDuration(key string, envMap map[string]string, def time.Duration) time.Duration {
	raw := getString(key, envMap, "")
	if raw == "" {
		return def
	}
	val, err := time.ParseDuration(raw)
	if err != nil {
		return def
	}
	return val
}

//1.- getStringSlice parses comma-separated lists into trimmed slices.
func getStringSlice(key string, envMap map[string]string, def []string) []string {
	raw := getString(key, envMap, "")
	if raw == "" {
		return append([]string(nil), def...)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	if len(parts) == 0 {
		return []string{}
	}
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		piece := strings.TrimSpace(part)
		if piece != "" {
			trimmed = append(trimmed, piece)
		}
	}
	return trimmed
}
