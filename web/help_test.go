// story: e15s01
package web_test

import (
	"net/http"
	"strings"
	"testing"
)

var helpPages = []string{
	"/ajuda/primeiros-passos",
	"/ajuda/contas",
	"/ajuda/importar",
	"/ajuda/orcamento",
	"/ajuda/dados",
}

func TestHelp_IndexLinksEveryPageForEveryone(t *testing.T) {
	h := newHandler(t)
	res, body := get(t, h, "/ajuda")
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "<h1>Ajuda</h1>") {
		t.Fatalf("GET /ajuda = %d, want 200 with the Ajuda heading", res.StatusCode)
	}
	for _, p := range helpPages {
		if !strings.Contains(body, `href="`+p+`"`) {
			t.Errorf("/ajuda lacks a link to %s", p)
		}
	}
}

func TestHelp_PagesRenderAndLinkBack(t *testing.T) {
	h := newHandler(t)
	for _, p := range append(helpPages, "/ajuda/csv") {
		res, body := get(t, h, p)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, "<h1>") {
			t.Errorf("GET %s = %d, want 200 with a heading", p, res.StatusCode)
		}
		if !strings.Contains(body, `href="/ajuda"`) {
			t.Errorf("%s lacks a link back to /ajuda", p)
		}
	}
}

func TestHelp_UnknownPageIsNotFound(t *testing.T) {
	res, _ := get(t, newHandler(t), "/ajuda/nao-existe")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET /ajuda/nao-existe = %d, want 404", res.StatusCode)
	}
}

func TestHelp_FooterLinkOnEveryPage(t *testing.T) {
	h := newHandler(t)
	for _, path := range []string{"/", "/privacidade", "/estatisticas", "/ajuda/csv"} {
		if _, body := get(t, h, path); !strings.Contains(body, `<a href="/ajuda">Ajuda</a>`) {
			t.Errorf("%s lacks the footer Ajuda link", path)
		}
	}
}
