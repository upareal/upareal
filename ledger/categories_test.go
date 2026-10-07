// story: e04s01
package ledger_test

import (
	"context"
	"testing"
)

const userB = "0196a000-0000-7000-8000-00000000000b"

func TestCategories_OrderedDecryptedWithoutSystemAccounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	groups, err := f.svc.Categories(ctx, userA)
	if err != nil {
		t.Fatalf("Categories: %v", err)
	}
	if len(groups) != wantGroups {
		t.Fatalf("groups = %d, want %d", len(groups), wantGroups)
	}
	first, last := groups[0], groups[len(groups)-1]
	if first.Name != "Moradia" || first.Kind != "expense" || len(first.Categories) != 7 || first.Categories[0].Name != "Aluguel ou financiamento" {
		t.Errorf("first group = %+v, want Moradia (expense) starting with Aluguel ou financiamento", first)
	}
	if last.Name != "Receitas" || last.Kind != "income" || last.Categories[0].Name != "Salário" {
		t.Errorf("last group = %+v, want Receitas (income) starting with Salário", last)
	}
	total := 0
	for _, g := range groups {
		for _, c := range g.Categories {
			total++
			if c.ID == "" || c.Name == "Saldo inicial" || c.Name == "A categorizar" {
				t.Errorf("unexpected category %+v (system accounts must be hidden)", c)
			}
		}
	}
	if total != wantCategories {
		t.Errorf("categories = %d, want %d", total, wantCategories)
	}
}

func TestCategories_EmptyForUnknownUser(t *testing.T) {
	f := newFixture(t)
	groups, err := f.svc.Categories(context.Background(), userB)
	if err != nil || len(groups) != 0 {
		t.Errorf("Categories for a user without data = %v, %v; want empty, nil", groups, err)
	}
}

func TestIsolation_UsersSeeOnlyTheirOwnCategories(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, u := range []string{userA, userB} {
		if err := f.svc.EnsureDefaults(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := f.svc.Categories(ctx, userA)
	b, err := f.svc.Categories(ctx, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != wantGroups {
		t.Fatalf("user B groups = %d, want %d", len(b), wantGroups)
	}
	ids := map[string]bool{}
	for _, g := range a {
		ids[g.ID] = true
		for _, c := range g.Categories {
			ids[c.ID] = true
		}
	}
	for _, g := range b {
		if ids[g.ID] {
			t.Errorf("user B sees user A's group %s", g.ID)
		}
		for _, c := range g.Categories {
			if ids[c.ID] {
				t.Errorf("user B sees user A's category %s", c.ID)
			}
		}
	}
}
