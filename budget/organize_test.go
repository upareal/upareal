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
