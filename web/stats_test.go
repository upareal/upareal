// story: e13s01
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPublicStats_VisitorSeesAggregates(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {"2026-01-01"}, "orcamento": {"on"}})
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	mes := time.Now().In(saoPaulo(t)).Format("2006-01")
	b.postForm(t, "/orcamento/"+mes+"/"+fuel, url.Values{"valor": {"1.300,00"}})

	visitor := secondBrowser(b) // signed out
	res, page := visitor.get(t, "/estatisticas")
	if res.StatusCode != http.StatusOK || res.Request.URL.Path != "/estatisticas" {
		t.Fatalf("visitor /estatisticas = %d at %s, want 200 without sign-in", res.StatusCode, res.Request.URL)
	}
	for _, want := range []string{
		"<dt>Usuários cadastrados</dt><dd>1</dd>",
		"<dt>Contas criadas</dt><dd>1</dd>",
		"<dt>Lançamentos registrados</dt><dd>1</dd>", // the opening balance
		"<dt>Linhas de extrato importadas</dt><dd>0</dd>",
		"<dt>Planejado neste mês</dt><dd>R$ 1.300,00</dd>",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("stats page missing %s", want)
		}
	}
	if strings.Contains(page, "Itaú") || strings.Contains(page, "maria") {
		t.Error("stats page shows per-user data")
	}
	if _, home := visitor.get(t, "/"); !strings.Contains(home, `href="/estatisticas"`) {
		t.Error("footer has no Estatísticas link")
	}
}
