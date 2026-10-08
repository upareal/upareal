// story: e08s06
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestArchiveMergeCategories_MergeArchiveRestore(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, accounts := createAccount(t, b, url.Values{"nome": {"Netflix"}, "tipo": {"expense"}})
	netflix := accountID(t, accounts, "Netflix")
	_, cats := b.get(t, "/categorias")
	streaming, outras := linkID(t, cats, "/categorias/", "Streaming"), linkID(t, cats, "/categorias/", "Outras assinaturas")
	hobbies := linkID(t, cats, "/categorias/", "Hobbies")
	b.postForm(t, "/planejamento/mapa/"+netflix, url.Values{"categoria": {streaming}})
	mes := time.Now().In(saoPaulo(t)).Format("2006-01")
	b.postForm(t, "/orcamento/"+mes+"/"+streaming, url.Values{"valor": {"50,00"}})
	b.postForm(t, "/orcamento/"+mes+"/"+outras, url.Values{"valor": {"30,00"}})

	if _, page := b.get(t, "/categorias/"+streaming); !strings.Contains(page, `action="/categorias/`+streaming+`/juntar"`) || !strings.Contains(page, ">Outras assinaturas<") || strings.Contains(page, ">Salário<") {
		t.Fatal("category page has no Juntar com form limited to same-kind categories")
	}
	res, body := b.postForm(t, "/categorias/"+streaming+"/juntar", url.Values{"destino": {outras}})
	if res.Request.URL.Path != "/categorias" || strings.Contains(body, ">Streaming<") {
		t.Errorf("after merge ended at %s; Streaming must leave the active list", res.Request.URL)
	}
	if _, month := b.get(t, "/orcamento?mes="+mes); !strings.Contains(month, `value="80,00"`) {
		t.Error("Outras assinaturas does not hold the merged R$ 80,00")
	}
	if _, mapa := b.get(t, "/planejamento/mapa"); !strings.Contains(formBlock(t, mapa, "Netflix"), `value="`+outras+`" selected`) {
		t.Error("Netflix does not map to Outras assinaturas after the merge")
	}

	b.postForm(t, "/orcamento/"+mes+"/"+hobbies, url.Values{"valor": {"20,00"}})
	if res, body := b.postForm(t, "/categorias/"+hobbies+"/arquivar", nil); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Disponível zero") {
		t.Errorf("archiving a non-empty envelope = %d, want 400 mentioning 'Disponível zero'", res.StatusCode)
	}
	b.postForm(t, "/orcamento/"+mes+"/"+hobbies, url.Values{"valor": {"0,00"}})
	_, body = b.postForm(t, "/categorias/"+hobbies+"/arquivar", nil)
	if !strings.Contains(body, "Arquivadas") || !strings.Contains(body, `action="/categorias/`+hobbies+`/restaurar"`) {
		t.Fatal("archived Hobbies is not listed under Arquivadas with Restaurar")
	}
	_, body = b.postForm(t, "/categorias/"+hobbies+"/restaurar", nil)
	if !strings.Contains(body, `href="/categorias/`+hobbies+`">Hobbies<`) {
		t.Error("restored Hobbies is not back in the active list")
	}
}

func saoPaulo(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
