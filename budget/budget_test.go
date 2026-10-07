// story: e05s01
package budget_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/internal/vault"
)

type fixture struct {
	conn *sql.DB
	svc  *budget.Service
	path string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "budget.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(context.Background(), conn, "budget", budget.Migrations); err != nil {
		t.Fatal(err)
	}
	master := make([]byte, 32)
	master[0] = 5
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	return fixture{conn: conn, svc: budget.NewService(conn, kr), path: path}
}

func (f fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

const (
	userA          = "0196a000-0000-7000-8000-00000000000a"
	userB          = "0196a000-0000-7000-8000-00000000000b"
	wantGroups     = 10 // budget/defaults.go
	wantCategories = 37
)

func (f fixture) assertOneDefaultSet(t *testing.T, userID string) {
	t.Helper()
	if n := f.count(t, `SELECT count(*) FROM budget_group WHERE user_id = ?`, userID); n != wantGroups {
		t.Errorf("groups = %d, want %d", n, wantGroups)
	}
	if n := f.count(t, `SELECT count(*) FROM budget_category WHERE user_id = ?`, userID); n != wantCategories {
		t.Errorf("categories = %d, want %d", n, wantCategories)
	}
	if n := f.count(t, `SELECT count(*) FROM budget_user_key WHERE user_id = ?`, userID); n != 1 {
		t.Errorf("budget data keys = %d, want 1", n)
	}
}

func TestEnsureDefaults_SeedsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for range 2 {
		if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
			t.Fatalf("EnsureDefaults: %v", err)
		}
		f.assertOneDefaultSet(t, userA)
	}
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
