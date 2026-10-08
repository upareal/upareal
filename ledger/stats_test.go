// story: e13s01
package ledger_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func TestPublicStats_CountsAcrossUsers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := func(user, name, typ string) ledger.Account {
		t.Helper()
		if err := f.svc.EnsureDefaults(ctx, user); err != nil {
			t.Fatal(err)
		}
		a, err := f.svc.CreateAccount(ctx, user, ledger.NewAccount{Name: name, Type: typ, OnBudget: typ == "asset"})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	itau, shell := acc(userA, "Itaú", "asset"), acc(userA, "Posto Shell", "expense")
	contaB := acc(userB, "Conta B", "asset")
	acc(userB, "Mercado B", "expense")
	post := func() string {
		id, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: itau.ID, CounterID: shell.ID, Amount: 100, At: jan10, Description: "x"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	post()
	if err := f.svc.DeleteTransaction(ctx, userA, post()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Import(ctx, userB, contaB.ID, []importer.Line{{ImportID: "1", At: jan10, Description: "PIX", Amount: -500}}); err != nil {
		t.Fatal(err)
	}

	s, err := f.svc.PublicStats(ctx)
	if err != nil {
		t.Fatalf("PublicStats: %v", err)
	}
	dbtest.AssertNumbersOnly(t, s)
	// 4 user accounts (system accounts excluded); 2 live transactions (1 deleted excluded); 1 imported line.
	if s.Accounts != 4 || s.Transactions != 2 || s.ImportedLines != 1 {
		t.Errorf("stats = %+v; want accounts 4, transactions 2, imported lines 1", s)
	}
}
