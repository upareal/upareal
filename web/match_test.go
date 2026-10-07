// story: e07s03
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestReviewTransfer_ConfirmTwinAndDirectPath(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, _, _ := splitBook(t, b)
	_, body := createAccount(t, b, url.Values{"nome": {"Poupança"}, "tipo": {"asset"}})
	poupanca := accountID(t, body, "Poupança")
	uploadCSV(t, b, itau, "h\nt1;10/02/2026;;TED para poupança;-1.000,00\nt2;12/02/2026;;TED sem par;-50,00\n")
	uploadCSV(t, b, poupanca, "h\np1;11/02/2026;;TED recebida;1.000,00\n")

	_, queue := b.get(t, "/revisao")
	link := txLink(t, queue, "TED para poupança")
	if _, page := b.get(t, link); !strings.Contains(page, "É uma transferência") || !strings.Contains(page, "/transferencia?conta=") {
		t.Fatal("queued line page has no transfer section")
	}
	transfer := strings.Replace(link, "?conta=", "/transferencia?conta=", 1)
	res, confirm := b.postForm(t, transfer, url.Values{"outra": {poupanca}})
	if res.StatusCode != http.StatusOK || !strings.Contains(confirm, "TED recebida") || !strings.Contains(confirm, "Não juntar") {
		t.Fatalf("confirmation page = %d; want the twin and 'Não juntar'", res.StatusCode)
	}
	m := regexp.MustCompile(`name="par" value="([0-9a-f-]{36})"`).FindStringSubmatch(confirm)
	if m == nil {
		t.Fatal("no twin choice on the confirmation page")
	}
	confirmURL := strings.Replace(link, "?conta=", "/transferencia/confirmar?conta=", 1)
	if res, body := b.postForm(t, confirmURL, url.Values{"outra": {poupanca}, "par": {"0196a000-0000-7000-8000-000000000999"}}); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Correspondente inválido") {
		t.Errorf("unknown twin = %d, want 400 'Correspondente inválido'", res.StatusCode)
	}
	res, queue = b.postForm(t, confirmURL, url.Values{"outra": {poupanca}, "par": {m[1]}})
	if res.Request.URL.Path != "/revisao" || strings.Contains(queue, "TED para poupança") || strings.Contains(queue, "TED recebida") {
		t.Errorf("after merging ended at %s; both lines must leave the queue", res.Request.URL)
	}
	if _, reg := b.get(t, "/contas/"+poupanca+"/lancamentos?mes=2026-02"); !strings.Contains(reg, "TED para poupança") || strings.Contains(reg, "TED recebida") {
		t.Error("Poupança register should show the one transfer, not the twin")
	}

	// No twin: the line becomes a transfer immediately.
	solo := strings.Replace(txLink(t, queue, "TED sem par"), "?conta=", "/transferencia?conta=", 1)
	res, queue = b.postForm(t, solo, url.Values{"outra": {poupanca}})
	if res.Request.URL.Path != "/revisao" || strings.Contains(queue, "TED sem par") {
		t.Errorf("no-twin transfer ended at %s; the line must leave the queue directly", res.Request.URL)
	}
}
