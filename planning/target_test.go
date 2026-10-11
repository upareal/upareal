// story: e16s01
package planning_test

import (
	"context"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/planning"
)

// targetBook maps Posto Shell to Combustível and gives Combustível a target
// due `months` months after the current month (both included: months+1 left).
type targetBook struct {
	shell, itau, fuel string
	cur              budget.Month
}

func newTargetBook(t *testing.T, f fixture, amount int64, months int) targetBook {
	t.Helper()
	ctx := context.Background()
	tb := targetBook{
		shell: f.account(t, userA, "Posto Shell", "expense").ID,
		itau:  f.account(t, userA, "Itaú", "asset").ID,
		fuel:  f.category(t, userA, "Combustível"),
		cur:   budget.MonthOf(time.Now()),
	}
	if err := f.svc.SetMapping(ctx, userA, tb.shell, tb.fuel); err != nil {
		t.Fatal(err)
	}
	due := tb.cur
	for range months {
		due = due.Next()
	}
	if err := f.bud.SetTarget(ctx, userA, tb.fuel, amount, due); err != nil {
		t.Fatal(err)
	}
	return tb
}

func (tb targetBook) at(t *testing.T, f fixture, m budget.Month) *planning.TargetPlan {
	t.Helper()
	p, err := f.svc.Month(context.Background(), userA, m)
	if err != nil {
		t.Fatal(err)
	}
	return p.Categories[tb.fuel].Target
}

func (tb targetBook) set(t *testing.T, f fixture, m budget.Month, amount int64) {
	t.Helper()
	if err := f.bud.SetAmount(context.Background(), userA, tb.fuel, m, amount); err != nil {
		t.Fatal(err)
	}
}

func TestTarget_NeededSplitsWhatIsMissingOverTheMonthsLeft(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 240000, 5) // 6 months, current included
	got := tb.at(t, f, tb.cur)
	if got == nil || got.Amount != 240000 || got.Needed != 40000 || got.Progress != 0 || got.Status != planning.TargetBehind {
		t.Fatalf("empty month = %+v; want needed 40000, 0%%, behind", got)
	}
	tb.set(t, f, tb.cur, 40000)
	if got := tb.at(t, f, tb.cur); got.Needed != 40000 || got.Progress != 16 || got.Status != planning.TargetOnTrack {
		t.Errorf("after putting 400 = %+v; want needed 40000, 16%%, on track", got)
	}
	if got := tb.at(t, f, tb.cur.Next()); got.Needed != 40000 || got.Status != planning.TargetOnTrack {
		t.Errorf("next month (amount copied) = %+v; want needed 40000, on track", got)
	}
}

func TestTarget_SkippedMonthRaisesNeeded(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 240000, 5)
	tb.set(t, f, tb.cur, 0)
	if got := tb.at(t, f, tb.cur.Next()); got.Needed != 48000 || got.Status != planning.TargetBehind {
		t.Errorf("after a skipped month = %+v; want needed 48000 (2400 / 5), behind", got)
	}
}

func TestTarget_NeededRoundsUpToTheCentavo(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 100001, 2) // 3 months
	if got := tb.at(t, f, tb.cur); got.Needed != 33334 {
		t.Errorf("needed = %d, want 33334 (rounded up)", got.Needed)
	}
}

func TestTarget_ReachedThenSpent(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 120000, 1)
	tb.set(t, f, tb.cur, 150000)
	if got := tb.at(t, f, tb.cur); got.Status != planning.TargetReached || got.Progress != 100 || got.Needed != 0 {
		t.Errorf("over-funded = %+v; want reached, 100%%, needed 0", got)
	}
	f.post(t, tb.itau, tb.shell, 100000, tb.cur.Start().Add(time.Hour)) // spending leaves R$ 500,00
	if got := tb.at(t, f, tb.cur); got.Status != planning.TargetOnTrack || got.Progress != 41 {
		t.Errorf("after spending = %+v; want on track (amount >= needed), 41%%", got)
	}
}

func TestTarget_OverdueAfterTheDueMonthUntilReached(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 120000, 0) // due this month
	tb.set(t, f, tb.cur, 20000)
	next := tb.cur.Next()
	if got := tb.at(t, f, next); got.Status != planning.TargetOverdue || got.Needed != 0 || got.Progress != 33 {
		t.Errorf("after the due month = %+v; want overdue, needed 0, 33%%", got)
	}
	tb.set(t, f, next, 100000)
	if got := tb.at(t, f, next); got.Status != planning.TargetReached {
		t.Errorf("funded after the due month = %+v; want reached", got)
	}
}

func TestTarget_NilWithoutTarget(t *testing.T) {
	f := newFixture(t)
	tb := newTargetBook(t, f, 100, 0)
	other := f.category(t, userA, "Mercado")
	p, err := f.svc.Month(context.Background(), userA, tb.cur)
	if err != nil {
		t.Fatal(err)
	}
	if p.Categories[other].Target != nil {
		t.Errorf("category without a target got %+v", p.Categories[other].Target)
	}
	pb, err := f.svc.Month(context.Background(), userB, tb.cur)
	if err != nil {
		t.Fatal(err)
	}
	for id, c := range pb.Categories {
		if c.Target != nil {
			t.Errorf("user B category %s has user A's target %+v", id, c.Target)
		}
	}
}
