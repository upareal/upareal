// story: e14s02
package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func TestReviewSuggest_UserDecides(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	_, accounts := createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	shell := accountID(t, accounts, "Posto Shell")
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "50,00", "2026-01-11", "10:00", "POSTO SHELL 1234"))
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "60,00", "2026-01-12", "10:00", "POSTO SHELL 5678"))
	uploadCSV(t, b, itau, "id;data;hora;descricao;valor\n"+
		"a1;20/01/2026;;COMPRA CARTAO POSTO SHELL 4567;-100,00\n"+
		"a2;21/01/2026;;LOJA NOVA;-30,00\n")

	_, page := b.get(t, "/revisao")
	use := regexp.MustCompile(`<form method="post" action="/revisao/([0-9a-f-]{36})"[^>]*><input type="hidden" name="contra" value="`+shell+`"><button type="submit">Usar Posto Shell</button>`).FindAllStringSubmatch(page, -1)
	if len(use) != 1 {
		t.Fatalf("want exactly one 'Usar Posto Shell' suggestion (for the posto line), got %d", len(use))
	}
	if !strings.Contains(page, "2 lançamentos parecidos") {
		t.Error("the suggestion does not explain itself")
	}
	if strings.Count(page, `<option value="" selected>— escolha —</option>`) != 2 {
		t.Error("each manual select must start at '— escolha —'")
	}
	if !strings.Contains(page, "COMPRA CARTAO POSTO SHELL 4567") || !strings.Contains(page, "LOJA NOVA") {
		t.Fatal("a suggestion categorized a line without the user's click")
	}

	_, page = b.postForm(t, "/revisao/"+use[0][1], url.Values{"contra": {shell}})
	if strings.Contains(page, "COMPRA CARTAO POSTO SHELL 4567") || !strings.Contains(page, "LOJA NOVA") {
		t.Error("using the suggestion did not categorize exactly that line")
	}
}
