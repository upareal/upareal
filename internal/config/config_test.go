// story: e01s01
package config_test

import (
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/config"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad_ReadsDBPath(t *testing.T) {
	cfg, err := config.Load(envOf(map[string]string{"DB_PATH": "/data/upareal.db"}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.DBPath != "/data/upareal.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/data/upareal.db")
	}
}

func TestLoad_FailsWhenRequiredVarMissingOrEmpty(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"missing": {},
		"empty":   {"DB_PATH": ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := config.Load(envOf(env))
			if err == nil {
				t.Fatal("Load: expected error, got nil")
			}
			if !strings.Contains(err.Error(), "DB_PATH") {
				t.Errorf("error %q does not name DB_PATH", err)
			}
		})
	}
}
