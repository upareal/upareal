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

// System accounts from ledger/defaults.go: Saldo inicial, A categorizar (income), A categorizar (expense).
const wantSystem = 3

func (f fixture) assertOneDefaultSet(t *testing.T, userID string) {
	t.Helper()
	if n := f.count(t, `SELECT count(*) FROM ledger_account WHERE user_id = ?`, userID); n != wantSystem {
		t.Errorf("accounts = %d, want only the %d system accounts", n, wantSystem)
	}
	if n := f.count(t, `SELECT count(DISTINCT system_role) FROM ledger_account WHERE user_id = ?`, userID); n != wantSystem {
		t.Errorf("distinct system roles = %d, want %d", n, wantSystem)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_user_key WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("ledger data keys = %d, want 1", n)
	}
}

// The ledger holds money movement only: categories live in the budget module.
func TestEnsureDefaults_LedgerHasNoCategories(t *testing.T) {
	f := newFixture(t)
	if n := f.count(t, `SELECT count(*) FROM sqlite_master WHERE name = 'ledger_group'`); n != 0 {
		t.Error("ledger_group table exists; groups and categories belong to the budget module")
	}
	if n := f.count(t, `SELECT count(*) FROM pragma_table_info('ledger_account') WHERE name = 'group_id'`); n != 0 {
		t.Error("ledger_account.group_id exists; categories belong to the budget module")
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
