// story: e03s02 e03s03
package web_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/oidctest"
)

// signIn completes the Google flow in b and fails the test unless greeted.
func signIn(t *testing.T, b browser) {
	t.Helper()
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true})
	b.get(t, "/auth/login")
	if _, body := acceptTerms(t, b); !strings.Contains(body, "Olá, Maria") {
		t.Fatalf("sign-in did not greet the user:\n%s", body)
	}
}

// secondBrowser is another browser (own cookie jar) on the same app server.
func secondBrowser(b browser) browser {
	c := *b.client
	c.Jar, _ = cookiejar.New(nil)
	return browser{app: b.app, base: b.base, client: &c}
}

func (b browser) post(t *testing.T, path string) *http.Response {
	t.Helper()
	res, err := b.client.PostForm(b.base+path, url.Values{})
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	_ = res.Body.Close()
	return res
}

func TestLogout_HomeOffersBothLogoutForms(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	_, body := b.get(t, "/")
	for _, want := range []string{`action="/auth/logout"`, `action="/auth/logout-all"`, "Sair de todos os dispositivos", `method="post"`} {
		if !strings.Contains(body, want) {
			t.Errorf("signed-in home missing %s", want)
		}
	}
}

func TestLogout_SignsOutThisBrowser(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	res := b.post(t, "/auth/logout")
	if res.Request.URL.Path != "/" {
		t.Errorf("logout ended at %s, want /", res.Request.URL)
	}
	if _, body := b.get(t, "/"); !strings.Contains(body, "Entrar com Google") {
		t.Error("still signed in after logout")
	}
}

func TestLogout_AllDevicesSignsOutOtherBrowsers(t *testing.T) {
	a := newBrowser(t)
	signIn(t, a)
	b := secondBrowser(a)
	signIn(t, b)

	a.post(t, "/auth/logout-all")
	if _, body := b.get(t, "/"); !strings.Contains(body, "Entrar com Google") {
		t.Error("other browser still signed in after logout-all")
	}
	if _, body := a.get(t, "/"); !strings.Contains(body, "Entrar com Google") {
		t.Error("this browser still signed in after logout-all")
	}
}

func TestLogout_WithoutSessionIsHarmless(t *testing.T) {
	b := newBrowser(t)
	res := b.post(t, "/auth/logout")
	if res.StatusCode != http.StatusOK || res.Request.URL.Path != "/" {
		t.Errorf("logout without session = %d at %s, want 200 at /", res.StatusCode, res.Request.URL)
	}
}
