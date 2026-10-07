// story: e04s02
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func createAccount(t *testing.T, b browser, form url.Values) (*http.Response, string) {
	t.Helper()
	return b.postForm(t, "/contas", form)
}

func TestAccountsPage_ListsByTypeWithoutSystemAccounts(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	res, body := b.get(t, "/contas")
	if res.Request.URL.Path != "/contas" || res.StatusCode != http.StatusOK {
		t.Fatalf("GET /contas ended at %s (%d)", res.Request.URL, res.StatusCode)
	}
	for _, want := range []string{"Contas e dinheiro", "Cartões e dívidas", "Receitas", "Despesas", `action="/contas"`} {
		if !strings.Contains(body, want) {
			t.Errorf("accounts page missing %q", want)
		}
	}
	for _, hidden := range []string{"Saldo inicial", "A categorizar"} {
		if strings.Contains(body, hidden) {
			t.Errorf("system account %q shown", hidden)
		}
	}
}

func TestAccountsPage_CreateAccountsWithOpeningBalances(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Conta corrente"}, "tipo": {"asset"}, "saldo": {"2.500,00"}, "data": {"2026-01-10"}, "orcamento": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Nubank"}, "tipo": {"liability"}, "cartao": {"on"}, "saldo": {"800,00"}})
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	res, body := createAccount(t, b, url.Values{"nome": {"Salário"}, "tipo": {"income"}})
	if res.Request.URL.Path != "/contas" {
		t.Fatalf("create ended at %s, want redirect to /contas", res.Request.URL)
	}
	for _, want := range []string{"Conta corrente", "R$ 2.500,00", "Nubank", "R$ 800,00 a pagar", "Posto Shell", "Salário"} {
		if !strings.Contains(body, want) {
			t.Errorf("accounts page missing %q", want)
		}
	}
	if strings.Index(body, "Posto Shell") < strings.Index(body, "Despesas") {
		t.Error("expense account not listed under Despesas")
	}
}

func TestAccountsPage_AdjustBalance(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := createAccount(t, b, url.Values{"nome": {"Conta corrente"}, "tipo": {"asset"}, "saldo": {"2.500,00"}})
	m := regexp.MustCompile(`action="(/contas/[^"]+/ajuste)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("no adjust form on the accounts page")
	}
	_, body = b.postForm(t, m[1], url.Values{"saldo": {"2.480,00"}})
	if !strings.Contains(body, "R$ 2.480,00") || strings.Contains(body, "R$ 2.500,00") {
		t.Error("balance not adjusted to R$ 2.480,00")
	}
}

func TestAccountsPage_InvalidInputIsRejected(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	res, body := createAccount(t, b, url.Values{"nome": {"Poupança"}, "tipo": {"asset"}, "saldo": {"12,345"}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Valor inválido") {
		t.Errorf("malformed amount = %d, want 400 with 'Valor inválido'", res.StatusCode)
	}
	res, _ = createAccount(t, b, url.Values{"nome": {""}, "tipo": {"asset"}})
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("empty name = %d, want 400", res.StatusCode)
	}
	if _, body := b.get(t, "/contas"); strings.Contains(body, "Poupança") {
		t.Error("an invalid account was created")
	}
}

func TestAccountsPage_HomeLinksAndSignedOutGoesHome(t *testing.T) {
	b := newBrowser(t)
	if res, _ := b.get(t, "/contas"); res.Request.URL.Path != "/" {
		t.Errorf("signed-out /contas ended at %s, want /", res.Request.URL)
	}
	signIn(t, b)
	if _, body := b.get(t, "/"); !strings.Contains(body, `href="/contas"`) {
		t.Error("home has no link to /contas")
	}
}
