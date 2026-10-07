// story: e04s02
package ledger_test

import (
	"context"
	"errors"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func newChecking(t *testing.T, f fixture, userID string, opening int64) ledger.Account {
	t.Helper()
	if err := f.svc.EnsureDefaults(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	acc, err := f.svc.CreateAccount(context.Background(), userID, ledger.NewAccount{Name: "Conta corrente", Type: "asset", OnBudget: true, Opening: opening, OpeningAt: jan10})
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

func TestBalance_IsSumOfLegs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := newChecking(t, f, userA, 250000)
	if b, err := f.svc.Balance(ctx, userA, acc.ID); err != nil || b != 250000 {
		t.Errorf("Balance = %d, %v; want 250000", b, err)
	}
	equity := f.systemID(t, userA, "opening_balance")
	if b, _ := f.svc.Balance(ctx, userA, equity); b != -250000 {
		t.Errorf("Saldo inicial balance = %d, want -250000", b)
	}
	empty, _ := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Carteira", Type: "asset"})
	if b, err := f.svc.Balance(ctx, userA, empty.ID); err != nil || b != 0 {
		t.Errorf("Balance of an account without legs = %d, %v; want 0", b, err)
	}
	all, err := f.svc.Balances(ctx, userA)
	if err != nil || all[acc.ID] != 250000 || all[equity] != -250000 || all[empty.ID] != 0 {
		t.Errorf("Balances = %v, %v", all, err)
	}
}

func TestBalance_AdjustPostsTheDifference(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := newChecking(t, f, userA, 250000)

	if err := f.svc.AdjustBalance(ctx, userA, acc.ID, 248000, jan10); err != nil {
		t.Fatalf("AdjustBalance: %v", err)
	}
	if b, _ := f.svc.Balance(ctx, userA, acc.ID); b != 248000 {
		t.Errorf("balance after adjust = %d, want 248000", b)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_leg WHERE account_id = ? AND amount = -2000`, acc.ID); n != 1 {
		t.Error("adjustment did not post exactly one -2000 leg")
	}
	before := f.count(t, `SELECT count(*) FROM ledger_transaction`)
	if err := f.svc.AdjustBalance(ctx, userA, acc.ID, 248000, jan10); err != nil {
		t.Fatal(err)
	}
	if f.count(t, `SELECT count(*) FROM ledger_transaction`) != before {
		t.Error("adjusting to the current balance created a transaction")
	}
}

func TestBalance_OnlyAssetsAndLiabilitiesCanBeAdjusted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_ = f.svc.EnsureDefaults(ctx, userA)
	income, _ := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Salário", Type: "income"})
	if err := f.svc.AdjustBalance(ctx, userA, income.ID, -100, jan10); !errors.Is(err, ledger.ErrInvalidAccount) {
		t.Errorf("AdjustBalance on income = %v, want ErrInvalidAccount", err)
	}
	if err := f.svc.AdjustBalance(ctx, userA, f.systemID(t, userA, "opening_balance"), 0, jan10); !errors.Is(err, ledger.ErrInvalidAccount) {
		t.Errorf("AdjustBalance on Saldo inicial = %v, want ErrInvalidAccount", err)
	}
}

func TestIsolation_BalancesAndAdjustmentsStayWithTheirUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	accA := newChecking(t, f, userA, 100000)
	_ = newChecking(t, f, userB, 5000)

	if _, err := f.svc.Balance(ctx, userB, accA.ID); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B reading user A's balance = %v, want ErrAccountNotFound", err)
	}
	if err := f.svc.AdjustBalance(ctx, userB, accA.ID, 0, jan10); !errors.Is(err, ledger.ErrAccountNotFound) {
		t.Errorf("user B adjusting user A's account = %v, want ErrAccountNotFound", err)
	}
	if b, _ := f.svc.Balance(ctx, userA, accA.ID); b != 100000 {
		t.Errorf("user A balance = %d after user B's attempts, want 100000", b)
	}
	all, _ := f.svc.Balances(ctx, userB)
	if _, leaked := all[accA.ID]; leaked {
		t.Error("user B's Balances include user A's account")
	}
}
