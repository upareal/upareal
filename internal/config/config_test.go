// story: e01s01 e03s01
package config_test

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/config"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv(t *testing.T) map[string]string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"DB_PATH":              "/data/upareal.db",
		"MASTER_KEY":           base64.StdEncoding.EncodeToString(key),
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "http://localhost:8080",
	}
}

func TestLoad_ReadsAllSettings(t *testing.T) {
	env := validEnv(t)
	cfg, err := config.Load(envOf(env))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.DBPath != "/data/upareal.db" || cfg.GoogleClientID != "client-id" ||
		cfg.GoogleClientSecret != "client-secret" || cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("Load = %+v, want values from env", cfg)
	}
	if len(cfg.MasterKey) != 32 {
		t.Errorf("MasterKey length = %d, want 32", len(cfg.MasterKey))
	}
	if cfg.OIDCIssuerURL != "https://accounts.google.com" {
		t.Errorf("OIDCIssuerURL default = %q, want Google", cfg.OIDCIssuerURL)
	}
}

func TestLoad_OIDCIssuerOverride(t *testing.T) {
	env := validEnv(t)
	env["OIDC_ISSUER_URL"] = "http://127.0.0.1:9999"
	cfg, err := config.Load(envOf(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OIDCIssuerURL != "http://127.0.0.1:9999" {
		t.Errorf("OIDCIssuerURL = %q, want override", cfg.OIDCIssuerURL)
	}
}

func TestLoad_NamesEveryMissingRequiredVar(t *testing.T) {
	_, err := config.Load(envOf(map[string]string{}))
	if err == nil {
		t.Fatal("Load with empty env: expected error, got nil")
	}
	for _, name := range []string{"DB_PATH", "MASTER_KEY", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "BASE_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestLoad_FailsWhenOneVarMissingOrEmpty(t *testing.T) {
	for _, name := range []string{"DB_PATH", "MASTER_KEY", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "BASE_URL"} {
		t.Run(name, func(t *testing.T) {
			env := validEnv(t)
			env[name] = ""
			_, err := config.Load(envOf(env))
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("Load without %s = %v, want error naming it", name, err)
			}
		})
	}
}

func TestLoad_RejectsInvalidValuesWithoutLeakingSecrets(t *testing.T) {
	env := validEnv(t)
	env["MASTER_KEY"] = "short-secret-value"
	_, err := config.Load(envOf(env))
	if err == nil || !strings.Contains(err.Error(), "MASTER_KEY") {
		t.Fatalf("Load with short MASTER_KEY = %v, want error naming MASTER_KEY", err)
	}
	if strings.Contains(err.Error(), "short-secret-value") {
		t.Errorf("error leaks MASTER_KEY: %q", err)
	}

	env = validEnv(t)
	env["BASE_URL"] = "localhost:8080"
	if _, err := config.Load(envOf(env)); err == nil || !strings.Contains(err.Error(), "BASE_URL") {
		t.Errorf("Load with BASE_URL without scheme = %v, want error naming BASE_URL", err)
	}
}
