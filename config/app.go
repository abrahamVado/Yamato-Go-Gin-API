package config

import "os"

// AppConfig holds basic application configuration.
type AppConfig struct {
	Name string
	Port string
}

// LoadAppConfig reads environment variables and returns application config.
func LoadAppConfig() AppConfig {
	// 1.- Provide sensible defaults for configuration values.
	config := AppConfig{
		Name: "Larago API",
		Port: "8080",
	}

	// 2.- Override defaults with environment variables when present.
	if value := os.Getenv("APP_NAME"); value != "" {
		config.Name = value
	}

	// 3.- Override listening port from environment variables when set.
	if value := os.Getenv("APP_PORT"); value != "" {
		config.Port = value
	}

	// 4.- Return the fully populated configuration instance.
	return config
}
