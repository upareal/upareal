// story: e01s01 e03s01 e09s01

// Package config reads the app configuration from environment variables once at startup.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/upareal/upareal/internal/vault"
)

const defaultOIDCIssuerURL = "https://accounts.google.com"

// Config is the validated app configuration.
type Config struct {
	DBPath             string   // DB_PATH: SQLite file path
	MasterKey          []byte   // MASTER_KEY: base64 of 32 bytes; wraps per-user data keys
	GoogleClientID     string   // GOOGLE_CLIENT_ID
	GoogleClientSecret string   // GOOGLE_CLIENT_SECRET
	BaseURL            string   // BASE_URL: public origin, e.g. http://localhost:8080
	OIDCIssuerURL      string   // OIDC_ISSUER_URL: optional; Google unless overridden (tests)
	AdminEmails        []string // ADMIN_EMAILS: optional, comma-separated; trimmed, lowercased
}

// Load reads configuration through getenv (os.Getenv in production) and fails
// fast, naming every missing or invalid variable. Secret values never appear
// in errors.
//
//	cfg, err := config.Load(os.Getenv)
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DBPath:             getenv("DB_PATH"),
		GoogleClientID:     getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET"),
		BaseURL:            getenv("BASE_URL"),
		OIDCIssuerURL:      getenv("OIDC_ISSUER_URL"),
	}
	for _, e := range strings.Split(getenv("ADMIN_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			cfg.AdminEmails = append(cfg.AdminEmails, e)
		}
	}
	if cfg.OIDCIssuerURL == "" {
		cfg.OIDCIssuerURL = defaultOIDCIssuerURL
	}

	var missing []string
	for _, v := range []struct{ name, value string }{
		{"DB_PATH", cfg.DBPath},
		{"MASTER_KEY", getenv("MASTER_KEY")},
		{"GOOGLE_CLIENT_ID", cfg.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", cfg.GoogleClientSecret},
		{"BASE_URL", cfg.BaseURL},
	} {
		if v.value == "" {
			missing = append(missing, v.name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("config: required env vars not set: %s (see README → Configuration)", strings.Join(missing, ", "))
	}

	var errs []error
	key, err := vault.ParseMasterKey(getenv("MASTER_KEY"))
	if err != nil {
		errs = append(errs, fmt.Errorf("MASTER_KEY: %w", err))
	}
	cfg.MasterKey = key
	if u, err := url.Parse(cfg.BaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("BASE_URL %q must be an absolute http(s) URL, e.g. http://localhost:8080", cfg.BaseURL))
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("config: %w", errors.Join(errs...))
	}
	return cfg, nil
}
