// story: e04s02
package ledger_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger"
)

var jan10 = time.Date(2026, 1, 10, 15, 0, 0, 0, time.UTC)

// twoAccounts seeds userA and returns the IDs of its Saldo inicial and A categorizar (expense).
func twoAccounts(t *testing.T, f fixture, userID string) (equity, expense string) {
	t.Helper()
	if err := f.svc.EnsureDefaults(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	accounts, err := f.svc.Accounts(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range accounts {
		switch a.SystemRole {
		case "opening_balance":
			equity = a.ID
		case "uncategorized_expense":
			expense = a.ID
		}
	}
	return equity, expense
}

func (f fixture) txCount(t *testing.T) int {
	t.Helper()
	return f.count(t, `SELECT count(*) FROM ledger_transaction`) + f.count(t, `SELECT count(*) FROM ledger_leg`)
}

func TestTransactionInvariants_BalancedTransactionIsSaved(t *testing.T) {
	f := newFixture(t)
	eq, ex := twoAccounts(t, f, userA)
	id, err := ledger.PostTransaction(context.Background(), f.svc, userA, jan10, "Teste", []ledger.Leg{
		{AccountID: ex, Amount: 1500}, {AccountID: eq, Amount: -1000}, {AccountID: eq, Amount: -500},
	})
	if err != nil || id == "" {
		t.Fatalf("PostTransaction balanced = %q, %v; want saved", id, err)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_leg WHERE transaction_id = ?`, id); n != 3 {
		t.Errorf("legs saved = %d, want 3", n)
	}
}

func TestTransactionInvariants_RejectsInvalidAtomically(t *testing.T) {
	f := newFixture(t)
	eq, ex := twoAccounts(t, f, userA)
	otherEq, _ := twoAccounts(t, f, userB)
	if _, err := f.conn.Exec(`INSERT INTO ledger_account (id, user_id, type, name, position, created_at, archived_at)
		VALUES ('archived-acc', ?, 'asset', x'00', 0, '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z')`, userA); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		legs []ledger.Leg
		want error
	}{
		"L1 unbalanced":       {[]ledger.Leg{{ex, 100}, {eq, -99}}, ledger.ErrUnbalanced},
		"L1 single leg":       {[]ledger.Leg{{ex, 100}}, ledger.ErrTooFewLegs},
		"L1 no legs":          {nil, ledger.ErrTooFewLegs},
		"L1 overflow":         {[]ledger.Leg{{ex, math.MaxInt64}, {ex, math.MaxInt64}, {eq, math.MinInt64}, {eq, math.MinInt64}}, ledger.ErrUnbalanced},
		"L2 zero leg":         {[]ledger.Leg{{ex, 100}, {eq, -100}, {eq, 0}}, ledger.ErrZeroLeg},
		"L3 other user's leg": {[]ledger.Leg{{ex, 100}, {otherEq, -100}}, ledger.ErrAccountNotPostable},
		"L3 unknown account":  {[]ledger.Leg{{ex, 100}, {"no-such-account", -100}}, ledger.ErrAccountNotPostable},
		"L6 archived account": {[]ledger.Leg{{ex, 100}, {"archived-acc", -100}}, ledger.ErrAccountNotPostable},
	} {
		t.Run(name, func(t *testing.T) {
			before := f.txCount(t)
			_, err := ledger.PostTransaction(context.Background(), f.svc, userA, jan10, "x", tc.legs)
			if !errors.Is(err, tc.want) {
				t.Errorf("PostTransaction = %v, want %v", err, tc.want)
			}
			if f.txCount(t) != before {
				t.Error("an invalid transaction left rows behind")
			}
		})
	}
}

func TestTransactionInvariants_SameAccountTwiceIsAllowed(t *testing.T) {
	f := newFixture(t)
	eq, ex := twoAccounts(t, f, userA)
	if _, err := ledger.PostTransaction(context.Background(), f.svc, userA, jan10, "x", []ledger.Leg{
		{AccountID: ex, Amount: 300}, {AccountID: ex, Amount: 200}, {AccountID: eq, Amount: -500},
	}); err != nil {
		t.Errorf("two legs on the same account: %v, want saved", err)
	}
}
