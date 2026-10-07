// story: e06s03
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func TestTransfer_PayCardBillAndMoveToSavings(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú 100000; Nubank card
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.nubank.ID, CounterID: b.shell.ID, Amount: 80000, At: jan10}); err != nil {
		t.Fatal(err)
	}
	savings := create(t, f, ledger.NewAccount{Name: "Poupança", Type: "asset"})

	if _, err := f.svc.Transfer(ctx, userA, ledger.TransferEntry{FromID: b.itau.ID, ToID: b.nubank.ID, Amount: 80000, At: jan10, Description: "Fatura"}); err != nil {
		t.Fatalf("pay card: %v", err)
	}
	if balance(t, f, b.itau.ID) != 20000 || balance(t, f, b.nubank.ID) != 0 {
		t.Errorf("after paying the card: Itaú %d, Nubank %d; want 20000, 0", balance(t, f, b.itau.ID), balance(t, f, b.nubank.ID))
	}
	if _, err := f.svc.Transfer(ctx, userA, ledger.TransferEntry{FromID: b.itau.ID, ToID: savings.ID, Amount: 5000, At: jan10}); err != nil {
		t.Fatalf("to savings: %v", err)
	}
	if balance(t, f, b.itau.ID) != 15000 || balance(t, f, savings.ID) != 5000 {
		t.Errorf("after saving: Itaú %d, Poupança %d; want 15000, 5000", balance(t, f, b.itau.ID), balance(t, f, savings.ID))
	}
}

func TestTransfer_RejectsInvalid(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	oldBank := create(t, f, ledger.NewAccount{Name: "Banco antigo", Type: "asset"})
	if err := f.svc.ArchiveAccount(ctx, userA, oldBank.ID); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		e    ledger.TransferEntry
		want error
	}{
		"same account":   {ledger.TransferEntry{FromID: b.itau.ID, ToID: b.itau.ID, Amount: 100, At: jan10}, ledger.ErrInvalidEntry},
		"zero amount":    {ledger.TransferEntry{FromID: b.itau.ID, ToID: b.nubank.ID, Amount: 0, At: jan10}, ledger.ErrInvalidEntry},
		"to an expense":  {ledger.TransferEntry{FromID: b.itau.ID, ToID: b.shell.ID, Amount: 100, At: jan10}, ledger.ErrInvalidEntry},
		"from an income": {ledger.TransferEntry{FromID: b.salario.ID, ToID: b.itau.ID, Amount: 100, At: jan10}, ledger.ErrInvalidEntry},
		"archived (L6)":  {ledger.TransferEntry{FromID: b.itau.ID, ToID: oldBank.ID, Amount: 100, At: jan10}, ledger.ErrAccountNotPostable},
		"unknown (L3)":   {ledger.TransferEntry{FromID: b.itau.ID, ToID: "no-such", Amount: 100, At: jan10}, ledger.ErrAccountNotFound},
		"missing time":   {ledger.TransferEntry{FromID: b.itau.ID, ToID: b.nubank.ID, Amount: 100}, ledger.ErrInvalidEntry},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.Transfer(ctx, userA, tc.e); !errors.Is(err, tc.want) {
				t.Errorf("Transfer = %v, want %v", err, tc.want)
			}
		})
	}
	if balance(t, f, b.itau.ID) != 100000 {
		t.Error("a rejected transfer changed Itaú")
	}
}

func TestTransfer_CannotReachAnotherUsersAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	_ = f.svc.EnsureDefaults(ctx, userB)
	mine, _ := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Minha", Type: "asset", Opening: 1000, OpeningAt: jan10})
	if _, err := f.svc.Transfer(ctx, userB, ledger.TransferEntry{FromID: mine.ID, ToID: b.itau.ID, Amount: 1000, At: jan10}); err == nil {
		t.Error("user B transferred into user A's account")
	}
	if balance(t, f, b.itau.ID) != 100000 {
		t.Error("user A's account changed")
	}
}
