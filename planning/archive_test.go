// story: e08s06
package planning_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/planning"
)

func TestArchive_OnlyEmptyExpenseEnvelopes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	planning.SetClock(f.svc, func() time.Time { return jan10 })
	mb := newMonthBook(t, f) // Combustível R$ 300,00 in January; Posto Shell mapped
	if err := f.svc.ArchiveCategory(ctx, userA, mb.fuel); !errors.Is(err, planning.ErrEnvelopeNotEmpty) {
		t.Fatalf("archive with Disponível R$ 300,00 = %v, want ErrEnvelopeNotEmpty", err)
	}
	f.post(t, mb.itau.ID, mb.shell.ID, 30000, jan10)
	before := month(t, f, mb).ToBeAssigned
	if err := f.svc.ArchiveCategory(ctx, userA, mb.fuel); err != nil {
		t.Fatalf("archive an empty envelope: %v", err)
	}
	if after := month(t, f, mb).ToBeAssigned; after != before {
		t.Errorf("A distribuir changed from %d to %d on archive", before, after)
	}
	ms, _ := f.svc.Mappings(ctx, userA)
	if m := byName(ms)["Posto Shell"]; m.CategoryID != "" {
		t.Errorf("Posto Shell still mapped to %q after archive; want Sem categoria", m.CategoryName)
	}
	if err := f.svc.ArchiveCategory(ctx, userA, mb.salaryCat); err != nil {
		t.Errorf("archive an income category: %v, want allowed", err)
	}
	if err := f.svc.ArchiveCategory(ctx, userB, f.category(t, userA, "Mercado")); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B archiving user A's category = %v, want budget.ErrNotFound", err)
	}
}

func TestMerge_MovesAccountsAndAmountsAndKeepsEveryMonth(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f)
	parking := f.category(t, userA, "Estacionamento")
	estapar := f.account(t, userA, "Estapar", "expense")
	if err := f.svc.SetMapping(ctx, userA, estapar.ID, parking); err != nil {
		t.Fatal(err)
	}
	if err := f.bud.SetAmount(ctx, userA, parking, mb.jan, 10000); err != nil {
		t.Fatal(err)
	}
	f.post(t, mb.itau.ID, mb.salario.ID, 800000, jan10)
	f.post(t, mb.itau.ID, mb.shell.ID, 25000, jan10)
	f.post(t, mb.itau.ID, estapar.ID, 4000, jan10)
	f.post(t, mb.itau.ID, estapar.ID, 2000, jan10.AddDate(0, 1, 0))
	months := []budget.Month{mb.jan, mustMonth(t, "2026-02"), mustMonth(t, "2026-03")}
	tba := map[budget.Month]int64{}
	sum := map[budget.Month]int64{}
	for _, m := range months {
		p, _ := f.svc.Month(ctx, userA, m)
		tba[m], sum[m] = p.ToBeAssigned, p.Categories[mb.fuel].Available+p.Categories[parking].Available
	}

	if err := f.svc.MergeCategory(ctx, userA, parking, mb.fuel); err != nil {
		t.Fatalf("MergeCategory: %v", err)
	}
	for _, m := range months {
		p, _ := f.svc.Month(ctx, userA, m)
		if p.ToBeAssigned != tba[m] || p.Categories[mb.fuel].Available != sum[m] {
			t.Errorf("%s: A distribuir %d (was %d), Combustível %d (want the sum %d)", m, p.ToBeAssigned, tba[m], p.Categories[mb.fuel].Available, sum[m])
		}
		if _, ok := p.Categories[parking]; ok {
			t.Errorf("%s: merged category still in the plan", m)
		}
	}
	ms, _ := f.svc.Mappings(ctx, userA)
	if byName(ms)["Estapar"].CategoryID != mb.fuel {
		t.Error("Estapar does not map to Combustível after the merge")
	}
	if err := f.svc.MergeCategory(ctx, userA, mb.fuel, mb.salaryCat); !errors.Is(err, budget.ErrKindMismatch) {
		t.Errorf("merge across kinds = %v, want budget.ErrKindMismatch", err)
	}
	if byName(ms)["Posto Shell"].CategoryID != mb.fuel {
		t.Error("a rejected merge moved mappings")
	}
}
