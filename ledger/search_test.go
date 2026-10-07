// story: e06s05
package ledger_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger"
)

var (
	feb1 = time.Date(2026, 2, 1, 3, 0, 0, 0, time.UTC) // 00:00 São Paulo
	mar1 = time.Date(2026, 3, 1, 3, 0, 0, 0, time.UTC)
)

// searchBook records February entries (plus one outside the range and one deleted).
func searchBook(t *testing.T, f fixture) book {
	t.Helper()
	ctx := context.Background()
	b := newBook(t, f)
	rec := func(counter string, amount int64, at time.Time, desc string) string {
		id, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: counter, Amount: amount, At: at, Description: desc})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	rec(b.shell.ID, 15000, time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC), "Gasolina")
	rec(b.shell.ID, 5000, time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC), "Conta de Água")
	rec(b.salario.ID, 800000, time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC), "Salário fevereiro")
	rec(b.shell.ID, 999, time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC), "Abril")
	deleted := rec(b.shell.ID, 7777, time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC), "Apagada")
	if err := f.svc.DeleteTransaction(ctx, userA, deleted); err != nil {
		t.Fatal(err)
	}
	return b
}

func descriptions(rs []ledger.SearchResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Description
	}
	return out
}

func TestSearchFilters_RangeAccountAmount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := searchBook(t, f)

	all, truncated, err := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1})
	if err != nil || truncated {
		t.Fatalf("Search = %v, %v", err, truncated)
	}
	if got := descriptions(all); len(got) != 3 || got[0] != "Salário fevereiro" || got[2] != "Gasolina" {
		t.Errorf("February, newest first = %v; want [Salário fevereiro, Conta de Água, Gasolina]", got)
	}
	if all[0].Amount != 800000 || all[2].Amount != 15000 || all[2].LinkAccountID != b.itau.ID {
		t.Errorf("results = %+v; want amounts 800000 / 15000 and links from Itaú", all)
	}

	shell, _, _ := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1, AccountID: b.shell.ID})
	if got := descriptions(shell); len(got) != 2 {
		t.Errorf("account Posto Shell = %v, want 2", got)
	}
	big, _, _ := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1, AccountID: b.shell.ID, Min: 10000})
	if got := descriptions(big); len(got) != 1 || got[0] != "Gasolina" {
		t.Errorf("Posto Shell >= 100,00 = %v, want [Gasolina]", got)
	}
	small, _, _ := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1, Max: 10000})
	if got := descriptions(small); len(got) != 1 || got[0] != "Conta de Água" {
		t.Errorf("<= 100,00 = %v, want [Conta de Água]", got)
	}
}

func TestSearchFilters_IsolationAndValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := searchBook(t, f)
	_ = f.svc.EnsureDefaults(ctx, userB)
	if rs, _, err := f.svc.Search(ctx, userB, ledger.SearchQuery{From: feb1, To: mar1}); err != nil || len(rs) != 0 {
		t.Errorf("user B search = %v, %v; want nothing", descriptions(rs), err)
	}
	if rs, _, _ := f.svc.Search(ctx, userB, ledger.SearchQuery{From: feb1, To: mar1, AccountID: b.shell.ID}); len(rs) != 0 {
		t.Error("user B found transactions through user A's account ID")
	}
	for name, q := range map[string]ledger.SearchQuery{
		"no range":      {},
		"from after to": {From: mar1, To: feb1},
		"over 5 years":  {From: feb1.AddDate(-6, 0, 0), To: mar1},
		"min above max": {From: feb1, To: mar1, Min: 200, Max: 100},
	} {
		if _, _, err := f.svc.Search(ctx, userA, q); !errors.Is(err, ledger.ErrInvalidSearch) {
			t.Errorf("%s = %v, want ErrInvalidSearch", name, err)
		}
	}
}

func TestSearchText_AccentAndCaseInsensitive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	searchBook(t, f)
	for text, want := range map[string][]string{
		"agua":        {"Conta de Água"},
		"ÁGUA":        {"Conta de Água"},
		"conta de":    {"Conta de Água"},
		"posto shell": {"Conta de Água", "Gasolina"}, // matches the account name too
		"salario":     {"Salário fevereiro"},
		"apagada":     nil, // deleted never appear
		"inexistente": nil,
	} {
		got, _, err := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1, Text: text})
		if err != nil {
			t.Fatal(err)
		}
		if d := descriptions(got); len(d) != len(want) || (len(want) > 0 && d[0] != want[0]) {
			t.Errorf("text %q = %v, want %v", text, d, want)
		}
	}
}

func TestSearchText_CapsAt500(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := newBook(t, f)
	for i := range 501 {
		if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: b.itau.ID, CounterID: b.shell.ID, Amount: 1,
			At: feb1.Add(time.Duration(i) * time.Minute), Description: "Café"}); err != nil {
			t.Fatal(err)
		}
	}
	got, truncated, err := f.svc.Search(ctx, userA, ledger.SearchQuery{From: feb1, To: mar1, Text: "cafe"})
	if err != nil || len(got) != 500 || !truncated {
		t.Errorf("501 matches = %d results, truncated %v, %v; want 500, true", len(got), truncated, err)
	}
}
