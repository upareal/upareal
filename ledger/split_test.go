// story: e06s02
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func TestRecordSplit_ExpensesAndCashback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú starts at 100000
	casa := create(t, f, ledger.NewAccount{Name: "Casa", Type: "expense"})

	id, err := f.svc.RecordSplit(ctx, userA, ledger.SplitEntry{AccountID: b.itau.ID, Total: 30000, At: jan10, Description: "Supermercado",
		Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 20000}, {CounterID: casa.ID, Amount: 10000}}})
	if err != nil || id == "" {
		t.Fatalf("RecordSplit = %q, %v", id, err)
	}
	if balance(t, f, b.itau.ID) != 70000 || balance(t, f, b.shell.ID) != 20000 || balance(t, f, casa.ID) != 10000 {
		t.Errorf("after split: Itaú %d, Shell %d, Casa %d; want 70000, 20000, 10000",
			balance(t, f, b.itau.ID), balance(t, f, b.shell.ID), balance(t, f, casa.ID))
	}
	// Cashback: an income line reduces what left the account.
	if _, err := f.svc.RecordSplit(ctx, userA, ledger.SplitEntry{AccountID: b.itau.ID, Total: 9500, At: jan10,
		Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 10000}, {CounterID: b.salario.ID, Amount: 500}}}); err != nil {
		t.Fatalf("split with cashback: %v", err)
	}
	if balance(t, f, b.itau.ID) != 60500 || balance(t, f, b.salario.ID) != -500 {
		t.Errorf("after cashback split: Itaú %d, Salário %d; want 60500, -500", balance(t, f, b.itau.ID), balance(t, f, b.salario.ID))
	}
	// The same counter account twice is allowed.
	if _, err := f.svc.RecordSplit(ctx, userA, ledger.SplitEntry{AccountID: b.itau.ID, Total: 300, At: jan10,
		Parts: []ledger.Part{{CounterID: casa.ID, Amount: 100}, {CounterID: casa.ID, Amount: 200}}}); err != nil {
		t.Errorf("same counter twice: %v", err)
	}
}

func TestRecordSplit_MismatchReportsRemaining(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	casa := create(t, f, ledger.NewAccount{Name: "Casa", Type: "expense"})
	for name, tc := range map[string]struct {
		total     int64
		parts     []ledger.Part
		remaining int64
	}{
		"missing":  {30000, []ledger.Part{{b.shell.ID, 20000}, {casa.ID, 5000}}, 5000},
		"too much": {30000, []ledger.Part{{b.shell.ID, 20000}, {casa.ID, 15000}}, -5000},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.RecordSplit(ctx, userA, ledger.SplitEntry{AccountID: b.itau.ID, Total: tc.total, At: jan10, Parts: tc.parts})
			var mm ledger.SplitMismatch
			if !errors.Is(err, ledger.ErrSplitMismatch) || !errors.As(err, &mm) || mm.Remaining != tc.remaining {
				t.Errorf("RecordSplit = %v (remaining %d), want ErrSplitMismatch with remaining %d", err, mm.Remaining, tc.remaining)
			}
		})
	}
	if balance(t, f, b.itau.ID) != 100000 {
		t.Error("a mismatched split changed the balance")
	}
}

func TestRecordSplit_RejectsInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	casa := create(t, f, ledger.NewAccount{Name: "Casa", Type: "expense"})
	ok := []ledger.Part{{b.shell.ID, 100}, {casa.ID, 100}}
	for name, tc := range map[string]struct {
		e    ledger.SplitEntry
		want error
	}{
		"one line":            {ledger.SplitEntry{AccountID: b.itau.ID, Total: 100, At: jan10, Parts: []ledger.Part{{b.shell.ID, 100}}}, ledger.ErrInvalidEntry},
		"zero total":          {ledger.SplitEntry{AccountID: b.itau.ID, Total: 0, At: jan10, Parts: ok}, ledger.ErrInvalidEntry},
		"zero line":           {ledger.SplitEntry{AccountID: b.itau.ID, Total: 100, At: jan10, Parts: []ledger.Part{{b.shell.ID, 100}, {casa.ID, 0}}}, ledger.ErrInvalidEntry},
		"paying from expense": {ledger.SplitEntry{AccountID: b.shell.ID, Total: 200, At: jan10, Parts: ok}, ledger.ErrInvalidEntry},
		"line on an asset":    {ledger.SplitEntry{AccountID: b.itau.ID, Total: 200, At: jan10, Parts: []ledger.Part{{b.shell.ID, 100}, {b.nubank.ID, 100}}}, ledger.ErrInvalidEntry},
		"archived line (L6)":  {ledger.SplitEntry{AccountID: b.itau.ID, Total: 200, At: jan10, Parts: []ledger.Part{{b.shell.ID, 100}, {b.archived.ID, 100}}}, ledger.ErrAccountNotPostable},
		"unknown line (L3)":   {ledger.SplitEntry{AccountID: b.itau.ID, Total: 200, At: jan10, Parts: []ledger.Part{{b.shell.ID, 100}, {"no-such", 100}}}, ledger.ErrAccountNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.RecordSplit(ctx, userA, tc.e); !errors.Is(err, tc.want) {
				t.Errorf("RecordSplit = %v, want %v", err, tc.want)
			}
		})
	}
}
