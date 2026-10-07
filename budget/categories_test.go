// story: e05s01
package budget_test

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestCategories_OrderedAndDecrypted(t *testing.T) {
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
			if c.ID == "" {
				t.Errorf("category without ID in %s", g.Name)
			}
			total++
		}
	}
	if total != wantCategories {
		t.Errorf("categories = %d, want %d", total, wantCategories)
	}
}

func TestCategories_EmptyForUnknownUser(t *testing.T) {
	f := newFixture(t)
	if groups, err := f.svc.Categories(context.Background(), userB); err != nil || len(groups) != 0 {
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
	if err != nil || len(b) != wantGroups {
		t.Fatalf("user B groups = %d, %v; want %d", len(b), err, wantGroups)
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

// TestBudgetHasNoPlaintext proves a copy of the database reveals no group or category names.
func TestBudgetHasNoPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if groups, err := f.svc.Categories(ctx, userA); err != nil || len(groups) == 0 || groups[0].Name != "Moradia" {
		t.Fatalf("round trip = %v, %v; the scan below would prove nothing", groups, err)
	}
	if err := f.conn.Close(); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	for _, p := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
		if b, err := os.ReadFile(p); err == nil {
			raw = append(raw, b...)
		}
	}
	if !bytes.Contains(raw, []byte("budget_group")) {
		t.Fatal("control: schema text not found; the scan is not reading the database")
	}
	for _, name := range []string{"Moradia", "Aluguel ou financiamento", "Mercado", "Receitas", "Salário"} {
		if bytes.Contains(raw, []byte(name)) {
			t.Errorf("database files contain %q in plaintext", name)
		}
	}
}
