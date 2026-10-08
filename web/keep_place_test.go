// story: e12s01
package web_test

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var postForms = regexp.MustCompile(`<form method="post" action="([^"]+)"[^>]*>`)

// assertKeepsPlace checks that every POST form whose action starts with
// prefix also posts through htmx and swaps <main> in place.
func assertKeepsPlace(t *testing.T, page, prefix string) {
	t.Helper()
	found := 0
	for _, m := range postForms.FindAllStringSubmatch(page, -1) {
		if !strings.HasPrefix(m[1], prefix) {
			continue
		}
		found++
		for _, attr := range []string{`hx-post="` + m[1] + `"`, `hx-target="main"`, `hx-select="main"`, `hx-swap="outerHTML"`} {
			if !strings.Contains(m[0], attr) {
				t.Errorf("form %s lacks %s", m[1], attr)
			}
		}
	}
	if found == 0 {
		t.Fatalf("no form posting to %s*", prefix)
	}
}

// htmxPost posts like htmx does (XHR with HX-Request), following redirects.
func htmxPost(t *testing.T, b browser, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, b.base+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", b.base)
	res, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func TestKeepPlace_RowFormsSwapMainInPlace(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	uploadCSV(t, b, itau, goodCSV)

	_, home := b.get(t, "/")
	if !strings.Contains(home, `<meta name="htmx-config"`) || !strings.Contains(home, `"responseHandling"`) || !strings.Contains(home, `"[45]..","swap":true`) {
		t.Error("layout has no htmx-config that swaps error responses")
	}
	_, orcamento := b.get(t, "/orcamento?mes=2026-01")
	assertKeepsPlace(t, orcamento, "/orcamento/2026-01/")
	_, revisao := b.get(t, "/revisao")
	assertKeepsPlace(t, revisao, "/revisao/")
	_, mapa := b.get(t, "/planejamento/mapa")
	assertKeepsPlace(t, mapa, "/planejamento/mapa/")
}

func TestKeepPlace_HtmxSaveReturnsFullPageAndErrors(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")

	res, body := htmxPost(t, b, "/orcamento/2026-01/"+fuel, url.Values{"valor": {"300,00"}})
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "<main>") || !strings.Contains(body, `value="300,00"`) {
		t.Errorf("htmx save = %d; want 200 with the updated <main>", res.StatusCode)
	}
	res, body = htmxPost(t, b, "/orcamento/2026-01/"+fuel, url.Values{"valor": {"abc"}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "<main>") || !strings.Contains(body, "Valor inválido") {
		t.Errorf("htmx invalid save = %d; want 400 with <main> and the message", res.StatusCode)
	}
}
