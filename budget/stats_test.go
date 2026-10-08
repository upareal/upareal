// story: e13s01
package budget_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
)

func TestPublicStats_CategoriesAndPlannedThisMonth(t *testing.T) {
	f := seeded(t) // users A and B with default categories
	ctx := context.Background()
	jan := month(t, "2026-01")
	fuel := categoryNamed(t, f, "Combustível").ID // user A's
	if err := f.svc.SetAmount(ctx, userA, fuel, jan, 30000); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetAmount(ctx, userA, fuel, month(t, "2026-02"), 99900); err != nil {
		t.Fatal(err) // another month: not counted
	}
	for _, g := range mustGroups(t, f, userB) {
		for _, c := range g.Categories {
			if c.Name == "Combustível" {
				if err := f.svc.SetAmount(ctx, userB, c.ID, jan, 20000); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := f.svc.ArchiveCategory(ctx, userA, categoryNamed(t, f, "Hobbies").ID); err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, u := range []string{userA, userB} {
		for _, g := range mustGroups(t, f, u) {
			active += len(g.Categories)
		}
	}

	s, err := f.svc.PublicStats(ctx, jan)
	if err != nil {
		t.Fatalf("PublicStats: %v", err)
	}
	dbtest.AssertNumbersOnly(t, s)
	if s.Categories != int64(active) || s.PlannedCentavos != 50000 {
		t.Errorf("stats = %+v; want categories %d (archived excluded), planned 50000 (January only, both users)", s, active)
	}
}
