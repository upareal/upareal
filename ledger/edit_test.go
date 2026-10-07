// story: e06s04
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func expense(t *testing.T, f fixture, b book, amount int64, desc string) string {
	t.Helper()
	id, err := f.svc.RecordSimple(context.Background(), userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: amount, At: jan10, Description: desc})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func legOf(tx ledger.Transaction, accountID string) ledger.TxLeg {
	for _, l := range tx.Legs {
		if l.AccountID == accountID {
			return l
		}
	}
	return ledger.TxLeg{}
}

func TestTransactionRead_LoadsLegsWithTypes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 15000, "Gasolina")

	tx, err := f.svc.Transaction(ctx, userA, id)
	if err != nil {
		t.Fatalf("Transaction: %v", err)
	}
	if tx.ID != id || tx.Description != "Gasolina" || !tx.At.Equal(jan10) || len(tx.Legs) != 2 {
		t.Fatalf("Transaction = %+v", tx)
	}
	if l := legOf(tx, b.shell.ID); l.Amount != 15000 || l.AccountType != "expense" {
		t.Errorf("Shell leg = %+v, want +15000 expense", l)
	}
	if l := legOf(tx, b.itau.ID); l.Amount != -15000 || l.AccountType != "asset" {
		t.Errorf("Itaú leg = %+v, want -15000 asset", l)
	}
	if h, err := f.svc.History(ctx, userA, id); err != nil || len(h) != 0 {
		t.Errorf("History of an untouched transaction = %v, %v; want empty", h, err)
	}
}

func TestTransactionRead_NotFoundForUnknownOrForeign(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 100, "")
	_ = f.svc.EnsureDefaults(ctx, userB)
	if _, err := f.svc.Transaction(ctx, userA, "no-such"); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("unknown = %v, want ErrTransactionNotFound", err)
	}
	if _, err := f.svc.Transaction(ctx, userB, id); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("user B reading user A's transaction = %v, want ErrTransactionNotFound", err)
	}
}
