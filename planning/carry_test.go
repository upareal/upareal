// story: e08s03 e08s04 e08s05
package planning_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func mustMonth(t *testing.T, s string) budget.Month {
	t.Helper()
	m, err := budget.ParseMonth(s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCarry_LeftoverAndOverspendingCarryIntoNextMonth(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f) // Combustível R$ 300,00 in January
	f.post(t, mb.itau.ID, mb.shell.ID, 25000, jan10)
	feb := mustMonth(t, "2026-02")

	p, err := f.svc.Month(ctx, userA, feb) // opens February, copying R$ 300,00
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Categories[mb.fuel]; c.CarryOver != 5000 || c.Amount != 30000 || c.Available != 35000 {
		t.Errorf("February Combustível = %+v; want carry 5000, amount 30000, available 35000", c)
	}
	f.post(t, mb.itau.ID, mb.shell.ID, 7000, jan10) // January now overspent by R$ 20,00
	p, _ = f.svc.Month(ctx, userA, feb)
	if c := p.Categories[mb.fuel]; c.CarryOver != -2000 || c.Available != 28000 {
		t.Errorf("February after January overspent = %+v; want carry -2000, available 28000", c)
	}
	if jan, _ := f.svc.Month(ctx, userA, mb.jan); jan.Categories[mb.fuel].CarryOver != 0 {
		t.Errorf("January carry = %d, want 0 (no earlier month)", jan.Categories[mb.fuel].CarryOver)
	}
}

func TestCarry_ToBeAssignedIsCumulativeAndIncludesOpeningBalances(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	_, err := f.ledg.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Poupança", Type: "asset", OnBudget: true, Opening: 100000, OpeningAt: jan10.AddDate(0, -1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	f.post(t, mb.itau.ID, mb.salario.ID, 800000, jan10)
	feb := mustMonth(t, "2026-02")
	p, _ := f.svc.Month(ctx, userA, feb)
	// Money in through February: 1.000 opening + 8.000 salary; assigned: 300 (Jan) + 300 (Feb).
	if p.Received != 900000 || p.Assigned != 60000 || p.ToBeAssigned != 840000 {
		t.Errorf("February totals = in %d, assigned %d, to be assigned %d; want 900000, 60000, 840000", p.Received, p.Assigned, p.ToBeAssigned)
	}
}

// A distribuir + Σ Disponível must equal the money in on-budget accounts.
func TestCarry_IdentityHoldsEveryMonth(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	nubank, err := f.ledg.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Nubank", Type: "liability", IsCard: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ledg.AdjustBalance(ctx, userA, mb.itau.ID, 150000, jan10.AddDate(0, -1, 0)); err != nil {
		t.Fatal(err)
	}
	f.post(t, mb.itau.ID, mb.salario.ID, 800000, jan10)
	f.post(t, mb.itau.ID, mb.shell.ID, 32000, jan10)  // overspent in January
	f.post(t, nubank.ID, mb.shell.ID, 4500, jan10)    // on the card
	f.post(t, mb.itau.ID, mb.padaria.ID, 1500, jan10) // Sem categoria
	f.post(t, mb.itau.ID, mb.shell.ID, 9000, jan10.AddDate(0, 1, 0))
	xp := f.offBudget(t, "Investimento XP", 500000) // e08s05: off-budget life must not break the identity
	f.transfer(t, mb.itau.ID, xp.ID, 100000)
	f.transfer(t, xp.ID, mb.itau.ID, 30000)
	f.post(t, xp.ID, mb.salario.ID, 4200, jan10)
	if _, err := f.ledg.Import(ctx, userA, mb.itau.ID, []importer.Line{{ImportID: "q", At: jan10.AddDate(0, 1, 0), Description: "PIX", Amount: -3000}}); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"2025-12", "2026-01", "2026-02", "2026-03"} {
		p, err := f.svc.Month(ctx, userA, mustMonth(t, m))
		if err != nil {
			t.Fatal(err)
		}
		envelopes := p.ToBeAssigned
		for _, c := range p.Categories {
			envelopes += c.Available // 0 for income categories
		}
		for _, c := range p.Cards { // e08s04: card debt is set aside too
			envelopes += c.Due
		}
		cash := balanceAt(t, f, m, mb.itau.ID)
		if envelopes != cash || p.Cash != cash {
			t.Errorf("%s: A distribuir + Σ Disponível + Σ Para pagar = %d, Cash = %d, on-budget non-card money = %d", m, envelopes, p.Cash, cash)
		}
	}
}

// balanceAt sums the accounts' activity through the end of month m.
func balanceAt(t *testing.T, f fixture, m string, accounts ...string) int64 {
	t.Helper()
	end := mustMonth(t, m).Next().Start()
	act, err := f.ledg.Activity(context.Background(), userA, end.AddDate(-100, 0, 0), end)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, a := range accounts {
		sum += act[a]
	}
	return sum
}
