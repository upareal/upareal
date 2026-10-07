// story: e08s01
package planning_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/db"
	"github.com/upareal/upareal/internal/vault"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
)

const (
	userA = "0196a000-0000-7000-8000-00000000000a"
	userB = "0196a000-0000-7000-8000-00000000000b"
)

type fixture struct {
	conn *sql.DB
	ledg *ledger.Service
	bud  *budget.Service
	svc  *planning.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(filepath.Join(t.TempDir(), "planning.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	for _, m := range []struct {
		name string
		fsys fs.FS
	}{{"ledger", ledger.Migrations}, {"budget", budget.Migrations}, {"planning", planning.Migrations}} {
		if err := db.Migrate(ctx, conn, m.name, m.fsys); err != nil {
			t.Fatal(err)
		}
	}
	master := make([]byte, 32)
	master[0] = 8
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{conn: conn, ledg: ledger.NewService(conn, kr), bud: budget.NewService(conn, kr)}
	f.svc = planning.NewService(conn, f.ledg, f.bud)
	for _, u := range []string{userA, userB} {
		if err := f.ledg.EnsureDefaults(ctx, u); err != nil {
			t.Fatal(err)
		}
		if err := f.bud.EnsureDefaults(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f fixture) account(t *testing.T, userID, name, typ string) ledger.Account {
	t.Helper()
	a, err := f.ledg.CreateAccount(context.Background(), userID, ledger.NewAccount{Name: name, Type: typ})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func (f fixture) category(t *testing.T, userID, name string) string {
	t.Helper()
	groups, err := f.bud.Categories(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		for _, c := range g.Categories {
			if c.Name == name {
				return c.ID
			}
		}
	}
	t.Fatalf("category %q not found", name)
	return ""
}

func (f fixture) mappedTo(t *testing.T, accountID string) string {
	t.Helper()
	var c string
	err := f.conn.QueryRow(`SELECT category_id FROM planning_mapping WHERE account_id = ?`, accountID).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMapping_ManyAccountsOneCategory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	shell := f.account(t, userA, "Posto Shell", "expense")
	ipiranga := f.account(t, userA, "Posto Ipiranga", "expense")
	salario := f.account(t, userA, "Salário", "income")
	fuel := f.category(t, userA, "Combustível")

	for _, acc := range []string{shell.ID, ipiranga.ID} {
		if err := f.svc.SetMapping(ctx, userA, acc, fuel); err != nil {
			t.Fatalf("SetMapping: %v", err)
		}
	}
	if err := f.svc.SetMapping(ctx, userA, salario.ID, f.category(t, userA, "Salário")); err != nil {
		t.Fatal(err)
	}
	if f.mappedTo(t, shell.ID) != fuel || f.mappedTo(t, ipiranga.ID) != fuel {
		t.Error("both postos must map to Combustível")
	}
	// P1: remapping replaces; "" clears.
	if err := f.svc.SetMapping(ctx, userA, shell.ID, f.category(t, userA, "Estacionamento")); err != nil {
		t.Fatal(err)
	}
	if f.mappedTo(t, shell.ID) == fuel {
		t.Error("remapping did not replace the category (P1: one per account)")
	}
	if err := f.svc.SetMapping(ctx, userA, shell.ID, ""); err != nil || f.mappedTo(t, shell.ID) != "" {
		t.Errorf("clearing the mapping = %v, still mapped to %q", err, f.mappedTo(t, shell.ID))
	}
}

func TestMapping_RejectsMismatchAndUnmappable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	salario := f.account(t, userA, "Salário", "income")
	itau := f.account(t, userA, "Itaú", "asset")
	shell := f.account(t, userA, "Posto Shell", "expense")
	mercado := f.category(t, userA, "Mercado")
	var uncategorized string
	accounts, _ := f.ledg.Accounts(ctx, userA)
	for _, a := range accounts {
		if a.SystemRole == "uncategorized_expense" {
			uncategorized = a.ID
		}
	}
	for name, tc := range map[string]struct {
		account, category string
		want              error
	}{
		"income to expense category (P2)": {salario.ID, mercado, planning.ErrKindMismatch},
		"asset account":                   {itau.ID, mercado, planning.ErrNotMappable},
		"system A categorizar":            {uncategorized, mercado, planning.ErrNotMappable},
		"unknown category":                {shell.ID, "no-such", planning.ErrNotFound},
		"unknown account":                 {"no-such", mercado, planning.ErrNotFound},
		"another user's category":         {shell.ID, f.category(t, userB, "Mercado"), planning.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			if err := f.svc.SetMapping(ctx, userA, tc.account, tc.category); !errors.Is(err, tc.want) {
				t.Errorf("SetMapping = %v, want %v", err, tc.want)
			}
		})
	}
	if err := f.svc.SetMapping(ctx, userB, shell.ID, f.category(t, userB, "Mercado")); !errors.Is(err, planning.ErrNotFound) {
		t.Errorf("user B mapping user A's account = %v, want ErrNotFound", err)
	}
}

func byName(ms []planning.Mapping) map[string]planning.Mapping {
	out := map[string]planning.Mapping{}
	for _, m := range ms {
		out[m.Account.Name] = m
	}
	return out
}

func TestUnmapped_EveryAccountListedWithItsCategoryOrNone(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	shell := f.account(t, userA, "Posto Shell", "expense")
	f.account(t, userA, "Padaria", "expense")
	f.account(t, userA, "Itaú", "asset") // not mappable: not listed
	if err := f.svc.SetMapping(ctx, userA, shell.ID, f.category(t, userA, "Combustível")); err != nil {
		t.Fatal(err)
	}
	ms, err := f.svc.Mappings(ctx, userA)
	if err != nil {
		t.Fatalf("Mappings: %v", err)
	}
	got := byName(ms)
	if len(ms) != 2 {
		t.Fatalf("mappings = %+v, want Posto Shell and Padaria only (no system or asset accounts)", ms)
	}
	if m := got["Posto Shell"]; m.CategoryName != "Combustível" || m.GroupName != "Transporte" {
		t.Errorf("Posto Shell = %+v, want Combustível (Transporte)", m)
	}
	if m := got["Padaria"]; m.CategoryID != "" {
		t.Errorf("Padaria = %+v, want Sem categoria (empty)", m)
	}
}

func TestUnmapped_StaleCategoryShowsSemCategoria(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	shell := f.account(t, userA, "Posto Shell", "expense")
	fuel := f.category(t, userA, "Combustível")
	if err := f.svc.SetMapping(ctx, userA, shell.ID, fuel); err != nil {
		t.Fatal(err)
	}
	// Archiving categories arrives in e08s06; simulate it in the budget's table.
	if _, err := f.conn.Exec(`UPDATE budget_category SET archived_at = '2026-01-01T00:00:00Z' WHERE id = ?`, fuel); err != nil {
		t.Fatal(err)
	}
	ms, _ := f.svc.Mappings(ctx, userA)
	if m := byName(ms)["Posto Shell"]; m.CategoryID != "" {
		t.Errorf("Posto Shell with an archived category = %+v, want Sem categoria (X1)", m)
	}
}

func TestUnmapped_IsolatedPerUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.account(t, userA, "Posto Shell", "expense")
	if ms, err := f.svc.Mappings(ctx, userB); err != nil || len(ms) != 0 {
		t.Errorf("user B mappings = %+v, %v; want none", ms, err)
	}
}
