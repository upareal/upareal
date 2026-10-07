// story: e05s03
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestBudgetMonthPage_SetAmountsAndTotals(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	salary := linkID(t, cats, "/categorias/", "Salário")

	res, body := b.get(t, "/orcamento?mes=2026-01")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Janeiro de 2026") {
		t.Fatalf("GET /orcamento?mes=2026-01 = %d, want 200 with 'Janeiro de 2026'", res.StatusCode)
	}
	b.postForm(t, "/orcamento/2026-01/"+fuel, url.Values{"valor": {"300,00"}})
	res, body = b.postForm(t, "/orcamento/2026-01/"+salary, url.Values{"valor": {"8.000,00"}})
	if res.Request.URL.String() != b.base+"/orcamento?mes=2026-01" {
		t.Errorf("after saving ended at %s, want back on the same month", res.Request.URL)
	}
	for _, want := range []string{`value="300,00"`, `value="8.000,00"`, "Despesas: R$ 300,00", "Receitas: R$ 8.000,00"} {
		if !strings.Contains(body, want) {
			t.Errorf("month page missing %q", want)
		}
	}
}

func TestBudgetMonthPage_NextMonthStartsFromThePlan(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	_, body := b.postForm(t, "/orcamento/2026-01/"+linkID(t, cats, "/categorias/", "Combustível"), url.Values{"valor": {"300,00"}})
	if !strings.Contains(body, `href="/orcamento?mes=2026-02"`) || !strings.Contains(body, `href="/orcamento?mes=2025-12"`) {
		t.Fatal("month page has no previous/next month links")
	}
	if _, body = b.get(t, "/orcamento?mes=2026-02"); !strings.Contains(body, `value="300,00"`) {
		t.Error("2026-02 does not start from January's plan")
	}
}

func TestBudgetMonthPage_InvalidInput(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	if res, body := b.postForm(t, "/orcamento/2026-01/"+fuel, url.Values{"valor": {"-10,00"}}); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Valor inválido") {
		t.Errorf("negative amount = %d, want 400 with 'Valor inválido'", res.StatusCode)
	}
	if res, _ := b.get(t, "/orcamento?mes=2026-13"); res.StatusCode != http.StatusBadRequest {
		t.Errorf("month 2026-13 = %d, want 400", res.StatusCode)
	}
	if res, _ := b.postForm(t, "/orcamento/2026-01/0196a000-0000-7000-8000-000000000999", url.Values{"valor": {"1,00"}}); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown category = %d, want 404", res.StatusCode)
	}
}

func TestBudgetMonthPage_DefaultsToCurrentMonthAndIsLinked(t *testing.T) {
	b := newBrowser(t)
	if res, _ := b.get(t, "/orcamento"); res.Request.URL.Path != "/" {
		t.Errorf("signed-out /orcamento ended at %s, want /", res.Request.URL)
	}
	signIn(t, b)
	if res, _ := b.get(t, "/orcamento"); res.StatusCode != http.StatusOK {
		t.Errorf("GET /orcamento = %d, want 200 (current month)", res.StatusCode)
	}
	if _, body := b.get(t, "/"); !strings.Contains(body, `href="/orcamento"`) {
		t.Error("home has no link to /orcamento")
	}
}

// story: e08s02
func TestBudgetMonthPage_ShowsSpentAvailableAndToBeAssigned(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"0,00"}, "data": {"2026-01-01"}, "orcamento": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	createAccount(t, b, url.Values{"nome": {"Padaria"}, "tipo": {"expense"}})
	_, accounts := createAccount(t, b, url.Values{"nome": {"Recebimentos"}, "tipo": {"income"}})
	itau, shell, padaria, recebimentos := accountID(t, accounts, "Itaú"), accountID(t, accounts, "Posto Shell"), accountID(t, accounts, "Padaria"), accountID(t, accounts, "Recebimentos")
	_, cats := b.get(t, "/categorias")
	fuel, salary := linkID(t, cats, "/categorias/", "Combustível"), linkID(t, cats, "/categorias/", "Salário")
	b.postForm(t, "/planejamento/mapa/"+shell, url.Values{"categoria": {fuel}})
	b.postForm(t, "/planejamento/mapa/"+recebimentos, url.Values{"categoria": {salary}})
	b.postForm(t, "/orcamento/2026-01/"+fuel, url.Values{"valor": {"300,00"}})
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "120,00", "2026-01-15", "", "Gasolina"))
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(recebimentos, "8.000,00", "2026-01-05", "", "Salário"))
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(padaria, "50,00", "2026-01-16", "", "Pão"))

	_, body := b.get(t, "/orcamento?mes=2026-01")
	for _, want := range []string{
		"A distribuir: R$ 7.700,00",
		"Gasto R$ 120,00", "Disponível R$ 180,00",
		"Recebido R$ 8.000,00",
		"Sem categoria", "Gasto R$ 50,00", `href="/planejamento/mapa"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("month page missing %q", want)
		}
	}
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "200,00", "2026-01-20", "", "Gasolina"))
	if _, body = b.get(t, "/orcamento?mes=2026-01"); !strings.Contains(body, `class="negative">Disponível -R$ 20,00`) {
		t.Error("overspent category is not shown as negative")
	}
}
