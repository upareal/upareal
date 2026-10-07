// story: e07s02
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

// queued imports the statement into Itaú and returns the queued transaction IDs by description.
func queued(t *testing.T, f fixture, b book) map[string]string {
	t.Helper()
	ctx := context.Background()
	if _, err := f.svc.Import(ctx, userA, b.itau.ID, statement()); err != nil {
		t.Fatal(err)
	}
	q, err := f.svc.ReviewQueue(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, l := range q {
		ids[l.Description] = l.TransactionID
	}
	return ids
}

func queueLen(t *testing.T, f fixture) int {
	t.Helper()
	q, err := f.svc.ReviewQueue(context.Background(), userA)
	if err != nil {
		t.Fatal(err)
	}
	return len(q)
}

func TestCategorize_OneAccountLeavesTheQueue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú 100000
	ids := queued(t, f, b)
	if err := f.svc.Categorize(ctx, userA, ids["SUPERMERCADO X"], []ledger.Part{{CounterID: b.shell.ID, Amount: 15000}}); err != nil {
		t.Fatalf("Categorize: %v", err)
	}
	if queueLen(t, f) != 1 || balance(t, f, b.shell.ID) != 15000 || balance(t, f, f.systemID(t, userA, "uncategorized_expense")) != 0 {
		t.Errorf("after categorize: queue %d, Shell %d, A categorizar %d; want 1, 15000, 0",
			queueLen(t, f), balance(t, f, b.shell.ID), balance(t, f, f.systemID(t, userA, "uncategorized_expense")))
	}
	if balance(t, f, b.itau.ID) != 885000 {
		t.Error("the money account changed during review")
	}
	if h, _ := f.svc.History(ctx, userA, ids["SUPERMERCADO X"]); len(h) != 1 {
		t.Errorf("history rows = %d, want 1 (the A categorizar version)", len(h))
	}
}

func TestCategorize_SplitIncomeKeepingTheBankAmount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	extra := create(t, f, ledger.NewAccount{Name: "Renda extra", Type: "income"})
	ids := queued(t, f, b)
	err := f.svc.Categorize(ctx, userA, ids["SALARIO ACME"], []ledger.Part{{CounterID: b.salario.ID, Amount: 700000}, {CounterID: extra.ID, Amount: 100000}})
	if err != nil {
		t.Fatalf("Categorize split: %v", err)
	}
	if balance(t, f, b.salario.ID) != -700000 || balance(t, f, extra.ID) != -100000 || balance(t, f, b.itau.ID) != 885000 {
		t.Errorf("after split: Salário %d, Renda extra %d, Itaú %d", balance(t, f, b.salario.ID), balance(t, f, extra.ID), balance(t, f, b.itau.ID))
	}
}

func TestCategorize_BankAmountIsFixed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	ids := queued(t, f, b)
	err := f.svc.Categorize(ctx, userA, ids["SUPERMERCADO X"], []ledger.Part{{CounterID: b.shell.ID, Amount: 14000}})
	var mm ledger.SplitMismatch
	if !errors.As(err, &mm) || mm.Remaining != 1000 {
		t.Errorf("lines below the bank amount = %v, want SplitMismatch remaining 1000", err)
	}
	if queueLen(t, f) != 2 || balance(t, f, b.itau.ID) != 885000 {
		t.Error("a mismatched categorize changed something")
	}
}

func TestCategorize_GuardsNonQueuedAndForeign(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	ids := queued(t, f, b)
	manual := expense(t, f, b, 500, "Manual")
	if err := f.svc.Categorize(ctx, userA, manual, []ledger.Part{{CounterID: b.shell.ID, Amount: 500}}); !errors.Is(err, ledger.ErrNotQueued) {
		t.Errorf("categorize a manual transaction = %v, want ErrNotQueued", err)
	}
	if err := f.svc.Categorize(ctx, userA, ids["SUPERMERCADO X"], []ledger.Part{{CounterID: b.nubank.ID, Amount: 15000}}); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("categorize onto a card = %v, want ErrInvalidEntry", err)
	}
	if err := f.svc.Categorize(ctx, userA, ids["SUPERMERCADO X"], []ledger.Part{{CounterID: f.systemID(t, userA, "uncategorized_expense"), Amount: 15000}}); !errors.Is(err, ledger.ErrInvalidEntry) {
		t.Errorf("categorize onto A categorizar = %v, want ErrInvalidEntry", err)
	}
	_ = f.svc.EnsureDefaults(ctx, userB)
	if err := f.svc.Categorize(ctx, userB, ids["SUPERMERCADO X"], []ledger.Part{{CounterID: b.shell.ID, Amount: 15000}}); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("user B categorizing user A's line = %v, want ErrTransactionNotFound", err)
	}
}
