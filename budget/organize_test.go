// story: e05s02
package budget_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/upareal/upareal/budget"
)

func seeded(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	for _, u := range []string{userA, userB} {
		if err := f.svc.EnsureDefaults(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func groupsOf(t *testing.T, f fixture) []budget.Group {
	t.Helper()
	g, err := f.svc.Categories(context.Background(), userA)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func groupNamed(t *testing.T, f fixture, name string) budget.Group {
	t.Helper()
	for _, g := range groupsOf(t, f) {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("group %q not found", name)
	return budget.Group{}
}

func TestGroups_CreateRenameMove(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	pets, err := f.svc.CreateGroup(ctx, userA, "expense", "  Pets  ")
	if err != nil || pets.Name != "Pets" || pets.Kind != "expense" {
		t.Fatalf("CreateGroup = %+v, %v; want expense group Pets", pets, err)
	}
	gs := groupsOf(t, f)
	if gs[len(gs)-1].Name != "Pets" {
		t.Errorf("new group is not last: %s", gs[len(gs)-1].Name)
	}
	if err := f.svc.RenameGroup(ctx, userA, groupNamed(t, f, "Lazer").ID, "Diversão"); err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	_ = groupNamed(t, f, "Diversão")

	if err := f.svc.MoveGroup(ctx, userA, pets.ID, true); err != nil {
		t.Fatal(err)
	}
	gs = groupsOf(t, f)
	if gs[len(gs)-2].Name != "Pets" || gs[len(gs)-1].Name != "Receitas" {
		t.Errorf("after moving Pets up: ... %s, %s; want Pets, Receitas", gs[len(gs)-2].Name, gs[len(gs)-1].Name)
	}
	if err := f.svc.MoveGroup(ctx, userA, gs[0].ID, true); err != nil {
		t.Errorf("moving the first group up = %v, want no-op", err)
	}
	if groupsOf(t, f)[0].Name != "Moradia" {
		t.Error("first group changed after a no-op move")
	}
}

func TestGroups_RejectsInvalidInputAndForeignIDs(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	if _, err := f.svc.CreateGroup(ctx, userA, "expense", " "); !errors.Is(err, budget.ErrInvalidName) {
		t.Errorf("empty name = %v, want ErrInvalidName", err)
	}
	if _, err := f.svc.CreateGroup(ctx, userA, "expense", strings.Repeat("x", 101)); !errors.Is(err, budget.ErrInvalidName) {
		t.Errorf("101 characters = %v, want ErrInvalidName", err)
	}
	if _, err := f.svc.CreateGroup(ctx, userA, "transfer", "X"); !errors.Is(err, budget.ErrInvalidKind) {
		t.Errorf("unknown kind = %v, want ErrInvalidKind", err)
	}
	moradiaA := groupNamed(t, f, "Moradia").ID
	if err := f.svc.RenameGroup(ctx, userB, moradiaA, "Roubada"); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B renaming user A's group = %v, want ErrNotFound", err)
	}
	if err := f.svc.MoveGroup(ctx, userB, moradiaA, false); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B moving user A's group = %v, want ErrNotFound", err)
	}
	if groupNamed(t, f, "Moradia").ID != moradiaA {
		t.Error("user A's group changed")
	}
}

func categoryNamed(t *testing.T, f fixture, name string) budget.Category {
	t.Helper()
	for _, g := range groupsOf(t, f) {
		for _, c := range g.Categories {
			if c.Name == name {
				return c
			}
		}
	}
	t.Fatalf("category %q not found", name)
	return budget.Category{}
}

func names(cs []budget.Category) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestCategoryCRUD_CreateRenameAndChangeGroup(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	pets, _ := f.svc.CreateGroup(ctx, userA, "expense", "Pets")
	racao, err := f.svc.CreateCategory(ctx, userA, pets.ID, " Ração ")
	if err != nil || racao.Name != "Ração" {
		t.Fatalf("CreateCategory = %+v, %v", racao, err)
	}
	if _, err := f.svc.CreateCategory(ctx, userA, pets.ID, "Veterinário"); err != nil {
		t.Fatal(err)
	}
	if got := names(groupNamed(t, f, "Pets").Categories); len(got) != 2 || got[0] != "Ração" || got[1] != "Veterinário" {
		t.Errorf("Pets = %v, want [Ração Veterinário]", got)
	}
	if err := f.svc.RenameCategory(ctx, userA, racao.ID, "Ração e petiscos"); err != nil {
		t.Fatal(err)
	}
	_ = categoryNamed(t, f, "Ração e petiscos")

	streaming := categoryNamed(t, f, "Streaming")
	if err := f.svc.SetCategoryGroup(ctx, userA, streaming.ID, groupNamed(t, f, "Lazer").ID); err != nil {
		t.Fatalf("SetCategoryGroup same kind: %v", err)
	}
	lazer := names(groupNamed(t, f, "Lazer").Categories)
	if lazer[len(lazer)-1] != "Streaming" {
		t.Errorf("Lazer = %v, want Streaming last", lazer)
	}
	for _, n := range names(groupNamed(t, f, "Contas e assinaturas").Categories) {
		if n == "Streaming" {
			t.Error("Streaming still in its old group")
		}
	}
}

func TestCategoryCRUD_KindsNeverMix(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	mercado := categoryNamed(t, f, "Mercado")
	if err := f.svc.SetCategoryGroup(ctx, userA, mercado.ID, groupNamed(t, f, "Receitas").ID); !errors.Is(err, budget.ErrKindMismatch) {
		t.Errorf("moving an expense category into Receitas = %v, want ErrKindMismatch (B2)", err)
	}
	if got := names(groupNamed(t, f, "Alimentação").Categories); got[0] != "Mercado" {
		t.Error("Mercado moved despite the kind mismatch")
	}
}

func TestCategoryCRUD_RejectsInvalidAndForeign(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	moradia := groupNamed(t, f, "Moradia")
	if _, err := f.svc.CreateCategory(ctx, userA, moradia.ID, ""); !errors.Is(err, budget.ErrInvalidName) {
		t.Errorf("empty name = %v, want ErrInvalidName", err)
	}
	if _, err := f.svc.CreateCategory(ctx, userB, moradia.ID, "Intrusa"); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B adding to user A's group = %v, want ErrNotFound", err)
	}
	mercado := categoryNamed(t, f, "Mercado")
	if err := f.svc.RenameCategory(ctx, userB, mercado.ID, "Roubada"); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B renaming user A's category = %v, want ErrNotFound", err)
	}
	if err := f.svc.SetCategoryGroup(ctx, userA, mercado.ID, "no-such-group"); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("unknown target group = %v, want ErrNotFound", err)
	}
	if err := f.svc.MoveCategory(ctx, userB, mercado.ID, true); !errors.Is(err, budget.ErrNotFound) {
		t.Errorf("user B moving user A's category = %v, want ErrNotFound", err)
	}
}

func TestReorder_CategoriesWithinGroup(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	condominio := categoryNamed(t, f, "Condomínio")
	if err := f.svc.MoveCategory(ctx, userA, condominio.ID, true); err != nil {
		t.Fatal(err)
	}
	got := names(groupNamed(t, f, "Moradia").Categories)
	if got[0] != "Condomínio" || got[1] != "Aluguel ou financiamento" {
		t.Errorf("Moradia after moving Condomínio up = %v", got[:2])
	}
	if err := f.svc.MoveCategory(ctx, userA, condominio.ID, true); err != nil {
		t.Errorf("moving the first category up = %v, want no-op", err)
	}
	last := categoryNamed(t, f, "Manutenção da casa")
	if err := f.svc.MoveCategory(ctx, userA, last.ID, false); err != nil {
		t.Errorf("moving the last category down = %v, want no-op", err)
	}
	if names(groupNamed(t, f, "Alimentação").Categories)[0] != "Mercado" {
		t.Error("moving within Moradia changed another group")
	}
}
