// story: e06s01
package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestRegister_MonthWithRunningBalanceAndCounterparts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú opened with 100000 on jan10
	at := func(day, hour int) time.Time { return time.Date(2026, 2, day, hour, 0, 0, 0, time.UTC) }
	record := func(counter string, amount int64, when time.Time, desc string) {
		t.Helper()
		if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: counter, Amount: amount, At: when, Description: desc}); err != nil {
			t.Fatal(err)
		}
	}
	record(b.shell.ID, 2000, time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC), "Janeiro")
	record(b.salario.ID, 800000, at(20, 12), "Salário fevereiro")
	record(b.shell.ID, 15000, at(5, 12), "Gasolina")
	record(b.shell.ID, 999, time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC), "Março") // exactly at `to`: excluded

	lines, err := f.svc.Register(ctx, userA, b.itau.ID, time.Date(2026, 2, 1, 3, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %+v, want 2 (February only)", lines)
	}
	first, second := lines[0], lines[1]
	if first.Description != "Gasolina" || first.Amount != -15000 || first.Balance != 83000 ||
		len(first.Counterparts) != 1 || first.Counterparts[0] != "Posto Shell" || !first.At.Equal(at(5, 12)) {
		t.Errorf("first = %+v, want Gasolina -15000 balance 83000 vs Posto Shell", first)
	}
	if second.Description != "Salário fevereiro" || second.Amount != 800000 || second.Balance != 883000 {
		t.Errorf("second = %+v, want Salário fevereiro +800000 balance 883000", second)
	}
}

func TestRegister_SplitShowsEveryCounterpart(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	if _, err := ledger.PostTransaction(ctx, f.svc, userA, jan10, "Mercado e posto", []ledger.Leg{
		{AccountID: b.itau.ID, Amount: -3000}, {AccountID: b.shell.ID, Amount: 2000}, {AccountID: b.salario.ID, Amount: 1000},
	}); err != nil {
		t.Fatal(err)
	}
	lines, err := f.svc.Register(ctx, userA, b.itau.ID, time.Date(2026, 1, 10, 15, 0, 1, 0, time.UTC).Add(-time.Second), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var split ledger.RegisterLine
	for _, l := range lines {
		if l.Description == "Mercado e posto" {
			split = l
		}
	}
	if split.Amount != -3000 || len(split.Counterparts) != 2 {
		t.Errorf("split line = %+v, want -3000 with 2 counterparts", split)
	}
}

func TestRegister_IsolatedPerUser(t *testing.T) {
	f := newFixture(t)
	b := newBook(t, f)
	_ = f.svc.EnsureDefaults(context.Background(), userB)
	if _, err := f.svc.Register(context.Background(), userB, b.itau.ID, jan10.AddDate(0, -1, 0), jan10.AddDate(0, 1, 0)); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B reading user A's register = %v, want ErrAccountNotFound", err)
	}
}
