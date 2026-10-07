// story: e01s01
package web_test

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/web"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func get(t *testing.T, h http.Handler, path string) (*http.Response, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func TestHealthz_OKWhenDatabaseAnswers(t *testing.T) {
	res, body := get(t, web.NewHandler(openTestDB(t)), "/healthz")
	if res.StatusCode != http.StatusOK || strings.TrimSpace(body) != "ok" {
		t.Errorf("GET /healthz = %d %q, want 200 \"ok\"", res.StatusCode, body)
	}
}

func TestHealthz_UnavailableWhenDatabaseClosed(t *testing.T) {
	conn := openTestDB(t)
	_ = conn.Close()
	res, _ := get(t, web.NewHandler(conn), "/healthz")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /healthz with closed DB = %d, want 503", res.StatusCode)
	}
}
