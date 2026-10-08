// story: e01s01 e03s01 e04s01 e05s01 e08s01 e09s01
package web_test

import (
	"context"
	"database/sql"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/internal/oidctest"
	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
	"github.com/upareal/upareal/web"
)

// testApp is the full web app over a migrated temp database and a fake Google.
type testApp struct {
	handler http.Handler
	conn    *sql.DB
	fake    *oidctest.Provider
	ledger  *ledger.Service
}

// newTestApp builds the app as if served at baseURL (used for the OAuth redirect).
func newTestApp(t *testing.T, baseURL string) testApp {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	for _, m := range []struct {
		name string
		fsys fs.FS
	}{{"users", users.Migrations}, {"ledger", ledger.Migrations}, {"budget", budget.Migrations}, {"planning", planning.Migrations}} {
		name, fsys := m.name, m.fsys
		if err := db.Migrate(ctx, conn, name, fsys); err != nil {
			t.Fatal(err)
		}
	}
	master := make([]byte, 32)
	master[0] = 9
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	fake := oidctest.New(t, "web-client")
	google, err := users.NewGoogle(ctx, fake.URL, "web-client", "secret", baseURL+"/auth/callback")
	if err != nil {
		t.Fatal(err)
	}
	ledg := ledger.NewService(conn, kr)
	bud := budget.NewService(conn, kr)
	h := web.NewHandler(web.Deps{DB: conn, Users: users.NewService(conn, kr, users.AdminEmails("maria@example.com")), Ledger: ledg, Budget: bud,
		Planning: planning.NewService(conn, ledg, bud), Google: google})
	return testApp{handler: h, conn: conn, fake: fake, ledger: ledg}
}

// newHandler is the app handler for tests that do not sign in.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	return newTestApp(t, "http://app.test").handler
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
	res, body := get(t, newHandler(t), "/healthz")
	if res.StatusCode != http.StatusOK || strings.TrimSpace(body) != "ok" {
		t.Errorf("GET /healthz = %d %q, want 200 \"ok\"", res.StatusCode, body)
	}
}

func TestHealthz_UnavailableWhenDatabaseClosed(t *testing.T) {
	app := newTestApp(t, "http://app.test")
	_ = app.conn.Close()
	res, _ := get(t, app.handler, "/healthz")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /healthz with closed DB = %d, want 503", res.StatusCode)
	}
}
