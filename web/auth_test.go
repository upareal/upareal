// story: e03s01 e03s03
package web_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/oidctest"
)

// browser is an HTTPS app server plus a client with a cookie jar, like a real browser.
type browser struct {
	app    testApp
	base   string
	client *http.Client
}

func newBrowser(t *testing.T) browser {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	base := "https://" + srv.Listener.Addr().String()
	app := newTestApp(t, base)
	srv.Config.Handler = app.handler
	srv.StartTLS()
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.Jar, _ = cookiejar.New(nil)
	return browser{app: app, base: base, client: client}
}

func (b browser) get(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	res, err := b.client.Get(b.base + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

// noFollow returns a copy of the client that stops at the first redirect.
func (b browser) noFollow() *http.Client {
	c := *b.client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

func (b browser) cookie(name string) *http.Cookie {
	u, _ := url.Parse(b.base)
	for _, c := range b.client.Jar.Cookies(u) {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestGoogleSignIn_SignedOutHomeOffersGoogle(t *testing.T) {
	b := newBrowser(t)
	_, body := b.get(t, "/")
	if !strings.Contains(body, `href="/auth/login"`) || !strings.Contains(body, "Entrar com Google") {
		t.Errorf("signed-out home lacks the Google sign-in link:\n%s", body)
	}
}

func TestGoogleSignIn_FullFlowGreetsUserAndPersists(t *testing.T) {
	b := newBrowser(t)
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true})

	res, _ := b.get(t, "/auth/login") // follows: app → fake Google → callback → / → terms
	if res.StatusCode != http.StatusOK || res.Request.URL.Path != "/termos/aceitar" {
		t.Fatalf("first sign-in ended at %s with %d, want /termos/aceitar with 200", res.Request.URL, res.StatusCode)
	}
	res, body := acceptTerms(t, b)
	if res.Request.URL.Path != "/" || !strings.Contains(body, "Olá, Maria") {
		t.Errorf("home after sign-in lacks greeting:\n%s", body)
	}
	if c := b.cookie("upareal_session"); c == nil || c.Value == "" {
		t.Fatal("no upareal_session cookie after sign-in")
	}
	if _, body := b.get(t, "/"); !strings.Contains(body, "Olá, Maria") {
		t.Error("greeting gone on the next request; session not kept")
	}
}

func TestGoogleSignIn_CookiesAreHardened(t *testing.T) {
	b := newBrowser(t)
	res, err := b.noFollow().Get(b.base + "/auth/login")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("GET /auth/login = %d, want 302", res.StatusCode)
	}
	var oauth *http.Cookie
	for _, c := range res.Cookies() {
		if c.Name == "upareal_oauth" {
			oauth = c
		}
	}
	if oauth == nil || !oauth.HttpOnly || !oauth.Secure || oauth.SameSite != http.SameSiteLaxMode || oauth.Path != "/auth" {
		t.Errorf("upareal_oauth cookie = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/auth", oauth)
	}
}

func TestGoogleSignIn_ForgedStateIsRejected(t *testing.T) {
	b := newBrowser(t)
	res, err := b.noFollow().Get(b.base + "/auth/login") // sets the oauth cookie
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	res, body := b.get(t, "/auth/callback?code=whatever&state=forged")
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("callback with forged state = %d, want 400 (%s)", res.StatusCode, body)
	}
	if b.cookie("upareal_session") != nil {
		t.Error("a session cookie was set despite the forged state")
	}
}

func TestGoogleSignIn_UnverifiedEmailIsRefused(t *testing.T) {
	b := newBrowser(t)
	b.app.fake.SetUser(oidctest.Claims{Subject: "g-9", Email: "x@example.com", Name: "X", EmailVerified: false})
	res, body := b.get(t, "/auth/login")
	if res.StatusCode != http.StatusForbidden || !strings.Contains(body, "verificado") {
		t.Errorf("unverified sign-in = %d, want 403 explaining the email must be verified:\n%s", res.StatusCode, body)
	}
	var n int
	if err := b.app.conn.QueryRow(`SELECT count(*) FROM users_user`).Scan(&n); err != nil || n != 0 {
		t.Errorf("users after refused sign-in = %d (%v), want 0", n, err)
	}
}

func TestGoogleSignIn_DeniedConsentReturnsHome(t *testing.T) {
	b := newBrowser(t)
	res, _ := b.get(t, "/auth/callback?error=access_denied&state=x")
	if res.Request.URL.Path != "/" || b.cookie("upareal_session") != nil {
		t.Errorf("denied consent ended at %s; want / without a session", res.Request.URL)
	}
}

func TestGoogleSignIn_TamperedSessionCookieIsSignedOut(t *testing.T) {
	b := newBrowser(t)
	u, _ := url.Parse(b.base)
	b.client.Jar.SetCookies(u, []*http.Cookie{{Name: "upareal_session", Value: "forged-token", Path: "/"}})
	_, body := b.get(t, "/")
	if strings.Contains(body, "Olá,") || !strings.Contains(body, "Entrar com Google") {
		t.Error("a forged session cookie was treated as signed in")
	}
}
