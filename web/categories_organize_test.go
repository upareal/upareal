// story: e05s02
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// linkID finds the ID in an edit link like href="/categorias/grupos/<id>">Name<.
func linkID(t *testing.T, body, prefix, name string) string {
	t.Helper()
	m := regexp.MustCompile(`href="` + regexp.QuoteMeta(prefix) + `([0-9a-f-]{36})">` + regexp.QuoteMeta(name) + `<`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no link %s<id> for %q", prefix, name)
	}
	return m[1]
}

func TestOrganizeCategories_CreateGroupAndCategory(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := b.postForm(t, "/categorias/grupos", url.Values{"nome": {"Pets"}, "tipo": {"expense"}})
	pets := linkID(t, body, "/categorias/grupos/", "Pets")
	_, body = b.postForm(t, "/categorias", url.Values{"grupo": {pets}, "nome": {"Ração"}})
	if !strings.Contains(body, ">Ração<") || strings.Index(body, ">Ração<") < strings.Index(body, ">Pets<") {
		t.Error("Ração not listed inside Pets")
	}
	res, body := b.postForm(t, "/categorias/grupos", url.Values{"nome": {""}, "tipo": {"expense"}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Informe um nome com até 100 caracteres.") {
		t.Errorf("empty group name = %d, want 400 with the name message", res.StatusCode)
	}
}

func TestOrganizeCategories_RenameAndRegroup(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := b.get(t, "/categorias")
	lazer := linkID(t, body, "/categorias/grupos/", "Lazer")
	receitas := linkID(t, body, "/categorias/grupos/", "Receitas")
	streaming := linkID(t, body, "/categorias/", "Streaming")
	mercado := linkID(t, body, "/categorias/", "Mercado")

	if res, page := b.get(t, "/categorias/grupos/"+lazer); res.StatusCode != http.StatusOK || !strings.Contains(page, `value="Lazer"`) {
		t.Fatalf("group edit page = %d", res.StatusCode)
	}
	_, body = b.postForm(t, "/categorias/grupos/"+lazer, url.Values{"nome": {"Diversão"}})
	if !strings.Contains(body, ">Diversão<") {
		t.Fatal("group not renamed")
	}
	res, page := b.get(t, "/categorias/"+streaming)
	if res.StatusCode != http.StatusOK || !strings.Contains(page, ">Diversão<") || strings.Contains(page, ">Receitas<") {
		t.Errorf("category edit page = %d; want same-kind groups only", res.StatusCode)
	}
	_, body = b.postForm(t, "/categorias/"+streaming, url.Values{"nome": {"Streaming"}, "grupo": {lazer}})
	if strings.Index(body, ">Streaming<") < strings.Index(body, ">Diversão<") {
		t.Error("Streaming not moved into Diversão")
	}
	res, body = b.postForm(t, "/categorias/"+mercado, url.Values{"nome": {"Mercado"}, "grupo": {receitas}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "A categoria só pode ir para um grupo do mesmo tipo.") {
		t.Errorf("expense category into Receitas = %d, want 400 with the B2 message", res.StatusCode)
	}
}

func TestOrganizeCategories_Move(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := b.get(t, "/categorias")
	_, body = b.postForm(t, "/categorias/grupos/"+linkID(t, body, "/categorias/grupos/", "Alimentação")+"/mover?dir=up", url.Values{})
	if strings.Index(body, ">Alimentação<") > strings.Index(body, ">Moradia<") {
		t.Error("Alimentação not moved above Moradia")
	}
	_, body = b.postForm(t, "/categorias/"+linkID(t, body, "/categorias/", "Condomínio")+"/mover?dir=up", url.Values{})
	if strings.Index(body, ">Condomínio<") > strings.Index(body, ">Aluguel ou financiamento<") {
		t.Error("Condomínio not moved above Aluguel ou financiamento")
	}
}

func TestOrganizeCategories_UnknownIDIs404(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	for _, path := range []string{"/categorias/0196a000-0000-7000-8000-000000000999", "/categorias/grupos/0196a000-0000-7000-8000-000000000999"} {
		if res, _ := b.get(t, path); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, res.StatusCode)
		}
	}
}
