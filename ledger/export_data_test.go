// story: e10s01
package ledger_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/dbtest"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func TestExportUserData_EverythingOfTheUserReadable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	id := expense(t, f, b, 1500, "Gasolina velha")
	if err := f.svc.EditEntry(ctx, userA, id, ledger.SplitEntry{AccountID: b.itau.ID, Total: 1600, At: jan10, Description: "Gasolina nova", Parts: []ledger.Part{{CounterID: b.shell.ID, Amount: 1600}}}); err != nil {
		t.Fatal(err)
	}
	gone := expense(t, f, b, 700, "Apagado")
	if err := f.svc.DeleteTransaction(ctx, userA, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Import(ctx, userA, b.itau.ID, []importer.Line{{ImportID: "1", At: jan10, Description: "PIX", Amount: -500, Account: "conta corrente 123"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.EnsureDefaults(ctx, userB); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Segredo do B", Type: "asset", OnBudget: true, Opening: 1000, OpeningAt: jan10}); err != nil {
		t.Fatal(err)
	}

	ex, err := f.svc.ExportUserData(ctx, userA)
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	covered := map[string]int{
		"ledger_account":             len(ex.Accounts),
		"ledger_transaction":         len(ex.Transactions),
		"ledger_transaction_history": len(ex.History),
		"ledger_import_line":         len(ex.ImportLines),
		"ledger_user_key":            -1, // security material, not exported
	}
	for table, n := range dbtest.UserRows(t, f.conn, "ledger_", userA) {
		want, ok := covered[table]
		switch {
		case !ok:
			t.Errorf("table %s is not covered by the export", table)
		case want >= 0 && want != n:
			t.Errorf("%s: exported %d of %d rows", table, want, n)
		}
	}
	legs := 0
	for _, tx := range ex.Transactions {
		legs += len(tx.Legs)
	}
	if want := countLegs(t, f, userA); legs != want {
		t.Errorf("exported %d legs, want %d", legs, want)
	}

	raw, err := json.Marshal(ex)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)
	for _, want := range []string{`"Posto Shell"`, `"Gasolina nova"`, `Gasolina velha`, `conta corrente 123`, `"amount_centavos":1600`, `"deleted_at":"`, `"system_role":"opening_balance"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("export missing %s", want)
		}
	}
	if strings.Contains(doc, "Segredo do B") {
		t.Error("export leaks user B's data")
	}
}
