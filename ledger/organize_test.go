// story: e04s03
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func create(t *testing.T, f fixture, n ledger.NewAccount) ledger.Account {
	t.Helper()
	_ = f.svc.EnsureDefaults(context.Background(), userA)
	a, err := f.svc.CreateAccount(context.Background(), userA, n)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func accountByID(t *testing.T, f fixture, id string) ledger.Account {
	t.Helper()
	accounts, _ := f.svc.Accounts(context.Background(), userA)
	for _, a := range accounts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("account %s not in active list", id)
	return ledger.Account{}
}

func TestUpdateAccount_RenameAndFlags(t *testing.T) {
	f := newFixture(t)
	a := create(t, f, ledger.NewAccount{Name: "Conta", Type: "asset", OnBudget: true, Opening: 1000})
	got, err := f.svc.UpdateAccount(context.Background(), userA, a.ID, ledger.AccountUpdate{Name: " Itaú ", Type: "asset", OnBudget: false})
	if err != nil {
		t.Fatalf("UpdateAccount: %v", err)
	}
	if got.Name != "Itaú" || got.OnBudget {
		t.Errorf("UpdateAccount = %+v, want name Itaú, off-budget", got)
	}
	if listed := accountByID(t, f, a.ID); listed.Name != "Itaú" || listed.OnBudget {
		t.Errorf("listed = %+v, want the update persisted", listed)
	}
}

func TestUpdateAccount_TypeLockedOnceUsed(t *testing.T) {
	f := newFixture(t)
	used := create(t, f, ledger.NewAccount{Name: "Conta", Type: "asset", Opening: 1000})
	if _, err := f.svc.UpdateAccount(context.Background(), userA, used.ID, ledger.AccountUpdate{Name: "Conta", Type: "liability"}); !errors.Is(err, ledger.ErrTypeLocked) {
		t.Errorf("type change with legs = %v, want ErrTypeLocked (L5)", err)
	}
	if accountByID(t, f, used.ID).Type != "asset" {
		t.Error("type changed despite legs")
	}

	unused := create(t, f, ledger.NewAccount{Name: "Errada", Type: "asset"})
	got, err := f.svc.UpdateAccount(context.Background(), userA, unused.ID, ledger.AccountUpdate{Name: "Nubank", Type: "liability", IsCard: true})
	if err != nil || got.Type != "liability" || !got.IsCard || !got.OnBudget {
		t.Errorf("type change without legs = %+v, %v; want card liability, on-budget", got, err)
	}
}

func TestUpdateAccount_RejectsInvalidAndProtectsSystemAccounts(t *testing.T) {
	f := newFixture(t)
	a := create(t, f, ledger.NewAccount{Name: "Conta", Type: "asset"})
	for name, tc := range map[string]struct {
		id   string
		u    ledger.AccountUpdate
		want error
	}{
		"empty name":      {a.ID, ledger.AccountUpdate{Name: " ", Type: "asset"}, ledger.ErrInvalidAccount},
		"asset as card":   {a.ID, ledger.AccountUpdate{Name: "X", Type: "asset", IsCard: true}, ledger.ErrInvalidAccount},
		"to equity":       {a.ID, ledger.AccountUpdate{Name: "X", Type: "equity"}, ledger.ErrInvalidAccount},
		"system account":  {f.systemID(t, userA, "opening_balance"), ledger.AccountUpdate{Name: "X", Type: "equity"}, ledger.ErrSystemAccount},
		"unknown account": {"no-such-id", ledger.AccountUpdate{Name: "X", Type: "asset"}, ledger.ErrAccountNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.svc.UpdateAccount(context.Background(), userA, tc.id, tc.u); !errors.Is(err, tc.want) {
				t.Errorf("UpdateAccount = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestIsolation_CannotUpdateAnotherUsersAccount(t *testing.T) {
	f := newFixture(t)
	a := create(t, f, ledger.NewAccount{Name: "Conta", Type: "asset"})
	_ = f.svc.EnsureDefaults(context.Background(), userB)
	if _, err := f.svc.UpdateAccount(context.Background(), userB, a.ID, ledger.AccountUpdate{Name: "Roubada", Type: "asset"}); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B updating user A's account = %v, want ErrAccountNotFound", err)
	}
	if accountByID(t, f, a.ID).Name != "Conta" {
		t.Error("user A's account was renamed by user B")
	}
}

func expenseNames(t *testing.T, f fixture) []string {
	t.Helper()
	accounts, _ := f.svc.Accounts(context.Background(), userA)
	var names []string
	for _, a := range accounts {
		if a.Type == "expense" && a.SystemRole == "" {
			names = append(names, a.Name)
		}
	}
	return names
}

func TestMoveAccount_SwapsWithNeighbourWithinType(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := create(t, f, ledger.NewAccount{Name: "A", Type: "expense"})
	_ = create(t, f, ledger.NewAccount{Name: "B", Type: "expense"})
	c := create(t, f, ledger.NewAccount{Name: "C", Type: "expense"})
	_ = create(t, f, ledger.NewAccount{Name: "Conta", Type: "asset"}) // other type, must not interfere

	if err := f.svc.MoveAccount(ctx, userA, c.ID, true); err != nil {
		t.Fatalf("MoveAccount up: %v", err)
	}
	if got := expenseNames(t, f); len(got) != 3 || got[0] != "A" || got[1] != "C" || got[2] != "B" {
		t.Errorf("after moving C up = %v, want [A C B]", got)
	}
	if err := f.svc.MoveAccount(ctx, userA, a.ID, true); err != nil {
		t.Errorf("moving the first account up: %v, want no-op", err)
	}
	if err := f.svc.MoveAccount(ctx, userA, c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MoveAccount(ctx, userA, c.ID, false); err != nil {
		t.Errorf("moving the last account down: %v, want no-op", err)
	}
	if got := expenseNames(t, f); got[0] != "A" || got[1] != "B" || got[2] != "C" {
		t.Errorf("after moving C back down = %v, want [A B C]", got)
	}
}

func TestMoveAccount_TypeChangeGoesToEndOfNewType(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_ = create(t, f, ledger.NewAccount{Name: "A", Type: "expense"})
	x := create(t, f, ledger.NewAccount{Name: "X", Type: "income"})
	if _, err := f.svc.UpdateAccount(ctx, userA, x.ID, ledger.AccountUpdate{Name: "X", Type: "expense"}); err != nil {
		t.Fatal(err)
	}
	if got := expenseNames(t, f); len(got) != 2 || got[1] != "X" {
		t.Errorf("expenses after type change = %v, want X last", got)
	}
	if err := f.svc.MoveAccount(ctx, userA, x.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := expenseNames(t, f); got[0] != "X" {
		t.Errorf("after moving X up = %v, want X first", got)
	}
}

func TestMoveAccount_GuardsSystemAndForeignAccounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := create(t, f, ledger.NewAccount{Name: "A", Type: "expense"})
	if err := f.svc.MoveAccount(ctx, userA, f.systemID(t, userA, "uncategorized_expense"), true); !errors.Is(err, ledger.ErrSystemAccount) {
		t.Errorf("moving a system account = %v, want ErrSystemAccount", err)
	}
	if err := f.svc.MoveAccount(ctx, userB, a.ID, true); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B moving user A's account = %v, want ErrAccountNotFound", err)
	}
}
