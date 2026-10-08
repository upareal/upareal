// story: e09s01
package web_test

import (
	"net/http"
	"net/url"
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
