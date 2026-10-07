// story: e06s03
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestTransferForm_PayCardBill(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {"2026-01-10"}, "orcamento": {"on"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Nubank"}, "tipo": {"liability"}, "cartao": {"on"}, "saldo": {"800,00"}, "data": {"2026-01-10"}})
	itau, nubank := accountID(t, body, "Itaú"), accountID(t, body, "Nubank")

	if _, page := b.get(t, "/contas/"+itau+"/lancamentos?mes=2026-01"); !strings.Contains(page, `action="/contas/`+itau+`/transferencia"`) || !strings.Contains(page, `name="destino"`) {
		t.Fatal("register has no transfer form")
	}
	form := url.Values{"destino": {nubank}, "valor": {"800,00"}, "data": {"2026-01-20"}, "hora": {"09:00"}, "descricao": {"Fatura janeiro"}}
	res, body := b.postForm(t, "/contas/"+itau+"/transferencia", form)
	if res.Request.URL.String() != b.base+"/contas/"+itau+"/lancamentos?mes=2026-01" {
		t.Errorf("after transfer ended at %s, want Itaú's January register", res.Request.URL)
	}
	for _, want := range []string{"Fatura janeiro", "Nubank", "-R$ 800,00", "R$ 200,00"} {
		if !strings.Contains(body, want) {
			t.Errorf("Itaú register missing %q", want)
		}
	}
	_, body = b.get(t, "/contas/"+nubank+"/lancamentos?mes=2026-01")
	if !strings.Contains(body, "Fatura janeiro") || !strings.Contains(body, "Itaú") || !strings.Contains(body, "R$ 0,00") {
		t.Error("Nubank register does not show the payment bringing the debt to R$ 0,00")
	}
}

func TestTransferForm_RejectsSameAccountAndBadAmount(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"100,00"}})
	itau := accountID(t, body, "Itaú")
	same := url.Values{"destino": {itau}, "valor": {"10,00"}, "data": {"2026-01-20"}, "hora": {"09:00"}}
	if res, _ := b.postForm(t, "/contas/"+itau+"/transferencia", same); res.StatusCode != http.StatusBadRequest {
		t.Errorf("transfer to the same account = %d, want 400", res.StatusCode)
	}
	zero := url.Values{"destino": {itau}, "valor": {"0"}, "data": {"2026-01-20"}, "hora": {"09:00"}}
	if res, body := b.postForm(t, "/contas/"+itau+"/transferencia", zero); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Valor inválido") {
		t.Errorf("zero amount = %d, want 400 with 'Valor inválido'", res.StatusCode)
	}
}
