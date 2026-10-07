// story: e08s04
package planning_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
)

// cardBook: Itaú R$ 1.000,00 in December, Nubank card, Combustível R$ 300,00 in January.
func cardBook(t *testing.T, f fixture, cardOpening int64) (monthBook, ledger.Account) {
	t.Helper()
	ctx := context.Background()
	mb := newMonthBook(t, f)
	if err := f.ledg.AdjustBalance(ctx, userA, mb.itau.ID, 100000, jan10.AddDate(0, -1, 0)); err != nil {
		t.Fatal(err)
	}
	nubank, err := f.ledg.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Nubank", Type: "liability", IsCard: true, Opening: cardOpening, OpeningAt: jan10.AddDate(0, -1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	return mb, nubank
}

func month(t *testing.T, f fixture, mb monthBook) planning.MonthPlan {
	t.Helper()
	p, err := f.svc.Month(context.Background(), userA, mb.jan)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCards_FundedSpendingIsSetAsideWithoutChangingToBeAssigned(t *testing.T) {
	f := newFixture(t)
	mb, nubank := cardBook(t, f, 0)
	f.post(t, nubank.ID, mb.shell.ID, 4500, jan10)
	p := month(t, f, mb)
	if len(p.Cards) != 1 || p.Cards[0].AccountID != nubank.ID || p.Cards[0].Name != "Nubank" || p.Cards[0].Due != 4500 {
		t.Fatalf("cards = %+v; want Nubank due 4500", p.Cards)
	}
	if p.ToBeAssigned != 70000 || p.Categories[mb.fuel].Available != 25500 || p.Cash != 100000 || p.Uncovered != 0 {
		t.Errorf("A distribuir %d, Combustível %d, cash %d, uncovered %d; want 70000, 25500, 100000, 0",
			p.ToBeAssigned, p.Categories[mb.fuel].Available, p.Cash, p.Uncovered)
	}
}

func TestCards_OverspendingOnTheCardIsUncovered(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb, nubank := cardBook(t, f, 0)
	f.post(t, nubank.ID, mb.shell.ID, 40000, jan10)
	if p := month(t, f, mb); p.Uncovered != 10000 {
		t.Errorf("uncovered = %d, want 10000 (R$ 400 on the card, R$ 300 assigned)", p.Uncovered)
	}
	if err := f.bud.SetAmount(ctx, userA, mb.fuel, mb.jan, 40000); err != nil {
		t.Fatal(err)
	}
	if p := month(t, f, mb); p.Uncovered != 0 {
		t.Errorf("uncovered after covering Combustível = %d, want 0", p.Uncovered)
	}
}

func TestCards_PayingTheBillLowersDueAndCashOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb, nubank := cardBook(t, f, 0)
	f.post(t, nubank.ID, mb.shell.ID, 4500, jan10)
	if _, err := f.ledg.Transfer(ctx, userA, ledger.TransferEntry{FromID: mb.itau.ID, ToID: nubank.ID, Amount: 4500, At: jan10, Description: "Fatura"}); err != nil {
		t.Fatal(err)
	}
	p := month(t, f, mb)
	if len(p.Cards) != 0 || p.Cash != 95500 || p.ToBeAssigned != 70000 || p.Categories[mb.fuel].Available != 25500 {
		t.Errorf("after paying: cards %+v, cash %d, A distribuir %d, Combustível %d; want none, 95500, 70000, 25500",
			p.Cards, p.Cash, p.ToBeAssigned, p.Categories[mb.fuel].Available)
	}
}

func TestCards_OpeningDebtLowersToBeAssignedAndIsCovered(t *testing.T) {
	f := newFixture(t)
	mb, _ := cardBook(t, f, -50000)
	p := month(t, f, mb)
	if p.ToBeAssigned != 20000 || len(p.Cards) != 1 || p.Cards[0].Due != 50000 || p.Uncovered != 0 {
		t.Errorf("A distribuir %d, cards %+v, uncovered %d; want 20000, Nubank due 50000, 0", p.ToBeAssigned, p.Cards, p.Uncovered)
	}
}
