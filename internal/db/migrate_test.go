// story: e03s01
package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/upareal/upareal/internal/db"
)

func migration(sql string) fstest.MapFS {
	return fstest.MapFS{"00001_init.sql": {Data: []byte("-- +goose Up\n" + sql + "\n-- +goose Down\nSELECT 1;\n")}}
}

func tableExists(t *testing.T, conn *sql.DB, name string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestMigrate_AppliesEachModuleWithItsOwnVersionTable(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	if err := db.Migrate(ctx, conn, "alpha", migration("CREATE TABLE alpha_thing (id TEXT PRIMARY KEY);")); err != nil {
		t.Fatalf("Migrate alpha: %v", err)
	}
	if err := db.Migrate(ctx, conn, "beta", migration("CREATE TABLE beta_thing (id TEXT PRIMARY KEY);")); err != nil {
		t.Fatalf("Migrate beta: %v", err)
	}
	for _, table := range []string{"alpha_thing", "beta_thing", "goose_alpha", "goose_beta"} {
		if !tableExists(t, conn, table) {
			t.Errorf("table %s missing after migrations", table)
		}
	}

	// Running again is a no-op, not an error.
	if err := db.Migrate(ctx, conn, "alpha", migration("CREATE TABLE alpha_thing (id TEXT PRIMARY KEY);")); err != nil {
		t.Errorf("second Migrate alpha: %v", err)
	}
}

func TestMigrate_FailureNamesTheModule(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	err = db.Migrate(context.Background(), conn, "broken", migration("CREATE TABLE ( nonsense;"))
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("Migrate with bad SQL = %v, want error naming module", err)
	}
}
