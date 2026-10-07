// story: e01s01

// Package config reads the app configuration from environment variables once at startup.
package config

import "fmt"

// Config is the validated app configuration.
type Config struct {
	DBPath string // SQLite file path (DB_PATH)
}

// Load reads configuration through getenv (os.Getenv in production) and fails
// fast when a required variable is missing.
//
//	cfg, err := config.Load(os.Getenv)
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{DBPath: getenv("DB_PATH")}
	if cfg.DBPath == "" {
		return Config{}, fmt.Errorf("config: required env var DB_PATH is not set; set it to the SQLite file path, e.g. DB_PATH=/tmp/upareal.db")
	}
	return cfg, nil
}
