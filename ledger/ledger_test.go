// story: e04s01
package ledger_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/ledger"
)

type fixture struct {
	conn *sql.DB
	svc  *ledger.Service
	path string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn, "ledger", ledger.Migrations); err != nil {
		t.Fatal(err)
	}
	master := make([]byte, 32)
	master[0] = 4
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{conn: conn, svc: ledger.NewService(conn, kr), path: path}
}

func (f fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

const userA = "0196a000-0000-7000-8000-00000000000a"

// Default set from ledger/defaults.go: 10 groups, 37 categories, 3 system accounts.
const (
	wantGroups     = 10
	wantCategories = 37
	wantSystem     = 3
)

func (f fixture) assertOneDefaultSet(t *testing.T, userID string) {
	t.Helper()
	if n := f.count(t, `SELECT count(*) FROM ledger_group WHERE user_id = ?`, userID); n != wantGroups {
		t.Errorf("groups = %d, want %d", n, wantGroups)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_account WHERE user_id = ? AND group_id IS NOT NULL`, userID); n != wantCategories {
		t.Errorf("categories = %d, want %d", n, wantCategories)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_account WHERE user_id = ? AND system_role IS NOT NULL`, userID); n != wantSystem {
		t.Errorf("system accounts = %d, want %d", n, wantSystem)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_user_key WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("ledger data keys = %d, want 1", n)
	}
}

func TestEnsureDefaults_SeedsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	f.assertOneDefaultSet(t, userA)
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatalf("second EnsureDefaults: %v", err)
	}
	f.assertOneDefaultSet(t, userA)
}

func TestEnsureDefaults_ConcurrentCallsSeedOneSet(t *testing.T) {
	f := newFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- f.svc.EnsureDefaults(context.Background(), userA)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent EnsureDefaults: %v", err)
		}
	}
	f.assertOneDefaultSet(t, userA)
}

func TestEnsureDefaults_RequiresUserID(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.EnsureDefaults(context.Background(), ""); err == nil {
		t.Error("EnsureDefaults(\"\"): expected error, got nil")
	}
}
