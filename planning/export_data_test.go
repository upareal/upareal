// story: e10s01
package planning_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
	"github.com/upareal/upareal/ledger"
)

func TestExportUserData_EveryMappingOfTheUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mb := newMonthBook(t, f) // user A: Posto Shell and Salário mapped
	b, err := f.ledg.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Posto B", Type: "expense"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetMapping(ctx, userB, b.ID, f.category(t, userB, "Combustível")); err != nil {
		t.Fatal(err)
	}
	ex, err := f.svc.ExportUserData(ctx, userA)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	for table, n := range dbtest.UserRows(t, f.conn, "planning_", userA) {
		if table != "planning_mapping" {
			t.Errorf("table %s is not covered by the export", table)
		} else if len(ex.Mappings) != n {
			t.Errorf("exported %d of %d mappings", len(ex.Mappings), n)
		}
	}
	found := false
	for _, m := range ex.Mappings {
		found = found || (m.AccountID == mb.shell.ID && m.CategoryID == mb.fuel)
		if m.AccountID == b.ID {
			t.Error("export leaks user B's mapping")
		}
	}
	if !found {
		t.Errorf("mappings = %+v; want Posto Shell → Combustível", ex.Mappings)
	}
}
