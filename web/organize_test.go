// story: e04s03
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// accountID finds the edit link of the account called name on the accounts page.
func accountID(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(`href="/contas/([0-9a-f-]{36})">` + regexp.QuoteMeta(name) + `<`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no edit link for %q on the accounts page", name)
	}
	return m[1]
}

func TestOrganizeAccounts_RenameAndTypeLock(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := createAccount(t, b, url.Values{"nome": {"Conta"}, "tipo": {"asset"}, "saldo": {"100,00"}, "orcamento": {"on"}})
	id := accountID(t, body, "Conta")

	res, body := b.get(t, "/contas/"+id)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, `value="Conta"`) {
		t.Fatalf("edit page = %d, want 200 with the current name", res.StatusCode)
	}
	_, body = b.postForm(t, "/contas/"+id, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "orcamento": {"on"}})
	if !strings.Contains(body, ">Itaú<") {
		t.Error("rename not shown on the accounts page")
	}
	res, body = b.postForm(t, "/contas/"+id, url.Values{"nome": {"Itaú"}, "tipo": {"liability"}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Contas com lançamentos não podem mudar de tipo.") {
		t.Errorf("type change on a used account = %d, want 400 with the L5 message", res.StatusCode)
	}
}

func TestOrganizeAccounts_Move(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Alfa"}, "tipo": {"expense"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Beta"}, "tipo": {"expense"}})
	_, body = b.postForm(t, "/contas/"+accountID(t, body, "Beta")+"/mover?dir=up", url.Values{})
	if strings.Index(body, ">Beta<") > strings.Index(body, ">Alfa<") {
		t.Error("Beta not moved above Alfa")
	}
}

func TestOrganizeAccounts_ArchiveRestore(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Cheia"}, "tipo": {"asset"}, "saldo": {"10,00"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Velha"}, "tipo": {"asset"}})

	res, msg := b.postForm(t, "/contas/"+accountID(t, body, "Cheia")+"/arquivar", url.Values{})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(msg, "Zere o saldo antes de arquivar.") {
		t.Errorf("archiving a non-zero asset = %d, want 400 with the zero-balance message", res.StatusCode)
	}
	velha := accountID(t, body, "Velha")
	_, body = b.postForm(t, "/contas/"+velha+"/arquivar", url.Values{})
	arch := strings.Index(body, "Arquivadas")
	if arch < 0 || strings.Index(body, "Velha") < arch {
		t.Fatal("archived account not listed under Arquivadas")
	}
	_, body = b.postForm(t, "/contas/"+velha+"/restaurar", url.Values{})
	if strings.Contains(body, "Arquivadas") || !strings.Contains(body, ">Velha<") {
		t.Error("restored account not back in the active list")
	}
}

func TestOrganizeAccounts_Delete(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Usada"}, "tipo": {"asset"}, "saldo": {"10,00"}})
	_, body := createAccount(t, b, url.Values{"nome": {"Teste"}, "tipo": {"expense"}})
	usada := accountID(t, body, "Usada")

	_, body = b.postForm(t, "/contas/"+accountID(t, body, "Teste")+"/excluir", url.Values{})
	if strings.Contains(body, ">Teste<") {
		t.Error("unused account still listed after delete")
	}
	res, body := b.postForm(t, "/contas/"+usada+"/excluir", url.Values{})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Contas com lançamentos não podem ser excluídas. Arquive-a.") {
		t.Errorf("deleting a used account = %d, want 400 with the archive hint", res.StatusCode)
	}
}

func TestOrganizeAccounts_UnknownAccountIs404(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	if res, _ := b.get(t, "/contas/0196a000-0000-7000-8000-000000000999"); res.StatusCode != http.StatusNotFound {
		t.Errorf("edit page for an unknown account = %d, want 404", res.StatusCode)
	}
}
