// story: e10s02
package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDeleteMyAccount_TypedEmailDeletesEverythingAndSignsOut(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b) // Maria, admin
	j := signInJoao(t, b)
	createAccount(t, j, url.Values{"nome": {"Conta do João"}, "tipo": {"asset"}})

	if _, page := j.get(t, "/conta/dados"); !strings.Contains(page, `action="/conta/excluir"`) || !strings.Contains(page, `name="confirmar"`) {
		t.Fatal("Meus dados has no Excluir minha conta form")
	}
	if res, body := j.postForm(t, "/conta/excluir", url.Values{"confirmar": {"outra@example.com"}}); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Digite seu e-mail exatamente") {
		t.Fatalf("wrong confirmation = %d, want 400", res.StatusCode)
	}
	if _, contas := j.get(t, "/contas"); !strings.Contains(contas, "Conta do João") {
		t.Fatal("a refused deletion removed data")
	}

	res, body := j.postForm(t, "/conta/excluir", url.Values{"confirmar": {" JOAO@example.com "}})
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "Conta excluída") {
		t.Fatalf("confirmed deletion = %d, want 200 'Conta excluída'", res.StatusCode)
	}
	if _, home := j.get(t, "/"); strings.Contains(home, "Olá, João") {
		t.Error("João is still signed in after deleting his account")
	}
	if _, admin := b.get(t, "/admin/usuarios"); strings.Contains(admin, "joao@example.com") {
		t.Error("João is still listed after deleting his account")
	}
	joaoLogin(t, b, j)
	acceptTerms(t, j)
	if _, contas := j.get(t, "/contas"); strings.Contains(contas, "Conta do João") {
		t.Error("signing in again shows deleted data")
	}
}

func TestDeleteMyAccount_AdminsCannotDeleteThemselves(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	res, body := b.postForm(t, "/conta/excluir", url.Values{"confirmar": {"maria@example.com"}})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Administradores não podem excluir a própria conta") {
		t.Errorf("admin self-delete = %d, want 400", res.StatusCode)
	}
	if _, home := b.get(t, "/"); !strings.Contains(home, "Olá, Maria") {
		t.Error("a refused admin self-delete signed Maria out")
	}
}
