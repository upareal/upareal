// story: e06s04
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// txLink finds the edit link of the register line described as desc.
func txLink(t *testing.T, body, desc string) string {
	t.Helper()
	m := regexp.MustCompile(`href="(/lancamentos/[0-9a-f-]{36}\?conta=[0-9a-f-]{36})">` + regexp.QuoteMeta(desc) + `<`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no edit link for register line %q", desc)
	}
	return strings.ReplaceAll(m[1], "&amp;", "&")
}

func TestEditTransactionPage_EditEntryKeepsHistory(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, casa := splitBook(t, b) // Itaú R$ 1.000,00
	_, body := b.postForm(t, "/contas/"+itau+"/lancamentos", entry(mercado, "150,00", "2026-01-20", "10:00", "Gasolina"))
	link := txLink(t, body, "Gasolina")

	res, page := b.get(t, link)
	if res.StatusCode != http.StatusOK || !strings.Contains(page, `value="150,00"`) || !strings.Contains(page, `value="Gasolina"`) {
		t.Fatalf("edit page = %d; want the current amount and description", res.StatusCode)
	}
	form := url.Values{"contra": {casa}, "parte": {"120,00"}, "data": {"2026-01-20"}, "hora": {"10:00"}, "descricao": {"Gasolina corrigida"}}
	res, body = b.postForm(t, link, form)
	if res.Request.URL.Path != "/contas/"+itau+"/lancamentos" {
		t.Fatalf("after editing ended at %s, want the register", res.Request.URL)
	}
	for _, want := range []string{"Gasolina corrigida", "Casa", "-R$ 120,00", "R$ 880,00"} {
		if !strings.Contains(body, want) {
			t.Errorf("register after edit missing %q", want)
		}
	}
	if _, page = b.get(t, txLink(t, body, "Gasolina corrigida")); !strings.Contains(page, "Editado") || !strings.Contains(page, "-R$ 150,00") {
		t.Error("edit page does not show the previous version in the history")
	}
}

func TestEditTransactionPage_TransferAndDelete(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, _ := splitBook(t, b)
	_, body := createAccount(t, b, url.Values{"nome": {"Poupança"}, "tipo": {"asset"}})
	poupanca := accountID(t, body, "Poupança")
	_, body = b.postForm(t, "/contas/"+itau+"/transferencia", url.Values{"destino": {poupanca}, "valor": {"100,00"}, "data": {"2026-01-21"}, "hora": {"09:00"}, "descricao": {"Guardar"}})
	link := txLink(t, body, "Guardar")
	if _, page := b.get(t, link); !strings.Contains(page, `name="destino"`) {
		t.Fatal("a transfer opens without the transfer form")
	}
	_, body = b.postForm(t, link, url.Values{"origem": {itau}, "destino": {poupanca}, "valor": {"70,00"}, "data": {"2026-01-21"}, "hora": {"09:00"}, "descricao": {"Guardar"}})
	if !strings.Contains(body, "-R$ 70,00") || !strings.Contains(body, "R$ 930,00") {
		t.Error("transfer edit not reflected (want -R$ 70,00 and R$ 930,00)")
	}

	_, body = b.postForm(t, "/contas/"+itau+"/lancamentos", entry(mercado, "50,00", "2026-01-22", "10:00", "Apagar"))
	del := strings.Replace(txLink(t, body, "Apagar"), "?conta=", "/excluir?conta=", 1)
	_, body = b.postForm(t, del, url.Values{})
	if strings.Contains(body, "Apagar") || !strings.Contains(body, "R$ 930,00") {
		t.Error("deleted entry still shown or balance not restored")
	}
}

func TestEditTransactionPage_OpeningBalanceOnlyDeletes(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, _, _ := splitBook(t, b)
	_, body := b.get(t, "/contas/"+itau+"/lancamentos?mes=2026-01")
	_, page := b.get(t, txLink(t, body, "Saldo inicial"))
	if !strings.Contains(page, "use Ajustar saldo") || strings.Contains(page, `name="contra"`) || !strings.Contains(page, "/excluir") {
		t.Error("opening balance edit page should offer only delete, with the Ajustar saldo hint")
	}
	if res, _ := b.get(t, "/lancamentos/0196a000-0000-7000-8000-000000000999?conta="+itau); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown transaction = %d, want 404", res.StatusCode)
	}
}

func TestEditTransactionPage_AccountMustBelongToTheTransaction(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, casa := splitBook(t, b)
	_, body := b.postForm(t, "/contas/"+itau+"/lancamentos", entry(mercado, "10,00", "2026-01-20", "10:00", "Pão"))
	link := txLink(t, body, "Pão")
	tampered := strings.Replace(link, "conta="+itau, "conta="+casa, 1)
	form := url.Values{"contra": {mercado}, "parte": {"10,00"}, "data": {"2026-01-20"}, "hora": {"10:00"}}
	if res, _ := b.postForm(t, tampered, form); res.StatusCode != http.StatusNotFound {
		t.Errorf("edit with an account outside the transaction = %d, want 404", res.StatusCode)
	}
}
