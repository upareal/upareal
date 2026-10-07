// story: e05s01
package web_test

import (
	"strings"
	"testing"
)

func TestCategoriesPage_ListsDefaultsAfterFirstSignIn(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	res, body := b.get(t, "/categorias")
	if res.Request.URL.Path != "/categorias" || res.StatusCode != 200 {
		t.Fatalf("GET /categorias ended at %s (%d)", res.Request.URL, res.StatusCode)
	}
	for _, want := range []string{"Categorias", "Moradia", "Aluguel ou financiamento", "Alimentação", "Mercado", "Receitas", "Salário"} {
		if !strings.Contains(body, want) {
			t.Errorf("categories page missing %q", want)
		}
	}
	if strings.Index(body, "Moradia") > strings.Index(body, "Alimentação") {
		t.Error("groups are not in display order (Moradia before Alimentação)")
	}
	for _, hidden := range []string{"Saldo inicial", "A categorizar"} {
		if strings.Contains(body, hidden) {
			t.Errorf("system account %q shown on the categories page", hidden)
		}
	}
}

func TestCategoriesPage_SecondSignInAddsNothing(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	b.post(t, "/auth/logout")
	b.get(t, "/auth/login")
	_, body := b.get(t, "/categorias")
	if n := strings.Count(body, "Aluguel ou financiamento"); n != 1 {
		t.Errorf("'Aluguel ou financiamento' appears %d times after a second sign-in, want 1", n)
	}
}

func TestCategoriesPage_HomeLinksToIt(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	if _, body := b.get(t, "/"); !strings.Contains(body, `href="/categorias"`) {
		t.Error("signed-in home has no link to /categorias")
	}
}

func TestCategoriesPage_SignedOutGoesHome(t *testing.T) {
	b := newBrowser(t)
	if res, _ := b.get(t, "/categorias"); res.Request.URL.Path != "/" {
		t.Errorf("signed-out /categorias ended at %s, want /", res.Request.URL)
	}
}
