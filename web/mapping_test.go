// story: e08s01
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// formBlock returns the HTML from the row labelled name up to the end of its form.
func formBlock(t *testing.T, body, name string) string {
	t.Helper()
	i := strings.Index(body, ">"+name+"<")
	if i < 0 {
		t.Fatalf("row %q not found", name)
	}
	end := strings.Index(body[i:], "</form>")
	return body[i : i+end]
}

func TestMappingPage_MapAccountsAndCountSemCategoria(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	createAccount(t, b, url.Values{"nome": {"Padaria"}, "tipo": {"expense"}})
	_, accounts := createAccount(t, b, url.Values{"nome": {"Recebimentos"}, "tipo": {"income"}})
	shell, recebimentos := accountID(t, accounts, "Posto Shell"), accountID(t, accounts, "Recebimentos")
	_, cats := b.get(t, "/categorias")
	fuel, mercado := linkID(t, cats, "/categorias/", "Combustível"), linkID(t, cats, "/categorias/", "Mercado")

	res, page := b.get(t, "/planejamento/mapa")
	if res.StatusCode != http.StatusOK || !strings.Contains(page, "3 sem categoria") {
		t.Fatalf("map page = %d; want '3 sem categoria'", res.StatusCode)
	}
	if row := formBlock(t, page, "Recebimentos"); strings.Contains(row, "Mercado") || !strings.Contains(row, "Salário") {
		t.Error("an income account's picker must offer only income categories")
	}

	res, page = b.postForm(t, "/planejamento/mapa/"+shell, url.Values{"categoria": {fuel}})
	if res.Request.URL.Path != "/planejamento/mapa" || !strings.Contains(page, "2 sem categoria") {
		t.Errorf("after mapping ended at %s; want '2 sem categoria'", res.Request.URL)
	}
	if row := formBlock(t, page, "Posto Shell"); !strings.Contains(row, `value="`+fuel+`" selected`) {
		t.Error("Posto Shell does not show Combustível selected")
	}
	if res, body := b.postForm(t, "/planejamento/mapa/"+recebimentos, url.Values{"categoria": {mercado}}); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "mesmo tipo") {
		t.Errorf("income to expense category = %d, want 400 mentioning 'mesmo tipo'", res.StatusCode)
	}
	if _, home := b.get(t, "/"); !strings.Contains(home, `href="/planejamento/mapa"`) {
		t.Error("home has no link to the mapping page")
	}
}

// story: e08s05
func TestMappingPage_OffBudgetAccountMapsToExpenseAndCountsAsSpent(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"2.000,00"}, "data": {"2026-01-01"}, "orcamento": {"on"}})
	_, accounts := createAccount(t, b, url.Values{"nome": {"Investimento XP"}, "tipo": {"asset"}, "saldo": {"5.000,00"}, "data": {"2026-01-01"}})
	itau, xp := accountID(t, accounts, "Itaú"), accountID(t, accounts, "Investimento XP")
	_, cats := b.get(t, "/categorias")
	invest := linkID(t, cats, "/categorias/", "Investimentos")

	_, page := b.get(t, "/planejamento/mapa")
	if row := formBlock(t, page, "Investimento XP"); !strings.Contains(row, "Investimentos") || strings.Contains(row, "Salário") {
		t.Error("an off-budget account's picker must offer expense categories only")
	}
	if strings.Contains(page, ">Itaú<") {
		t.Error("an on-budget account is listed on the map")
	}
	b.postForm(t, "/planejamento/mapa/"+xp, url.Values{"categoria": {invest}})
	_, before := b.get(t, "/orcamento?mes=2026-01")
	if !strings.Contains(before, "A distribuir: R$ 2.000,00") {
		t.Error("the off-budget opening balance changed A distribuir")
	}
	b.postForm(t, "/contas/"+itau+"/transferencia", url.Values{"destino": {xp}, "valor": {"1.000,00"}, "data": {"2026-01-20"}, "hora": {"09:00"}, "descricao": {"Aporte"}})
	if _, body := b.get(t, "/orcamento?mes=2026-01"); !strings.Contains(body, "Gasto R$ 1.000,00") {
		t.Error("the transfer to the off-budget account is not spent in Investimentos")
	}
}
