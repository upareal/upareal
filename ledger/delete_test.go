// story: e09s02
package ledger_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func TestDeleteUserData_RemovesEveryLedgerRowOfTheUserOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // user A: accounts, opening balance
	id := expense(t, f, b, 1500, "Gasolina")
	if err := f.svc.EditEntry(ctx, userA, id, ledger.SplitEntry{AccountID: b.itau.ID, Total: 1600, At: jan10, Description: "Gasolina", Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 1600}}}); err != nil {
		t.Fatal(err) // history row
	}
	if _, err := f.svc.Import(ctx, userA, b.itau.ID, []importer.Line{{ImportID: "1", At: jan10, Description: "PIX", Amount: -500}}); err != nil {
		t.Fatal(err) // import line
	}
	if err := f.svc.EnsureDefaults(ctx, userB); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Conta B", Type: "asset", OnBudget: true, Opening: 1000, OpeningAt: jan10}); err != nil {
		t.Fatal(err)
	}
	before := dbtest.UserRows(t, f.conn, "ledger_", userB)
	legsB := countLegs(t, f, userB)

	for range 2 { // idempotent
		if err := f.svc.DeleteUserData(ctx, userA); err != nil {
			t.Fatalf("DeleteUserData: %v", err)
		}
	}
	dbtest.AssertGone(t, f.conn, "ledger_", userA)
	var legs int
	if err := f.conn.QueryRow(`SELECT count(*) FROM ledger_leg`).Scan(&legs); err != nil {
		t.Fatal(err)
	}
	if legs != legsB {
		t.Errorf("legs left = %d, want only user B's %d", legs, legsB)
	}
	for table, n := range dbtest.UserRows(t, f.conn, "ledger_", userB) {
		if n != before[table] {
			t.Errorf("user B %s: %d rows, was %d", table, n, before[table])
		}
	}
}

func countLegs(t *testing.T, f fixture, userID string) int {
	t.Helper()
	var n int
	if err := f.conn.QueryRow(`SELECT count(*) FROM ledger_leg l JOIN ledger_transaction x ON x.id = l.transaction_id WHERE x.user_id = ?`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
