// story: e16s01
package budget_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
)

func targetsOf(t *testing.T, f fixture, userID string) map[string]budget.Target {
	t.Helper()
	ts, err := f.svc.Targets(context.Background(), userID)
	if err != nil {
		t.Fatalf("Targets: %v", err)
	}
	return ts
}

func TestTarget_SetReplaceRemove(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	ipva := categoryNamed(t, f, "Combustível").ID
	due := budget.MonthOf(time.Now()).Next()
	if err := f.svc.SetTarget(ctx, userA, ipva, 240000, due); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}
	if got := targetsOf(t, f, userA)[ipva]; got.Amount != 240000 || got.Due != due {
		t.Fatalf("target = %+v, want 240000 due %s", got, due)
	}
	this := budget.MonthOf(time.Now())
	if err := f.svc.SetTarget(ctx, userA, ipva, 120000, this); err != nil {
		t.Fatalf("SetTarget replace (current month): %v", err)
	}
	if ts := targetsOf(t, f, userA); len(ts) != 1 || ts[ipva].Amount != 120000 || ts[ipva].Due != this {
		t.Fatalf("after replace = %+v, want one target 120000 due %s", ts, this)
	}
	for range 2 { // idempotent
		if err := f.svc.RemoveTarget(ctx, userA, ipva); err != nil {
			t.Fatalf("RemoveTarget: %v", err)
		}
	}
	if ts := targetsOf(t, f, userA); len(ts) != 0 {
		t.Errorf("after remove = %+v, want none", ts)
	}
}

func TestTarget_RejectsInvalid(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	expense := categoryNamed(t, f, "Combustível").ID
	income := categoryNamed(t, f, "Salário").ID
	next := budget.MonthOf(time.Now()).Next()
	for name, tc := range map[string]struct {
		id     string
		amount int64
		due    budget.Month
		want   error
	}{
		"zero amount":     {expense, 0, next, budget.ErrInvalidAmount},
		"negative amount": {expense, -1, next, budget.ErrInvalidAmount},
		"past due month":  {expense, 100, budget.MonthOf(time.Now()).Prev(), budget.ErrTargetPast},
		"income category": {income, 100, next, budget.ErrTargetIncome},
		"unknown":         {"0196a000-0000-7000-8000-0000000000ff", 100, next, budget.ErrNotFound},
	} {
		if err := f.svc.SetTarget(ctx, userA, tc.id, tc.amount, tc.due); !errors.Is(err, tc.want) {
			t.Errorf("%s: SetTarget = %v, want %v", name, err, tc.want)
		}
	}
	if ts := targetsOf(t, f, userA); len(ts) != 0 {
		t.Errorf("rejected targets were stored: %+v", ts)
	}
}

func TestTarget_IsolatedPerUser(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	id := categoryNamed(t, f, "Combustível").ID // user A's
	next := budget.MonthOf(time.Now()).Next()
	if err := f.svc.SetTarget(ctx, userB, id, 100, next); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B on user A's category = %v, want ErrNotFound", err)
	}
	if err := f.svc.SetTarget(ctx, userA, id, 100, next); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RemoveTarget(ctx, userB, id); err != nil {
		t.Fatal(err)
	}
	if ts := targetsOf(t, f, userB); len(ts) != 0 {
		t.Errorf("user B sees %+v", ts)
	}
	if ts := targetsOf(t, f, userA); len(ts) != 1 {
		t.Errorf("user B removed user A's target: %+v", ts)
	}
}

func TestTarget_ArchiveHidesMergeDropsSource(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	hobbies := categoryNamed(t, f, "Hobbies").ID
	streaming := categoryNamed(t, f, "Streaming").ID
	outras := categoryNamed(t, f, "Outras assinaturas").ID
	next := budget.MonthOf(time.Now()).Next()
	for _, id := range []string{hobbies, streaming, outras} {
		if err := f.svc.SetTarget(ctx, userA, id, 100, next); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.ArchiveCategory(ctx, userA, hobbies); err != nil {
		t.Fatal(err)
	}
	if _, ok := targetsOf(t, f, userA)[hobbies]; ok {
		t.Error("archived category's target is listed")
	}
	if err := f.svc.RestoreCategory(ctx, userA, hobbies); err != nil {
		t.Fatal(err)
	}
	if _, ok := targetsOf(t, f, userA)[hobbies]; !ok {
		t.Error("restoring the category did not bring its target back")
	}
	if err := f.svc.MergeCategory(ctx, userA, streaming, outras); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM budget_target WHERE category_id = ?`, streaming); n != 0 {
		t.Errorf("merged source keeps %d targets", n)
	}
	if _, ok := targetsOf(t, f, userA)[outras]; !ok {
		t.Error("merge target lost its own target")
	}
}

func TestTarget_ExportedAndDeleted(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	id := categoryNamed(t, f, "Combustível").ID
	next := budget.MonthOf(time.Now()).Next()
	if err := f.svc.SetTarget(ctx, userA, id, 240000, next); err != nil {
		t.Fatal(err)
	}
	ex, err := f.svc.ExportUserData(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Targets) != 1 || ex.Targets[0].CategoryID != id || ex.Targets[0].AmountCentavos != 240000 || ex.Targets[0].DueMonth != next.String() {
		t.Errorf("export targets = %+v", ex.Targets)
	}
	if err := f.svc.DeleteUserData(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM budget_target WHERE user_id = ?`, userA); n != 0 {
		t.Errorf("targets left after delete: %d", n)
	}
}
