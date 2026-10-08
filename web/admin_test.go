// story: e09s01 e09s02
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/oidctest"
)

// signInJoao signs a non-admin user in on another browser of the same app.
func signInJoao(t *testing.T, b browser) browser {
	t.Helper()
	j := secondBrowser(b)
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-456", Email: "joao@example.com", Name: "João Souza", EmailVerified: true})
	j.get(t, "/auth/login")
	acceptTerms(t, j)
	return j
}

func TestAdminUsers_AdminSeesIdentityNeverMoney(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b) // Maria: in ADMIN_EMAILS
	createAccount(t, b, url.Values{"nome": {"Itaú Segredo"}, "tipo": {"asset"}, "saldo": {"1.234,56"}, "data": {"2026-01-01"}, "orcamento": {"on"}})
	signInJoao(t, b)

	if _, home := b.get(t, "/"); !strings.Contains(home, `href="/admin/usuarios"`) {
		t.Error("admin has no Admin link")
	}
	res, page := b.get(t, "/admin/usuarios")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin page = %d, want 200", res.StatusCode)
	}
	for _, want := range []string{"2 usuários", "Maria Silva", "maria@example.com", "João Souza", "joao@example.com", ">admin<", "Último acesso"} {
		if !strings.Contains(page, want) {
			t.Errorf("admin page missing %q", want)
		}
	}
	for _, leak := range []string{"Itaú Segredo", "1.234,56", "R$"} {
		if strings.Contains(page, leak) {
			t.Errorf("admin page leaks financial data %q", leak)
		}
	}
}

func TestAdminUsers_NonAdminGetsPlain404(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	j := signInJoao(t, b)
	if _, home := j.get(t, "/"); strings.Contains(home, "/admin/usuarios") {
		t.Error("non-admin sees the Admin link")
	}
	res, body := j.get(t, "/admin/usuarios")
	_, unknown := j.get(t, "/pagina-que-nao-existe")
	if res.StatusCode != http.StatusNotFound || body != unknown {
		t.Errorf("non-admin /admin/usuarios = %d; want 404 identical to an unknown URL", res.StatusCode)
	}
	if res, _ := secondBrowser(b).noFollow().Get(b.base + "/admin/usuarios"); res.StatusCode != http.StatusSeeOther {
		t.Errorf("signed-out /admin/usuarios = %d, want redirect home", res.StatusCode)
	}
}

// story: e09s02
func adminAction(t *testing.T, page, action string) []string {
	t.Helper()
	return regexp.MustCompile(`action="/admin/usuarios/([0-9a-f-]{36})/` + action + `"`).FindAllString(page, -1)
}

func joaoID(t *testing.T, page string) string {
	t.Helper()
	m := regexp.MustCompile(`action="/admin/usuarios/([0-9a-f-]{36})/excluir"`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("no Excluir form on the admin page")
	}
	return m[1]
}

func joaoLogin(t *testing.T, b, j browser) (*http.Response, string) {
	t.Helper()
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-456", Email: "joao@example.com", Name: "João Souza", EmailVerified: true})
	return j.get(t, "/auth/login")
}

func TestAdminUsers_DisableEnableAndDelete(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	j := signInJoao(t, b)
	createAccount(t, j, url.Values{"nome": {"Conta do João"}, "tipo": {"asset"}})
	_, page := b.get(t, "/admin/usuarios")
	if n := len(adminAction(t, page, "desativar")); n != 1 {
		t.Fatalf("Desativar forms = %d, want 1 (none on the admin's own row)", n)
	}
	id := joaoID(t, page)

	_, page = b.postForm(t, "/admin/usuarios/"+id+"/desativar", nil)
	if !strings.Contains(page, "desativado") || len(adminAction(t, page, "reativar")) != 1 {
		t.Error("disabled João is not marked desativado with a Reativar form")
	}
	if _, home := j.get(t, "/"); strings.Contains(home, "Olá, João") {
		t.Error("disabled João is still signed in")
	}
	if res, body := joaoLogin(t, b, j); res.StatusCode != http.StatusForbidden || !strings.Contains(body, "Sua conta está desativada") {
		t.Errorf("disabled sign-in = %d, want 403 'Sua conta está desativada'", res.StatusCode)
	}

	b.postForm(t, "/admin/usuarios/"+id+"/reativar", nil)
	joaoLogin(t, b, j)
	if _, contas := j.get(t, "/contas"); !strings.Contains(contas, "Conta do João") {
		t.Error("re-enabled João lost his data")
	}

	if res, body := b.postForm(t, "/admin/usuarios/"+id+"/excluir", nil); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Marque a caixa para confirmar") {
		t.Errorf("unconfirmed delete = %d, want 400", res.StatusCode)
	}
	_, page = b.postForm(t, "/admin/usuarios/"+id+"/excluir", url.Values{"confirmo": {"on"}})
	if strings.Contains(page, "joao@example.com") || !strings.Contains(page, "1 usuários") {
		t.Error("deleted João is still listed")
	}
	joaoLogin(t, b, j)
	acceptTerms(t, j)
	if _, contas := j.get(t, "/contas"); strings.Contains(contas, "Conta do João") {
		t.Error("signing in again after deletion shows old data")
	}
}

func TestAdminUsers_AdminsProtectedAndNonAdminsGet404(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	j := signInJoao(t, b)
	_, page := b.get(t, "/admin/usuarios")
	id := joaoID(t, page)
	if res, _ := j.postForm(t, "/admin/usuarios/"+id+"/desativar", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("non-admin POST = %d, want 404", res.StatusCode)
	}
	maria := regexp.MustCompile(`<tr data-id="([0-9a-f-]{36})"><td>Maria Silva<`).FindStringSubmatch(page)
	if maria == nil {
		t.Fatal("admin row has no data-id")
	}
	if res, body := b.postForm(t, "/admin/usuarios/"+maria[1]+"/desativar", nil); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Administradores não podem") {
		t.Errorf("disabling an admin = %d, want 400", res.StatusCode)
	}
	if res, _ := b.postForm(t, "/admin/usuarios/0196a000-0000-7000-8000-000000000000/excluir", url.Values{"confirmo": {"on"}}); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown user = %d, want 404", res.StatusCode)
	}
}
