// story: e07s02
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestReviewCategorize_SplitQueuedLineKeepingTheBankAmount(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, casa := splitBook(t, b) // Itaú R$ 1.000,00
	uploadCSV(t, b, itau, goodCSV)
	_, queue := b.get(t, "/revisao")
	link := txLink(t, queue, "SUPERMERCADO X")

	res, page := b.get(t, link)
	if res.StatusCode != http.StatusOK || !strings.Contains(page, "Categorizar") || !strings.Contains(page, "Valor do extrato") || !strings.Contains(page, "-R$ 150,00") {
		t.Fatalf("queued line edit page = %d; want the Categorizar form with the bank amount", res.StatusCode)
	}
	if strings.Contains(page, "use Ajustar saldo") {
		t.Error("queued line still shows the system-account hint (e07s01 UAT feedback)")
	}

	bad := url.Values{"categorizar": {"1"}, "contra": {mercado, casa}, "parte": {"100,00", "40,00"}}
	if res, body := b.postForm(t, link, bad); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Falta distribuir R$ 10,00") {
		t.Errorf("lines below the bank amount = %d, want 400 'Falta distribuir R$ 10,00'", res.StatusCode)
	}
	good := url.Values{"categorizar": {"1"}, "contra": {mercado, casa}, "parte": {"100,00", "50,00"}}
	res, body := b.postForm(t, link, good)
	if res.Request.URL.Path != "/contas/"+itau+"/lancamentos" || !strings.Contains(body, "Mercado, Casa") || !strings.Contains(body, "-R$ 150,00") {
		t.Errorf("after categorizing ended at %s; want the register with 'Mercado, Casa' and -R$ 150,00", res.Request.URL)
	}
	if _, queue = b.get(t, "/revisao"); strings.Contains(queue, "SUPERMERCADO X") {
		t.Error("the categorized line is still in the review queue")
	}
}

func TestReviewCategorize_OpeningBalanceKeepsItsHint(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, _, _ := splitBook(t, b)
	_, body := b.get(t, "/contas/"+itau+"/lancamentos?mes=2026-01")
	if _, page := b.get(t, txLink(t, body, "Saldo inicial")); !strings.Contains(page, "use Ajustar saldo") || strings.Contains(page, "Categorizar") {
		t.Error("opening balance must keep the Ajustar saldo hint and no Categorizar form")
	}
}
