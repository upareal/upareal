// story: e06s01
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// registerBook creates Itaú (R$ 1.000,00 on 2026-01-10), a card, Posto Shell and Salário.
func registerBook(t *testing.T, b browser) (itau, card, shell, salario string) {
	t.Helper()
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {"2026-01-10"}, "orcamento": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Nubank"}, "tipo": {"liability"}, "cartao": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Salário"}, "tipo": {"income"}})
	return accountID(t, body, "Itaú"), accountID(t, body, "Nubank"), accountID(t, body, "Posto Shell"), accountID(t, body, "Salário")
}

func entry(counter, valor, data, hora, desc string) url.Values {
	return url.Values{"contra": {counter}, "valor": {valor}, "data": {data}, "hora": {hora}, "descricao": {desc}}
}

func TestRecordTransaction_ExpenseAndIncome(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, _, shell, salario := registerBook(t, b)

	res, body := b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "150,00", "2026-01-31", "22:30", "Gasolina"))
	if res.Request.URL.String() != b.base+"/contas/"+itau+"/lancamentos?mes=2026-01" {
		t.Errorf("after saving ended at %s, want the entry's month", res.Request.URL)
	}
	for _, want := range []string{"Gasolina", "Posto Shell", "31/01 22:30", "-R$ 150,00", "R$ 850,00"} {
		if !strings.Contains(body, want) {
			t.Errorf("register missing %q", want)
		}
	}
	_, body = b.postForm(t, "/contas/"+itau+"/lancamentos", entry(salario, "8.000,00", "2026-01-31", "08:00", "Salário"))
	if !strings.Contains(body, "R$ 8.850,00") {
		t.Error("income not reflected in the running balance (want R$ 8.850,00)")
	}
}

func TestMonthInSaoPaulo_LateNightEntryStaysInItsMonth(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, _, shell, _ := registerBook(t, b)
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "150,00", "2026-01-31", "22:30", "Gasolina"))
	if _, body := b.get(t, "/contas/"+itau+"/lancamentos?mes=2026-02"); strings.Contains(body, "Gasolina") {
		t.Error("22:30 on 31/01 in São Paulo appeared in February")
	}
}

func TestAccountRegister_CardInvalidInputAndLinks(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, card, shell, _ := registerBook(t, b)

	_, body := b.postForm(t, "/contas/"+card+"/lancamentos", entry(shell, "50,00", "2026-01-15", "12:00", "Combustível"))
	if !strings.Contains(body, "R$ 50,00 a pagar") {
		t.Error("card register does not show R$ 50,00 a pagar")
	}
	if res, body := b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "0", "2026-01-15", "12:00", "")); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Valor inválido") {
		t.Errorf("zero amount = %d, want 400 with 'Valor inválido'", res.StatusCode)
	}
	if res, _ := b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "1,00", "31/01/2026", "12:00", "")); res.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed date = %d, want 400", res.StatusCode)
	}
	if res, _ := b.get(t, "/contas/0196a000-0000-7000-8000-000000000999/lancamentos"); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown account register = %d, want 404", res.StatusCode)
	}
	if _, body := b.get(t, "/contas"); !strings.Contains(body, `href="/contas/`+itau+`/lancamentos"`) {
		t.Error("accounts page has no link to the register")
	}
}
