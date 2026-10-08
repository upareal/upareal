// story: e08s06
package budget_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/budget"
)

func isActive(t *testing.T, f fixture, id string) bool {
	t.Helper()
	for _, g := range groupsOf(t, f) {
		for _, c := range g.Categories {
			if c.ID == id {
				return true
			}
		}
	}
	return false
}

func TestArchiveCategory_ArchiveListAndRestore(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	hobbies := categoryNamed(t, f, "Hobbies")
	if err := f.svc.ArchiveCategory(ctx, userA, hobbies.ID); err != nil {
		t.Fatalf("ArchiveCategory: %v", err)
	}
	if isActive(t, f, hobbies.ID) {
		t.Error("archived category still listed as active")
	}
	archived, err := f.svc.ArchivedCategories(ctx, userA)
	if err != nil || len(archived) != 1 || archived[0].ID != hobbies.ID || archived[0].Name != "Hobbies" {
		t.Fatalf("ArchivedCategories = %+v, %v; want Pet", archived, err)
	}
	if err := f.svc.ArchiveCategory(ctx, userB, hobbies.ID); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B archiving user A's category = %v, want ErrNotFound", err)
	}
	if err := f.svc.RestoreCategory(ctx, userB, hobbies.ID); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B restoring = %v, want ErrNotFound", err)
	}
	if err := f.svc.RestoreCategory(ctx, userA, hobbies.ID); err != nil || !isActive(t, f, hobbies.ID) {
		t.Errorf("RestoreCategory = %v, active %v", err, isActive(t, f, hobbies.ID))
	}
	if archived, _ := f.svc.ArchivedCategories(ctx, userA); len(archived) != 0 {
		t.Errorf("after restore archived = %+v", archived)
	}
}

func TestMergeCategory_SumsAmountsPerMonthAndArchivesSource(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	streaming := categoryNamed(t, f, "Streaming").ID
	outras := categoryNamed(t, f, "Outras assinaturas").ID
	jan, feb := month(t, "2026-01"), month(t, "2026-02")
	set := func(id string, m budget.Month, v int64) {
		if err := f.svc.SetAmount(ctx, userA, id, m, v); err != nil {
			t.Fatal(err)
		}
	}
	set(streaming, jan, 5000)
	set(outras, jan, 3000)
	set(streaming, feb, 4000) // February opened by copying January, then changed

	if err := f.svc.MergeCategory(ctx, userA, streaming, outras); err != nil {
		t.Fatalf("MergeCategory: %v", err)
	}
	for m, want := range map[budget.Month]int64{jan: 8000, feb: 7000} {
		amounts, _ := f.svc.OpenMonth(ctx, userA, m)
		if amounts[outras] != want || amounts[streaming] != 0 {
			t.Errorf("%s: Outras assinaturas %d (want %d), Streaming %d (want 0)", m, amounts[outras], want, amounts[streaming])
		}
	}
	if isActive(t, f, streaming) {
		t.Error("source category is still active after the merge")
	}
	if n := f.count(t, `SELECT count(*) FROM budget_amount WHERE category_id = ?`, streaming); n != 0 {
		t.Errorf("source still has %d amount rows", n)
	}
}

func TestMergeCategory_RejectsInvalidPairs(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	streaming := categoryNamed(t, f, "Streaming").ID
	salary := categoryNamed(t, f, "Salário").ID
	hobbies := categoryNamed(t, f, "Hobbies").ID
	_ = f.svc.ArchiveCategory(ctx, userA, hobbies)
	var foreign string
	for _, g := range mustGroups(t, f, userB) {
		if len(g.Categories) > 0 {
			foreign = g.Categories[0].ID
			break
		}
	}
	for name, tc := range map[string]struct {
		src, dst string
		want     error
	}{
		"same":            {streaming, streaming, budget.ErrSameCategory},
		"across kinds":    {streaming, salary, budget.ErrKindMismatch},
		"archived target": {streaming, hobbies, budget.ErrNotFound},
		"foreign target":  {streaming, foreign, budget.ErrNotFound},
		"unknown source":  {"no-such", streaming, budget.ErrNotFound},
	} {
		if err := f.svc.MergeCategory(ctx, userA, tc.src, tc.dst); !errors.Is(err, tc.want) {
			t.Errorf("%s: MergeCategory = %v, want %v", name, err, tc.want)
		}
	}
	if !isActive(t, f, streaming) {
		t.Error("a rejected merge archived the source")
	}
}

func mustGroups(t *testing.T, f fixture, userID string) []budget.Group {
	t.Helper()
	gs, err := f.svc.Categories(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return gs
}
