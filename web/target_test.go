// story: e16s01
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/upareal/upareal/budget"
)

func TestTargetPage_SetSuggestAndRemove(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	salary := linkID(t, cats, "/categorias/", "Salário")
	cur := budget.MonthOf(time.Now())
	due := cur
	for range 5 {
		due = due.Next()
	}

	if _, page := b.get(t, "/categorias/"+salary); strings.Contains(page, `id="meta"`) {
		t.Error("income category offers a meta")
	}
	if _, page := b.get(t, "/categorias/"+fuel); !strings.Contains(page, `action="/categorias/`+fuel+`/meta"`) {
		t.Fatal("expense category page has no meta form")
	}
	res, page := b.postForm(t, "/categorias/"+fuel+"/meta", url.Values{"valor": {"2.400,00"}, "mes": {due.String()}})
	if res.StatusCode != http.StatusOK || !strings.Contains(page, `value="2.400,00"`) || !strings.Contains(page, `value="`+due.String()+`"`) {
		t.Fatalf("after saving the meta = %d; want the category page with the saved values", res.StatusCode)
	}

	_, month := b.get(t, "/orcamento?mes="+cur.String())
	for _, want := range []string{"Meta R$ 2.400,00 até", "0%", "Guarde R$ 400,00 este mês", "Abaixo do ritmo", "Usar sugestão", `name="valor" value="400,00"`} {
		if !strings.Contains(month, want) {
			t.Errorf("Orçamento missing %q", want)
		}
	}
	_, month = b.postForm(t, "/orcamento/"+cur.String()+"/"+fuel, url.Values{"valor": {"400,00"}})
	if !strings.Contains(month, "No ritmo") || strings.Contains(month, "Usar sugestão") {
		t.Error("after using the suggestion: want No ritmo and no suggestion button")
	}
	if _, late := b.get(t, "/orcamento?mes="+due.Next().String()); !strings.Contains(late, "Vencida") || strings.Contains(late, "Guarde") {
		t.Error("after the due month: want Vencida and no Guarde")
	}

	b.postForm(t, "/categorias/"+fuel+"/meta/remover", nil)
	if _, month = b.get(t, "/orcamento?mes="+cur.String()); strings.Contains(month, "Meta R$") {
		t.Error("removed meta still shown")
	}
}

func TestTargetPage_InvalidInput(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	cur := budget.MonthOf(time.Now())
	for _, tc := range []struct{ valor, mes, want string }{
		{"0,00", cur.String(), "Informe um valor maior que zero."},
		{"abc", cur.String(), "Informe um valor maior que zero."},
		{"100,00", cur.Prev().String(), "Escolha o mês atual ou um mês futuro."},
		{"100,00", "2026-13", "Informe o mês (AAAA-MM)."},
	} {
		res, page := b.postForm(t, "/categorias/"+fuel+"/meta", url.Values{"valor": {tc.valor}, "mes": {tc.mes}})
		if res.StatusCode != http.StatusBadRequest || !strings.Contains(page, tc.want) {
			t.Errorf("valor %q mes %q = %d; want 400 with %q", tc.valor, tc.mes, res.StatusCode, tc.want)
		}
	}
	if _, month := b.get(t, "/orcamento?mes="+cur.String()); strings.Contains(month, "Meta R$") {
		t.Error("an invalid meta was saved")
	}
}

func TestTargetHelp_ExplainsMetas(t *testing.T) {
	if _, body := get(t, newHandler(t), "/ajuda/orcamento"); !strings.Contains(body, "<h2>Metas</h2>") {
		t.Error("/ajuda/orcamento lacks the Metas section")
	}
}
