package config

import (
	"encoding/json"
	"os"
)

// CronEntry represents a scheduled job definition sourced from configuration.
type CronEntry struct {
	Name    string         `json:"name"`
	Spec    string         `json:"spec"`
	Job     string         `json:"job"`
	Payload map[string]any `json:"payload"`
}

// JobsConfig wraps the different job-related configuration facets.
type JobsConfig struct {
	CronEntries []CronEntry `json:"cron_entries"`
}

// LoadJobsConfig builds the jobs configuration from environment variables.
func LoadJobsConfig() JobsConfig {
	// 1.- Start with an empty configuration so we never return nil slices.
	cfg := JobsConfig{CronEntries: []CronEntry{}}
	// 2.- Read the JSON blob describing cron entries when present.
	raw := os.Getenv("JOB_CRON_ENTRIES")
	if raw == "" {
		return cfg
	}
	// 3.- Decode the JSON payload into the struct, ignoring errors silently is dangerous.
	var parsed JobsConfig
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return cfg
	}
	// 4.- Return the parsed configuration when decoding succeeded.
	return parsed
}
