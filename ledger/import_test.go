// story: e07s01
package ledger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func statement() []importer.Line {
	return []importer.Line{
		{ImportID: "abc123", At: time.Date(2026, 1, 31, 22, 30, 0, 0, time.UTC), Description: "SUPERMERCADO X", Amount: -15000, Account: "Cartão final 1234"},
		{ImportID: "def456", At: time.Date(2026, 2, 1, 3, 0, 0, 0, time.UTC), Description: "SALARIO ACME", Amount: 800000},
	}
}

func TestImportDedupe_PostsAgainstACategorizarOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú 100000
	res, err := f.svc.Import(ctx, userA, b.itau.ID, statement())
	if err != nil || res.Imported != 2 || res.Skipped != 0 {
		t.Fatalf("Import = %+v, %v; want 2 imported", res, err)
	}
	if balance(t, f, b.itau.ID) != 885000 ||
		balance(t, f, f.systemID(t, userA, "uncategorized_expense")) != 15000 ||
		balance(t, f, f.systemID(t, userA, "uncategorized_income")) != -800000 {
		t.Errorf("balances after import: Itaú %d, A categorizar (despesa) %d, (receita) %d",
			balance(t, f, b.itau.ID), balance(t, f, f.systemID(t, userA, "uncategorized_expense")), balance(t, f, f.systemID(t, userA, "uncategorized_income")))
	}
	again, err := f.svc.Import(ctx, userA, b.itau.ID, statement())
	if err != nil || again.Imported != 0 || again.Skipped != 2 {
		t.Errorf("re-import = %+v, %v; want 0 imported, 2 skipped", again, err)
	}
	if balance(t, f, b.itau.ID) != 885000 {
		t.Error("re-import changed the balance")
	}
}

func TestImportDedupe_PerAccountAndAfterDelete(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	other := create(t, f, ledger.NewAccount{Name: "Bradesco", Type: "asset"})
	if res, _ := f.svc.Import(ctx, userA, b.itau.ID, statement()); res.Imported != 2 {
		t.Fatal("first import failed")
	}
	if res, err := f.svc.Import(ctx, userA, other.ID, statement()); err != nil || res.Imported != 2 {
		t.Errorf("same ids into another account = %+v, %v; want 2 imported (dedupe is per account)", res, err)
	}
	queue, _ := f.svc.ReviewQueue(ctx, userA)
	for _, q := range queue {
		if q.AccountID == b.itau.ID {
			_ = f.svc.DeleteTransaction(ctx, userA, q.TransactionID)
		}
	}
	if res, _ := f.svc.Import(ctx, userA, b.itau.ID, statement()); res.Imported != 0 || res.Skipped != 2 {
		t.Errorf("re-import after deleting = %+v; want 0 imported, 2 skipped (deleted lines stay imported)", res)
	}
}

func TestImportDedupe_AllOrNothingAndGuards(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	bad := append(statement(), importer.Line{ImportID: "zero", At: jan10, Amount: 0})
	if _, err := f.svc.Import(ctx, userA, b.itau.ID, bad); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("a zero line = %v, want ErrInvalidEntry", err)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_import_line`); n != 0 {
		t.Errorf("import lines after a failed import = %d, want 0 (all-or-nothing)", n)
	}
	if balance(t, f, b.itau.ID) != 100000 {
		t.Error("a failed import changed the balance")
	}
	if _, err := f.svc.Import(ctx, userA, b.shell.ID, statement()); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("importing into an expense account = %v, want ErrInvalidEntry", err)
	}
	_ = f.svc.EnsureDefaults(ctx, userB)
	if _, err := f.svc.Import(ctx, userB, b.itau.ID, statement()); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B importing into user A's account = %v, want ErrAccountNotFound", err)
	}
}
