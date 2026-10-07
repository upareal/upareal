// story: e04s02
package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/upareal/upareal/ledger"
)

func (f fixture) legSum(t *testing.T, accountID string) int {
	t.Helper()
	return f.count(t, `SELECT COALESCE(SUM(amount), 0) FROM ledger_leg WHERE account_id = ?`, accountID)
}

func (f fixture) systemID(t *testing.T, userID, role string) string {
	t.Helper()
	var id string
	if err := f.conn.QueryRow(`SELECT id FROM ledger_account WHERE user_id = ? AND system_role = ?`, userID, role).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateAccount_EveryUserTypeWithFlags(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	for _, n := range []ledger.NewAccount{
		{Name: "Conta corrente", Type: "asset", OnBudget: true},
		{Name: "Tesouro Direto", Type: "asset", OnBudget: false},
		{Name: "Nubank", Type: "liability", IsCard: true, OnBudget: false}, // cards are forced on-budget
		{Name: "Financiamento", Type: "liability", OnBudget: false},
		{Name: "Salário", Type: "income", OnBudget: true}, // ignored for income/expense
		{Name: "Posto Shell", Type: "expense"},
	} {
		if _, err := f.svc.CreateAccount(ctx, userA, n); err != nil {
			t.Fatalf("CreateAccount(%+v): %v", n, err)
		}
	}
	got := map[string]ledger.Account{}
	accounts, _ := f.svc.Accounts(ctx, userA)
	for _, a := range accounts {
		got[a.Name] = a
	}
	for name, want := range map[string]struct {
		typ              string
		onBudget, isCard bool
	}{
		"Conta corrente": {"asset", true, false},
		"Tesouro Direto": {"asset", false, false},
		"Nubank":         {"liability", true, true},
		"Financiamento":  {"liability", false, false},
		"Salário":        {"income", false, false},
		"Posto Shell":    {"expense", false, false},
	} {
		a, ok := got[name]
		if !ok || a.Type != want.typ || a.OnBudget != want.onBudget || a.IsCard != want.isCard {
			t.Errorf("%s = %+v (found %v), want type %s on_budget %v is_card %v", name, a, ok, want.typ, want.onBudget, want.isCard)
		}
	}
	if accounts[0].Name != "Conta corrente" || accounts[1].Name != "Tesouro Direto" {
		t.Errorf("asset order = %s, %s; want creation order within type", accounts[0].Name, accounts[1].Name)
	}
}

func TestCreateAccount_OpeningBalancePostsAgainstSaldoInicial(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_ = f.svc.EnsureDefaults(ctx, userA)
	equity := f.systemID(t, userA, "opening_balance")

	checking, err := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Conta corrente", Type: "asset", OnBudget: true, Opening: 250000, OpeningAt: jan10})
	if err != nil {
		t.Fatal(err)
	}
	card, err := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Nubank", Type: "liability", IsCard: true, Opening: -80000, OpeningAt: jan10})
	if err != nil {
		t.Fatal(err)
	}
	if f.legSum(t, checking.ID) != 250000 || f.legSum(t, card.ID) != -80000 || f.legSum(t, equity) != -170000 {
		t.Errorf("legs: checking %d, card %d, Saldo inicial %d; want 250000, -80000, -170000",
			f.legSum(t, checking.ID), f.legSum(t, card.ID), f.legSum(t, equity))
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_transaction WHERE occurred_at = ?`, "2026-01-10T15:00:00Z"); n != 2 {
		t.Errorf("opening transactions on the chosen date = %d, want 2", n)
	}

	if _, err := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Carteira", Type: "asset"}); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_transaction`); n != 2 {
		t.Errorf("transactions after a zero opening balance = %d, want still 2", n)
	}
}

func TestCreateAccount_RejectsInvalidInputAndCreatesNothing(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_ = f.svc.EnsureDefaults(ctx, userA)
	for name, tc := range map[string]struct {
		n    ledger.NewAccount
		want error
	}{
		"empty name":                   {ledger.NewAccount{Name: "  ", Type: "asset"}, ledger.ErrInvalidAccount},
		"name too long":                {ledger.NewAccount{Name: strings.Repeat("x", 101), Type: "asset"}, ledger.ErrInvalidAccount},
		"equity is system only":        {ledger.NewAccount{Name: "Capital", Type: "equity"}, ledger.ErrInvalidAccount},
		"unknown type":                 {ledger.NewAccount{Name: "X", Type: "savings"}, ledger.ErrInvalidAccount},
		"card that is not a liability": {ledger.NewAccount{Name: "X", Type: "asset", IsCard: true}, ledger.ErrInvalidAccount},
		"opening on income":            {ledger.NewAccount{Name: "Salário", Type: "income", Opening: 100}, ledger.ErrInvalidAccount},
		"opening on expense":           {ledger.NewAccount{Name: "Mercado", Type: "expense", Opening: -100}, ledger.ErrInvalidAccount},
	} {
		t.Run(name, func(t *testing.T) {
			before := f.count(t, `SELECT count(*) FROM ledger_account`)
			if _, err := f.svc.CreateAccount(ctx, userA, tc.n); !errors.Is(err, tc.want) {
				t.Errorf("CreateAccount = %v, want %v", err, tc.want)
			}
			if f.count(t, `SELECT count(*) FROM ledger_account`) != before {
				t.Error("an invalid account was created")
			}
		})
	}
}

func TestCreateAccount_TrimsName(t *testing.T) {
	f := newFixture(t)
	_ = f.svc.EnsureDefaults(context.Background(), userA)
	a, err := f.svc.CreateAccount(context.Background(), userA, ledger.NewAccount{Name: "  Conta  ", Type: "asset"})
	if err != nil || a.Name != "Conta" {
		t.Errorf("CreateAccount name = %q, %v; want trimmed \"Conta\"", a.Name, err)
	}
}
