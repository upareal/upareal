// story: e14s02
package ledger_test

import (
	"context"
	"testing"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/ledger/importer"
)

func TestSuggest_FromSimilarPastLinesOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f) // Itaú, Posto Shell (expense), Salário (income)
	mercado := create(t, f, ledger.NewAccount{Name: "Mercado", Type: "expense"})
	padaria := create(t, f, ledger.NewAccount{Name: "Padaria", Type: "expense"})
	antiga := create(t, f, ledger.NewAccount{Name: "Antiga", Type: "expense"})
	simple := func(counter, desc string, amount int64) string {
		t.Helper()
		id, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: counter, Amount: amount, At: jan10, Description: desc})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	simple(b.shell.ID, "POSTO SHELL 1234", 5000)
	simple(b.shell.ID, "POSTO SHELL 5678", 6000)
	simple(b.shell.ID, "PAGTO ELETRON POSTO SHELL", 7000)
	simple(mercado.ID, "SUPERMERCADO X", 9000)
	simple(b.salario.ID, "SALARIO ACME", 800000)
	for range 5 { // deleted history must not count
		if err := f.svc.DeleteTransaction(ctx, userA, simple(padaria.ID, "POSTO SHELL", 100)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.RecordSplit(ctx, userA, ledger.SplitEntry{AccountID: b.itau.ID, Total: 3000, At: jan10, Description: "POSTO SHELL LOJA",
		Parts: []ledger.Part{{CounterID: mercado.ID, Amount: 1000}, {CounterID: padaria.ID, Amount: 2000}}}); err != nil {
		t.Fatal(err) // splits must not count
	}
	simple(antiga.ID, "FARMACIA DROGASIL", 2000)
	if _, err := f.conn.Exec(`UPDATE ledger_account SET archived_at = '2026-01-01T00:00:00Z' WHERE id = ?`, antiga.ID); err != nil {
		t.Fatal(err) // an archived account is never suggested
	}
	if err := f.svc.EnsureDefaults(ctx, userB); err != nil {
		t.Fatal(err)
	}
	contaB, _ := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Conta B", Type: "asset", OnBudget: true})
	lojaB, _ := f.svc.CreateAccount(ctx, userB, ledger.NewAccount{Name: "Loja do B", Type: "expense"})
	if _, err := f.svc.RecordSimple(ctx, userB, ledger.SimpleEntry{AccountID: contaB.ID, CounterID: lojaB.ID, Amount: 100, At: jan10, Description: "LOJA DESCONHECIDA"}); err != nil {
		t.Fatal(err) // another user's history is never used
	}

	if _, err := f.svc.Import(ctx, userA, b.itau.ID, []importer.Line{
		{ImportID: "1", At: jan10, Description: "COMPRA CARTAO POSTO SHELL 4567", Amount: -10000},
		{ImportID: "2", At: jan10, Description: "SALARIO ACME 10/2026", Amount: 800000},
		{ImportID: "3", At: jan10, Description: "POSTO SHELL", Amount: 500}, // money in: no expense suggestion
		{ImportID: "4", At: jan10, Description: "LOJA DESCONHECIDA", Amount: -100},
		{ImportID: "5", At: jan10, Description: "FARMÁCIA DROGASIL 22", Amount: -2000},
	}); err != nil {
		t.Fatal(err)
	}
	queue, err := f.svc.ReviewQueue(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.SuggestCounters(ctx, userA, queue)
	if err != nil {
		t.Fatalf("SuggestCounters: %v", err)
	}
	byDesc := map[string]ledger.Suggestion{}
	for _, l := range queue {
		if s, ok := got[l.TransactionID]; ok {
			byDesc[l.Description] = s
		}
	}
	if s := byDesc["COMPRA CARTAO POSTO SHELL 4567"]; s.AccountID != b.shell.ID || s.AccountName != "Posto Shell" || s.Matches != 3 {
		t.Errorf("posto = %+v; want Posto Shell with 3 similar lines (deleted and split ignored)", s)
	}
	if s := byDesc["SALARIO ACME 10/2026"]; s.AccountID != b.salario.ID || s.Matches != 1 {
		t.Errorf("salário = %+v; want Salário (income) with 1 similar line", s)
	}
	for _, desc := range []string{"POSTO SHELL", "LOJA DESCONHECIDA", "FARMÁCIA DROGASIL 22"} {
		if s, ok := byDesc[desc]; ok {
			t.Errorf("%q suggested %+v; want none (wrong direction, other user's history, archived account)", desc, s)
		}
	}
}
