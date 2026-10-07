// story: e03s03
package web_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/oidctest"
	"github.com/upareal/upareal/web"
)

// acceptTerms submits the acceptance form with the box checked and returns the final page.
func acceptTerms(t *testing.T, b browser) (*http.Response, string) {
	t.Helper()
	return b.postForm(t, "/termos/aceitar", url.Values{"aceito": {"on"}})
}

func (b browser) postForm(t *testing.T, path string, form url.Values) (*http.Response, string) {
	t.Helper()
	res, err := b.client.PostForm(b.base+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func signInWithoutAccepting(t *testing.T, b browser) {
	t.Helper()
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true})
	b.get(t, "/auth/login")
}

func TestTermsAcceptance_FirstUseRedirectsToForm(t *testing.T) {
	b := newBrowser(t)
	signInWithoutAccepting(t, b)
	res, body := b.get(t, "/")
	if res.Request.URL.Path != "/termos/aceitar" {
		t.Fatalf("GET / before accepting ended at %s, want /termos/aceitar", res.Request.URL)
	}
	for _, want := range []string{`name="aceito"`, `type="checkbox"`, "Aceitar e continuar", `href="/termos"`, `href="/privacidade"`} {
		if !strings.Contains(body, want) {
			t.Errorf("acceptance form missing %s", want)
		}
	}
}

func TestTermsAcceptance_UncheckedBoxIsRejected(t *testing.T) {
	b := newBrowser(t)
	signInWithoutAccepting(t, b)
	res, body := b.postForm(t, "/termos/aceitar", url.Values{})
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Marque a caixa para continuar") {
		t.Errorf("unchecked accept = %d, want 400 asking to check the box", res.StatusCode)
	}
	if res, _ := b.get(t, "/"); res.Request.URL.Path != "/termos/aceitar" {
		t.Error("app unlocked without acceptance")
	}
}

func TestTermsAcceptance_AcceptingUnlocksApp(t *testing.T) {
	b := newBrowser(t)
	signInWithoutAccepting(t, b)
	res, body := acceptTerms(t, b)
	if res.Request.URL.Path != "/" || !strings.Contains(body, "Olá, Maria") {
		t.Errorf("after accepting ended at %s, want / with greeting", res.Request.URL)
	}
	if res, _ := b.get(t, "/"); res.Request.URL.Path != "/" {
		t.Error("asked again right after accepting")
	}
}

func TestTermsAcceptance_NewVersionAsksAgain(t *testing.T) {
	b := newBrowser(t)
	signInWithoutAccepting(t, b)
	acceptTerms(t, b)
	if _, err := b.app.conn.Exec(`UPDATE users_user SET terms_version = 'older-version'`); err != nil {
		t.Fatal(err)
	}
	if res, _ := b.get(t, "/"); res.Request.URL.Path != "/termos/aceitar" {
		t.Errorf("user on an older terms version reached %s, want /termos/aceitar (current %s)", res.Request.URL, web.CurrentTermsVersion)
	}
}

func TestTermsAcceptance_ExemptRoutesWork(t *testing.T) {
	b := newBrowser(t)
	signInWithoutAccepting(t, b)
	for _, path := range []string{"/termos", "/privacidade", "/healthz"} {
		if res, _ := b.get(t, path); res.Request.URL.Path != path || res.StatusCode != http.StatusOK {
			t.Errorf("GET %s ended at %s (%d), want it served without the terms gate", path, res.Request.URL, res.StatusCode)
		}
	}
	res, err := b.client.PostForm(b.base+"/auth/logout", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if _, body := b.get(t, "/"); !strings.Contains(body, "Entrar com Google") {
		t.Error("logout did not work before accepting terms")
	}
}

func TestTermsAcceptance_SignedOutFormGoesHome(t *testing.T) {
	b := newBrowser(t)
	if res, _ := b.get(t, "/termos/aceitar"); res.Request.URL.Path != "/" {
		t.Errorf("signed-out /termos/aceitar ended at %s, want /", res.Request.URL)
	}
}
