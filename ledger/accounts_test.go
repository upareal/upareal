// story: e04s01
package ledger_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/ledger"
)

const userB = "0196a000-0000-7000-8000-00000000000b"

func TestAccounts_ListsSystemAccountsDecryptedInTypeOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	accounts, err := f.svc.Accounts(ctx, userA)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	want := []ledger.Account{
		{Type: "income", Name: "A categorizar", SystemRole: "uncategorized_income"},
		{Type: "expense", Name: "A categorizar", SystemRole: "uncategorized_expense"},
		{Type: "equity", Name: "Saldo inicial", SystemRole: "opening_balance"},
	}
	if len(accounts) != len(want) {
		t.Fatalf("accounts = %+v, want %d", accounts, len(want))
	}
	for i, w := range want {
		got := accounts[i]
		if got.ID == "" || got.Type != w.Type || got.Name != w.Name || got.SystemRole != w.SystemRole {
			t.Errorf("account %d = %+v, want %+v (order: asset, liability, income, expense, equity)", i, got, w)
		}
	}
}

func TestAccounts_EmptyForUnknownUser(t *testing.T) {
	f := newFixture(t)
	accounts, err := f.svc.Accounts(context.Background(), userB)
	if err != nil || len(accounts) != 0 {
		t.Errorf("Accounts for a user without data = %v, %v; want empty, nil", accounts, err)
	}
}

func TestIsolation_UsersSeeOnlyTheirOwnAccounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, u := range []string{userA, userB} {
		if err := f.svc.EnsureDefaults(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := f.svc.Accounts(ctx, userA)
	b, err := f.svc.Accounts(ctx, userB)
	if err != nil || len(b) != wantSystem {
		t.Fatalf("user B accounts = %v, %v; want %d", b, err, wantSystem)
	}
	ids := map[string]bool{}
	for _, acc := range a {
		ids[acc.ID] = true
	}
	for _, acc := range b {
		if ids[acc.ID] {
			t.Errorf("user B sees user A's account %s", acc.ID)
		}
	}
}
