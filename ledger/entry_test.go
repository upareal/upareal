// story: e06s01
package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/upareal/upareal/ledger"
)

type book struct {
	itau, nubank, shell, salario, archived ledger.Account
}

func newBook(t *testing.T, f fixture) book {
	t.Helper()
	ctx := context.Background()
	var b book
	b.itau = create(t, f, ledger.NewAccount{Name: "Itaú", Type: "asset", OnBudget: true, Opening: 100000, OpeningAt: jan10})
	b.nubank = create(t, f, ledger.NewAccount{Name: "Nubank", Type: "liability", IsCard: true})
	b.shell = create(t, f, ledger.NewAccount{Name: "Posto Shell", Type: "expense"})
	b.salario = create(t, f, ledger.NewAccount{Name: "Salário", Type: "income"})
	b.archived = create(t, f, ledger.NewAccount{Name: "Velha", Type: "expense"})
	if err := f.svc.ArchiveAccount(ctx, userA, b.archived.ID); err != nil {
		t.Fatal(err)
	}
	return b
}

func balance(t *testing.T, f fixture, id string) int64 {
	t.Helper()
	b, err := f.svc.Balance(context.Background(), userA, id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRecordSimple_ExpenseAndIncome(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)

	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 15000, At: jan10, Description: "Gasolina"}); err != nil {
		t.Fatalf("expense: %v", err)
	}
	if balance(t, f, b.itau.ID) != 85000 || balance(t, f, b.shell.ID) != 15000 {
		t.Errorf("after expense: Itaú %d, Shell %d; want 85000, 15000", balance(t, f, b.itau.ID), balance(t, f, b.shell.ID))
	}
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.salario.ID, Amount: 800000, At: jan10}); err != nil {
		t.Fatalf("income: %v", err)
	}
	if balance(t, f, b.itau.ID) != 885000 || balance(t, f, b.salario.ID) != -800000 {
		t.Errorf("after income: Itaú %d, Salário %d; want 885000, -800000", balance(t, f, b.itau.ID), balance(t, f, b.salario.ID))
	}
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.nubank.ID, CounterID: b.shell.ID, Amount: 5000, At: jan10}); err != nil {
		t.Fatalf("card expense: %v", err)
	}
	if balance(t, f, b.nubank.ID) != -5000 {
		t.Errorf("card after expense = %d, want -5000 (owes R$ 50,00)", balance(t, f, b.nubank.ID))
	}
}

func TestRecordSimple_RejectsInvalid(t *testing.T) {
	f := newFixture(t)
	b := newBook(t, f)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		e    ledger.SimpleEntry
		want error
	}{
		"zero amount":           {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 0, At: jan10}, ledger.ErrInvalidEntry},
		"negative amount":       {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: -1, At: jan10}, ledger.ErrInvalidEntry},
		"account is an expense": {ledger.SimpleEntry{AccountID: b.shell.ID, CounterID: b.salario.ID, Amount: 1, At: jan10}, ledger.ErrInvalidEntry},
		"counter is an asset":   {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.nubank.ID, Amount: 1, At: jan10}, ledger.ErrInvalidEntry},
		"description too long":  {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 1, At: jan10, Description: strings.Repeat("x", 201)}, ledger.ErrInvalidEntry},
		"archived counter (L6)": {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.archived.ID, Amount: 1, At: jan10}, ledger.ErrAccountNotPostable},
		"unknown account (L3)":  {ledger.SimpleEntry{AccountID: "no-such", CounterID: b.shell.ID, Amount: 1, At: jan10}, ledger.ErrAccountNotFound},
		"missing time":          {ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 1}, ledger.ErrInvalidEntry},
	} {
		t.Run(name, func(t *testing.T) {
			before := f.count(t, `SELECT count(*) FROM ledger_transaction`)
			if _, err := f.svc.RecordSimple(ctx, userA, tc.e); !errors.Is(err, tc.want) {
				t.Errorf("RecordSimple = %v, want %v", err, tc.want)
			}
			if f.count(t, `SELECT count(*) FROM ledger_transaction`) != before {
				t.Error("an invalid entry was saved")
			}
		})
	}
}

func TestRecordSimple_CannotUseAnotherUsersAccount(t *testing.T) {
	f := newFixture(t)
	b := newBook(t, f)
	ctx := context.Background()
	_ = f.svc.EnsureDefaults(ctx, userB)
	mine, _ := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Minha", Type: "asset"})
	if _, err := f.svc.RecordSimple(ctx, userB, ledger.SimpleEntry{AccountID: mine.ID, CounterID: b.shell.ID, Amount: 100, At: jan10}); err == nil {
		t.Error("user B recorded against user A's expense account")
	}
	if balance(t, f, b.shell.ID) != 0 {
		t.Error("user A's account changed")
	}
}
