// story: e08s02
package planning_test

import (
	"context"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

var jan10 = time.Date(2026, 1, 10, 15, 0, 0, 0, time.UTC)

type monthBook struct {
	itau, shell, padaria, salario ledger.Account
	fuel, salaryCat               string
	jan                           budget.Month
}

func newMonthBook(t *testing.T, f fixture) monthBook {
	t.Helper()
	ctx := context.Background()
	mb := monthBook{
		itau:    f.account(t, userA, "Itaú", "asset"),
		shell:   f.account(t, userA, "Posto Shell", "expense"),
		padaria: f.account(t, userA, "Padaria", "expense"), // stays unmapped
		salario: f.account(t, userA, "Salário", "income"),
		fuel:    f.category(t, userA, "Combustível"),
	}
	mb.salaryCat = f.category(t, userA, "Salário")
	mb.jan, _ = budget.ParseMonth("2026-01")
	for acc, cat := range map[string]string{mb.shell.ID: mb.fuel, mb.salario.ID: mb.salaryCat} {
		if err := f.svc.SetMapping(ctx, userA, acc, cat); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.bud.SetAmount(ctx, userA, mb.fuel, mb.jan, 30000); err != nil {
		t.Fatal(err)
	}
	if err := f.bud.SetAmount(ctx, userA, mb.salaryCat, mb.jan, 750000); err != nil {
		t.Fatal(err)
	}
	return mb
}

func (f fixture) post(t *testing.T, account, counter string, amount int64, at time.Time) {
	t.Helper()
	if _, err := f.ledg.RecordSimple(context.Background(), userA, ledger.SimpleEntry{AccountID: account, CounterID: counter, Amount: amount, At: at, Description: "x"}); err != nil {
		t.Fatal(err)
	}
}

func TestMonth_SpentAvailableReceivedAndToBeAssigned(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	f.post(t, mb.itau.ID, mb.shell.ID, 12000, jan10)
	f.post(t, mb.itau.ID, mb.shell.ID, 25000, jan10)
	f.post(t, mb.itau.ID, mb.shell.ID, 999, jan10.AddDate(0, 1, 0)) // February: not in January
	f.post(t, mb.itau.ID, mb.salario.ID, 800000, jan10)

	p, err := f.svc.Month(ctx, userA, mb.jan)
	if err != nil {
		t.Fatalf("Month: %v", err)
	}
	if c := p.Categories[mb.fuel]; c.Amount != 30000 || c.Spent != 37000 || c.Available != -7000 {
		t.Errorf("Combustível = %+v; want amount 30000, spent 37000, available -7000 (overspent)", c)
	}
	if c := p.Categories[mb.salaryCat]; c.Amount != 750000 || c.Received != 800000 {
		t.Errorf("Salário = %+v; want amount 750000, received 800000", c)
	}
	if p.Received != 800000 || p.Assigned != 30000 || p.ToBeAssigned != 770000 {
		t.Errorf("totals = received %d, assigned %d, to be assigned %d; want 800000, 30000, 770000", p.Received, p.Assigned, p.ToBeAssigned)
	}
	if p.UnmappedSpent != 0 || p.UnmappedReceived != 0 {
		t.Errorf("Sem categoria = %d / %d, want 0", p.UnmappedSpent, p.UnmappedReceived)
	}
}

func TestMonth_SemCategoriaCatchesUnmappedAndQueuedMoney(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	f.post(t, mb.itau.ID, mb.padaria.ID, 2000, jan10) // unmapped expense account
	if _, err := f.ledg.Import(ctx, userA, mb.itau.ID, []importer.Line{
		{ImportID: "1", At: jan10, Description: "PIX ENVIADO", Amount: -5000},  // A categorizar (expense)
		{ImportID: "2", At: jan10, Description: "PIX RECEBIDO", Amount: 10000}, // A categorizar (income)
	}); err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.Month(ctx, userA, mb.jan)
	if err != nil {
		t.Fatal(err)
	}
	if p.UnmappedSpent != 7000 || p.UnmappedReceived != 10000 {
		t.Errorf("Sem categoria = spent %d, received %d; want 7000 and 10000 (P3)", p.UnmappedSpent, p.UnmappedReceived)
	}
	if p.Received != 10000 || p.ToBeAssigned != 10000-30000 {
		t.Errorf("received %d, to be assigned %d; want 10000 and -20000 (unmapped income still counts)", p.Received, p.ToBeAssigned)
	}
}

func TestMonth_IsolatedPerUser(t *testing.T) {
	f := newFixture(t)
	mb := newMonthBook(t, f)
	f.post(t, mb.itau.ID, mb.shell.ID, 12000, jan10)
	p, err := f.svc.Month(context.Background(), userB, mb.jan)
	if err != nil {
		t.Fatal(err)
	}
	if p.Received != 0 || p.Assigned != 0 || p.UnmappedSpent != 0 {
		t.Errorf("user B month = %+v; want nothing of user A", p)
	}
	for id, c := range p.Categories {
		if c.Spent != 0 || id == mb.fuel {
			t.Errorf("user B sees category %s = %+v", id, c)
		}
	}
}
