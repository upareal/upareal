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
