// story: e06s02
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// splitBook creates Itaú (R$ 1.000,00 on 2026-01-10) and expenses Mercado and Casa.
func splitBook(t *testing.T, b browser) (itau, mercado, casa string) {
	t.Helper()
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {"2026-01-10"}, "orcamento": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Mercado"}, "tipo": {"expense"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Casa"}, "tipo": {"expense"}})
	return accountID(t, body, "Itaú"), accountID(t, body, "Mercado"), accountID(t, body, "Casa")
}

func splitForm(total string, lines ...[2]string) url.Values {
	v := url.Values{"total": {total}, "data": {"2026-01-20"}, "hora": {"10:00"}, "descricao": {"Supermercado"}}
	for _, l := range lines {
		v.Add("contra", l[0])
		v.Add("parte", l[1])
	}
	return v
}

func TestSplitForm_PageFragmentsAndLink(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, casa := splitBook(t, b)

	if _, page := b.get(t, "/contas/"+itau+"/lancamentos"); !strings.Contains(page, `href="/contas/`+itau+`/dividido"`) {
		t.Error("register has no link to the split form")
	}
	res, page := b.get(t, "/contas/"+itau+"/dividido")
	if res.StatusCode != http.StatusOK || strings.Count(page, `name="contra"`) < 2 || !strings.Contains(page, `hx-get="/contas/`+itau+`/dividido/linha"`) {
		t.Fatalf("split page = %d; want two lines and an HTMX add-line button", res.StatusCode)
	}
	res, frag := b.get(t, "/contas/"+itau+"/dividido/linha")
	if res.StatusCode != http.StatusOK || !strings.Contains(frag, `name="contra"`) || !strings.Contains(frag, `name="parte"`) || strings.Contains(frag, "<html") {
		t.Errorf("add-line fragment = %d; want one bare line", res.StatusCode)
	}
	for _, tc := range []struct {
		form url.Values
		want string
	}{
		{splitForm("300,00", [2]string{mercado, "200,00"}), "Falta distribuir R$ 100,00"},
		{splitForm("300,00", [2]string{mercado, "200,00"}, [2]string{casa, "100,00"}), "Falta distribuir R$ 0,00"},
		{splitForm("300,00", [2]string{mercado, "200,00"}, [2]string{casa, "150,00"}), "Distribuído a mais R$ 50,00"},
	} {
		if _, got := b.postForm(t, "/contas/"+itau+"/dividido/restante", tc.form); !strings.Contains(got, tc.want) {
			t.Errorf("remaining fragment = %q, want %q", got, tc.want)
		}
	}
}

func TestSplitBalances_SaveAndMismatch(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, casa := splitBook(t, b)
	reg := "/contas/" + itau + "/dividido"

	res, body := b.postForm(t, reg, splitForm("300,00", [2]string{mercado, "200,00"}, [2]string{casa, "50,00"}))
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Falta distribuir R$ 50,00") {
		t.Errorf("mismatched split = %d, want 400 with 'Falta distribuir R$ 50,00'", res.StatusCode)
	}
	if res, body := b.postForm(t, reg, splitForm("100,00", [2]string{mercado, "100,00"})); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Informe pelo menos duas linhas.") {
		t.Errorf("one-line split = %d, want 400 asking for two lines", res.StatusCode)
	}
	// Spare empty rows are ignored.
	res, body = b.postForm(t, reg, splitForm("300,00", [2]string{mercado, "200,00"}, [2]string{casa, "100,00"}, [2]string{"", ""}))
	if res.Request.URL.String() != b.base+"/contas/"+itau+"/lancamentos?mes=2026-01" {
		t.Fatalf("after saving ended at %s, want the register month", res.Request.URL)
	}
	for _, want := range []string{"Supermercado", "Mercado, Casa", "-R$ 300,00", "R$ 700,00"} {
		if !strings.Contains(body, want) {
			t.Errorf("register missing %q", want)
		}
	}
	if strings.Count(body, "Supermercado") != 1 {
		t.Error("the split shows more than one register line")
	}
}
