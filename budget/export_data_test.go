// story: e10s01
package budget_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
)

func TestExportUserData_EveryBudgetRowOfTheUser(t *testing.T) {
	f := seeded(t)
	ctx := context.Background()
	if err := f.svc.SetAmount(ctx, userA, categoryNamed(t, f, "Combustível").ID, month(t, "2026-01"), 30000); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveCategory(ctx, userA, categoryNamed(t, f, "Hobbies").ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateGroup(ctx, userB, "expense", "Segredo do B"); err != nil {
		t.Fatal(err)
	}
	ex, err := f.svc.ExportUserData(ctx, userA)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	covered := map[string]int{
		"budget_group": len(ex.Groups), "budget_category": len(ex.Categories),
		"budget_month": len(ex.Months), "budget_amount": len(ex.Amounts),
		"budget_target": len(ex.Targets),
		"budget_user_key": -1, // security material, not exported
	}
	for table, n := range dbtest.UserRows(t, f.conn, "budget_", userA) {
		want, ok := covered[table]
		switch {
		case !ok:
			t.Errorf("table %s is not covered by the export", table)
		case want >= 0 && want != n:
			t.Errorf("%s: exported %d of %d rows", table, want, n)
		}
	}
	raw, _ := json.Marshal(ex)
	doc := string(raw)
	for _, want := range []string{`"Combustível"`, `"Transporte"`, `"amount_centavos":30000`, `"month":"2026-01"`, `"archived_at":"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("export missing %s", want)
		}
	}
	if strings.Contains(doc, "Segredo do B") {
		t.Error("export leaks user B's data")
	}
}
