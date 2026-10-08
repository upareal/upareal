// story: e08s02
package ledger_test

import (
	"context"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger"
)

func TestActivity_SumsLegsPerAccountInRange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	sp, _ := time.LoadLocation("America/Sao_Paulo")
	janStart, febStart := time.Date(2026, 1, 1, 0, 0, 0, 0, sp), time.Date(2026, 2, 1, 0, 0, 0, 0, sp)
	post := func(at time.Time, counter string, amount int64) string {
		id, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: counter, Amount: amount, At: at, Description: "x"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	post(jan10, b.shell.ID, 12000)
	post(janStart, b.shell.ID, 100)                   // first instant of January: in
	post(febStart.Add(-time.Second), b.shell.ID, 200) // 31/01 23:59:59 São Paulo: in
	post(febStart, b.shell.ID, 9999)                  // February: out
	post(jan10, b.salario.ID, 800000)
	deleted := post(jan10, b.shell.ID, 5000)
	if err := f.svc.DeleteTransaction(ctx, userA, deleted); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.Activity(ctx, userA, janStart, febStart)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if got[b.shell.ID] != 12300 {
		t.Errorf("Posto Shell = %d, want 12300 (spent, debit-positive, bounds [from,to), deleted excluded)", got[b.shell.ID])
	}
	if got[b.salario.ID] != -800000 {
		t.Errorf("Salário = %d, want -800000 (received is credit)", got[b.salario.ID])
	}
	if _, ok := got[b.nubank.ID]; ok {
		t.Error("an account without legs in the range must be absent")
	}
	if other, err := f.svc.Activity(ctx, userB, janStart, febStart); err != nil || len(other) != 0 {
		t.Errorf("user B activity = %v, %v; want empty", other, err)
	}
}

// story: e08s05
func TestBudgetFlows_OnlyLegsCrossingTheBudgetBoundary(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú on-budget with R$ 1.000,00 opening on jan10; Nubank card
	sp, _ := time.LoadLocation("America/Sao_Paulo")
	from, to := time.Date(2026, 1, 1, 0, 0, 0, 0, sp), time.Date(2026, 2, 1, 0, 0, 0, 0, sp)
	xp := create(t, f, ledger.NewAccount{Name: "Investimento XP", Type: "asset", Opening: 500000, OpeningAt: jan10}) // off-budget
	transfer := func(from, to string, amount int64) {
		if _, err := f.svc.Transfer(ctx, userA, ledger.TransferEntry{FromID: from, ToID: to, Amount: amount, At: jan10, Description: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	transfer(b.itau.ID, xp.ID, 100000)  // money leaves the budget
	transfer(xp.ID, b.itau.ID, 30000)   // money comes back
	transfer(b.itau.ID, b.nubank.ID, 1) // on-budget to on-budget: nothing crosses
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: xp.ID, CounterID: b.salario.ID, Amount: 4200, At: jan10, Description: "rendimento"}); err != nil {
		t.Fatal(err) // off-budget only: ignored
	}
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.nubank.ID, CounterID: b.shell.ID, Amount: 4500, At: jan10, Description: "x"}); err != nil {
		t.Fatal(err)
	}

	got, err := f.svc.BudgetFlows(ctx, userA, from, to)
	if err != nil {
		t.Fatalf("BudgetFlows: %v", err)
	}
	if got[xp.ID] != (ledger.Flow{Debit: 100000, Credit: -30000}) {
		t.Errorf("Investimento XP = %+v, want debit 100000, credit -30000", got[xp.ID])
	}
	if got[b.shell.ID] != (ledger.Flow{Debit: 4500}) {
		t.Errorf("Posto Shell = %+v, want debit 4500 (card spending crosses)", got[b.shell.ID])
	}
	if _, ok := got[b.salario.ID]; ok {
		t.Errorf("off-budget yield counted: %+v", got[b.salario.ID])
	}
	for _, on := range []string{b.itau.ID, b.nubank.ID} {
		if _, ok := got[on]; ok {
			t.Errorf("on-budget account %s has flows %+v; want none", on, got[on])
		}
	}
	var opening ledger.Flow
	accounts, _ := f.svc.Accounts(ctx, userA)
	for _, a := range accounts {
		if a.SystemRole == "opening_balance" {
			opening = got[a.ID]
		}
	}
	if opening != (ledger.Flow{Credit: -100000}) {
		t.Errorf("Saldo inicial = %+v, want only Itaú's opening (-100000), not the off-budget one", opening)
	}
	if other, err := f.svc.BudgetFlows(ctx, userB, from, to); err != nil || len(other) != 0 {
		t.Errorf("user B = %v, %v; want empty", other, err)
	}
}
