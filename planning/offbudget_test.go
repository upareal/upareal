// story: e08s05
package planning_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
)

func (f fixture) offBudget(t *testing.T, name string, opening int64) ledger.Account {
	t.Helper()
	a, err := f.ledg.CreateAccount(context.Background(), userA, ledger.NewAccount{Name: name, Type: "asset", Opening: opening, OpeningAt: jan10.AddDate(0, -1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (f fixture) transfer(t *testing.T, from, to string, amount int64) {
	t.Helper()
	if _, err := f.ledg.Transfer(context.Background(), userA, ledger.TransferEntry{FromID: from, ToID: to, Amount: amount, At: jan10, Description: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestMapping_OffBudgetAccountsMapToExpenseCategories(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	xp := f.offBudget(t, "Investimento XP", 0)
	if err := f.svc.SetMapping(ctx, userA, xp.ID, f.category(t, userA, "Investimentos")); err != nil {
		t.Fatalf("off-budget to expense category: %v", err)
	}
	if err := f.svc.SetMapping(ctx, userA, xp.ID, f.category(t, userA, "Salário")); !errors.Is(err, planning.ErrKindMismatch) {
		t.Errorf("off-budget to income category = %v, want ErrKindMismatch", err)
	}
	ms, _ := f.svc.Mappings(ctx, userA)
	if m, ok := byName(ms)["Investimento XP"]; !ok || m.CategoryName != "Investimentos" {
		t.Errorf("Mappings = %+v; want Investimento XP mapped to Investimentos", ms)
	}
}

func TestMonth_OffBudgetOutflowIsSpentAndInflowIsMoneyIn(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	xp := f.offBudget(t, "Investimento XP", 500000) // opening off-budget: ignored
	invest := f.category(t, userA, "Investimentos")
	if err := f.svc.SetMapping(ctx, userA, xp.ID, invest); err != nil {
		t.Fatal(err)
	}
	f.post(t, mb.itau.ID, mb.salario.ID, 800000, jan10)
	f.transfer(t, mb.itau.ID, xp.ID, 100000)
	f.transfer(t, xp.ID, mb.itau.ID, 30000)
	f.post(t, xp.ID, mb.salario.ID, 4200, jan10) // yield inside the investment: ignored

	p, err := f.svc.Month(ctx, userA, mb.jan)
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Categories[invest]; c.Spent != 100000 {
		t.Errorf("Investimentos spent = %d, want 100000", c.Spent)
	}
	if c := p.Categories[mb.salaryCat]; c.Received != 800000 {
		t.Errorf("Salário received = %d, want 800000 (the off-budget yield is not budget income)", c.Received)
	}
	// Money in: 8.000 salary + 300 redeemed. Assigned: 300 (Combustível).
	if p.Received != 830000 || p.ToBeAssigned != 800000 {
		t.Errorf("received %d, A distribuir %d; want 830000, 800000", p.Received, p.ToBeAssigned)
	}
}

func TestMonth_UnmappedOffBudgetOutflowIsSemCategoria(t *testing.T) {
	f := newFixture(t)
	mb := newMonthBook(t, f)
	xp := f.offBudget(t, "Investimento XP", 0)
	f.transfer(t, mb.itau.ID, xp.ID, 100000)
	if p := month(t, f, mb); p.UnmappedSpent != 100000 {
		t.Errorf("Sem categoria spent = %d, want 100000", p.UnmappedSpent)
	}
}
