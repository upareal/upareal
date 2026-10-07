// story: e06s04
package ledger_test

import (
	"context"
	"errors"
	"testing"
	"time"

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

func TestEditKeepsHistory_EntryAmountAndCounter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	casa := create(t, f, ledger.NewAccount{Name: "Casa", Type: "expense"})
	id := expense(t, f, b, 15000, "Gasolina")

	err := f.svc.EditEntry(ctx, userA, id, ledger.SplitEntry{AccountID: b.itau.ID, Total: 12000, At: jan10, Description: "Gasolina corrigida",
		Parts: []ledger.Part{{CounterID: casa.ID, Amount: 12000}}})
	if err != nil {
		t.Fatalf("EditEntry: %v", err)
	}
	if balance(t, f, b.itau.ID) != 88000 || balance(t, f, b.shell.ID) != 0 || balance(t, f, casa.ID) != 12000 {
		t.Errorf("after edit: Itaú %d, Shell %d, Casa %d; want 88000, 0, 12000",
			balance(t, f, b.itau.ID), balance(t, f, b.shell.ID), balance(t, f, casa.ID))
	}
	tx, _ := f.svc.Transaction(ctx, userA, id)
	if tx.Description != "Gasolina corrigida" {
		t.Errorf("description = %q, want updated", tx.Description)
	}
	h, err := f.svc.History(ctx, userA, id)
	if err != nil || len(h) != 1 || h[0].Action != "edit" || h[0].Previous.Description != "Gasolina" || legOf(h[0].Previous, b.shell.ID).Amount != 15000 {
		t.Errorf("history = %+v, %v; want one edit with the previous Gasolina R$ 150,00", h, err)
	}
}

func TestEditKeepsHistory_Transfer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	savings := create(t, f, ledger.NewAccount{Name: "Poupança", Type: "asset"})
	id, err := f.svc.Transfer(ctx, userA, ledger.TransferEntry{FromID: b.itau.ID, ToID: b.nubank.ID, Amount: 10000, At: jan10})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.EditTransfer(ctx, userA, id, ledger.TransferEntry{FromID: b.itau.ID, ToID: savings.ID, Amount: 7000, At: jan10}); err != nil {
		t.Fatalf("EditTransfer: %v", err)
	}
	if balance(t, f, b.nubank.ID) != 0 || balance(t, f, savings.ID) != 7000 || balance(t, f, b.itau.ID) != 93000 {
		t.Errorf("after transfer edit: Nubank %d, Poupança %d, Itaú %d; want 0, 7000, 93000",
			balance(t, f, b.nubank.ID), balance(t, f, savings.ID), balance(t, f, b.itau.ID))
	}
}

func TestEditKeepsHistory_InvalidEditChangesNothing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 15000, "Gasolina")
	bad := ledger.SplitEntry{AccountID: b.itau.ID, Total: 12000, At: jan10, Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 11000}}}
	if err := f.svc.EditEntry(ctx, userA, id, bad); !errors.Is(err, ledger.ErrSplitMismatch) {
		t.Errorf("unbalanced edit = %v, want ErrSplitMismatch", err)
	}
	if balance(t, f, b.shell.ID) != 15000 {
		t.Error("an invalid edit changed the balance")
	}
	if h, _ := f.svc.History(ctx, userA, id); len(h) != 0 {
		t.Errorf("an invalid edit wrote %d history rows, want 0", len(h))
	}
}

func TestEditKeepsHistory_SystemAndForeignNotEditable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	// Itaú's opening balance touches Saldo inicial.
	lines, _ := f.svc.Register(ctx, userA, b.itau.ID, jan10.AddDate(0, 0, -1), jan10.AddDate(0, 0, 1))
	opening := lines[0].TransactionID
	if err := f.svc.EditEntry(ctx, userA, opening, ledger.SplitEntry{AccountID: b.itau.ID, Total: 1, At: jan10, Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 1}}}); !errors.Is(err, ledger.ErrNotEditable) {
		t.Errorf("editing the opening balance = %v, want ErrNotEditable", err)
	}
	id := expense(t, f, b, 100, "")
	_ = f.svc.EnsureDefaults(ctx, userB)
	if err := f.svc.EditEntry(ctx, userB, id, ledger.SplitEntry{AccountID: b.itau.ID, Total: 1, At: jan10, Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 1}}}); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("user B editing user A's transaction = %v, want ErrTransactionNotFound", err)
	}
}

func TestSoftDelete_LeavesEverySumAndList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú opened 100000 on jan10
	id := expense(t, f, b, 15000, "Gasolina")

	if err := f.svc.DeleteTransaction(ctx, userA, id); err != nil {
		t.Fatalf("DeleteTransaction: %v", err)
	}
	if balance(t, f, b.itau.ID) != 100000 || balance(t, f, b.shell.ID) != 0 {
		t.Errorf("Balance after delete: Itaú %d, Shell %d; want 100000, 0", balance(t, f, b.itau.ID), balance(t, f, b.shell.ID))
	}
	all, _ := f.svc.Balances(ctx, userA)
	if all[b.shell.ID] != 0 {
		t.Errorf("Balances still include the deleted transaction: Shell %d", all[b.shell.ID])
	}
	lines, _ := f.svc.Register(ctx, userA, b.itau.ID, jan10.AddDate(0, 0, -1), jan10.AddDate(0, 0, 1))
	for _, l := range lines {
		if l.TransactionID == id {
			t.Error("the deleted transaction is still in the register")
		}
	}
	// The balance carried into a later range (BalanceBefore) must ignore the deleted leg too.
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 1000, At: jan10.AddDate(0, 0, 1), Description: "Depois"}); err != nil {
		t.Fatal(err)
	}
	l2, _ := f.svc.Register(ctx, userA, b.itau.ID, jan10.Add(12*time.Hour), jan10.AddDate(0, 1, 0))
	if len(l2) != 1 || l2[0].Balance != 99000 {
		t.Errorf("running balance after the delete = %+v, want ending at 99000", l2)
	}
	if _, err := f.svc.Transaction(ctx, userA, id); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("reading a deleted transaction = %v, want ErrTransactionNotFound", err)
	}
	h, _ := f.svc.History(ctx, userA, id)
	if len(h) != 1 || h[0].Action != "delete" || h[0].Previous.Description != "Gasolina" {
		t.Errorf("history = %+v, want one delete with the previous version", h)
	}
}

func TestSoftDelete_AccountStillCountsAsUsed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 15000, "")
	_ = f.svc.DeleteTransaction(ctx, userA, id)
	if err := f.svc.DeleteAccount(ctx, userA, b.shell.ID); !errors.Is(err, ledger.ErrHasLegs) {
		t.Errorf("deleting an account whose only transaction was deleted = %v, want ErrHasLegs (archive instead)", err)
	}
}

func TestSoftDelete_GuardsForeignAndRepeated(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 100, "")
	_ = f.svc.EnsureDefaults(ctx, userB)
	if err := f.svc.DeleteTransaction(ctx, userB, id); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("user B deleting user A's transaction = %v, want ErrTransactionNotFound", err)
	}
	if err := f.svc.DeleteTransaction(ctx, userA, id); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteTransaction(ctx, userA, id); !errors.Is(err, ledger.ErrTransactionNotFound) {
		t.Errorf("deleting twice = %v, want ErrTransactionNotFound", err)
	}
}
