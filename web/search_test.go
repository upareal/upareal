// story: e06s05
package web_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSearchPage_FindsByTextAndLinksToEdit(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau, mercado, _ := splitBook(t, b)
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(mercado, "50,00", "2026-02-10", "10:00", "Conta de Água"))
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(mercado, "150,00", "2026-02-05", "10:00", "Gasolina"))

	res, body := b.get(t, "/buscar?de=2026-02-01&ate=2026-02-28&texto=agua")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Conta de Água") || strings.Contains(body, ">Gasolina<") {
		t.Fatalf("search 'agua' = %d; want only Conta de Água", res.StatusCode)
	}
	for _, want := range []string{"1 resultado", "R$ 50,00", `href="/lancamentos/`} {
		if !strings.Contains(body, want) {
			t.Errorf("results missing %q", want)
		}
	}
	if _, body = b.get(t, "/buscar?de=2026-02-01&ate=2026-02-28&conta="+mercado+"&min=100,00"); !strings.Contains(body, "Gasolina") || strings.Contains(body, "Conta de Água") {
		t.Error("filter by account and minimum did not return only Gasolina")
	}
}

func TestSearchPage_DefaultsValidationAndNav(t *testing.T) {
	b := newBrowser(t)
	if res, _ := b.get(t, "/buscar"); res.Request.URL.Path != "/" {
		t.Errorf("signed-out /buscar ended at %s, want /", res.Request.URL)
	}
	signIn(t, b)
	res, body := b.get(t, "/buscar")
	from := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, `value="`+from+`"`) {
		t.Errorf("default search = %d; want the range to start 12 months ago (%s)", res.StatusCode, from)
	}
	for _, q := range []string{"de=2026-03-01&ate=2026-02-01", "de=2026-02-01&ate=2026-02-28&min=abc", "de=01/02/2026&ate=2026-02-28"} {
		if res, _ := b.get(t, "/buscar?"+q); res.StatusCode != http.StatusBadRequest {
			t.Errorf("/buscar?%s = %d, want 400", q, res.StatusCode)
		}
	}
	if _, body := b.get(t, "/"); !strings.Contains(body, `href="/buscar"`) {
		t.Error("home has no link to /buscar")
	}
}
