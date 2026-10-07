// story: e03s01
package users_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/oidctest"
	"github.com/upareal/upareal/users"
)

const redirectURL = "http://app.test/auth/callback"

func newGoogle(t *testing.T) (*users.Google, *oidctest.Provider) {
	t.Helper()
	fake := oidctest.New(t, "client-1")
	g, err := users.NewGoogle(context.Background(), fake.URL, "client-1", "secret", redirectURL)
	if err != nil {
		t.Fatalf("NewGoogle: %v", err)
	}
	return g, fake
}

// approve follows the provider's authorize redirect and returns the callback query.
func approve(t *testing.T, authURL string) url.Values {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	return loc.Query()
}

func TestGoogle_AuthCodeURLCarriesStateNonceAndPKCE(t *testing.T) {
	g, _ := newGoogle(t)
	u, err := url.Parse(g.AuthCodeURL("st", "nn", "verifier-verifier-verifier-verifier-verifier"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("state") != "st" || q.Get("nonce") != "nn" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Errorf("auth URL query = %v, want state, nonce and S256 challenge", q)
	}
	if q.Get("redirect_uri") != redirectURL {
		t.Errorf("redirect_uri = %q, want %q", q.Get("redirect_uri"), redirectURL)
	}
	for _, scope := range []string{"openid", "email", "profile"} {
		if !strings.Contains(q.Get("scope"), scope) {
			t.Errorf("scope %q missing from %q", scope, q.Get("scope"))
		}
	}
}

func TestGoogle_ExchangeReturnsVerifiedIdentity(t *testing.T) {
	g, fake := newGoogle(t)
	fake.SetUser(oidctest.Claims{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true})
	verifier := "verifier-verifier-verifier-verifier-verifier"

	q := approve(t, g.AuthCodeURL("st", "nonce-1", verifier))
	id, err := g.Exchange(context.Background(), q.Get("code"), verifier, "nonce-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	want := users.Identity{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true}
	if id != want {
		t.Errorf("Exchange = %+v, want %+v", id, want)
	}
}

func TestGoogle_ExchangeReportsUnverifiedEmail(t *testing.T) {
	g, fake := newGoogle(t)
	fake.SetUser(oidctest.Claims{Subject: "g-9", Email: "x@example.com", Name: "X", EmailVerified: false})
	verifier := "verifier-verifier-verifier-verifier-verifier"
	q := approve(t, g.AuthCodeURL("st", "n", verifier))
	id, err := g.Exchange(context.Background(), q.Get("code"), verifier, "n")
	if err != nil {
		t.Fatal(err)
	}
	if id.EmailVerified {
		t.Error("EmailVerified = true, want false as issued")
	}
}

func TestGoogle_ExchangeRejectsNonceMismatch(t *testing.T) {
	g, _ := newGoogle(t)
	verifier := "verifier-verifier-verifier-verifier-verifier"
	q := approve(t, g.AuthCodeURL("st", "nonce-sent", verifier))
	if _, err := g.Exchange(context.Background(), q.Get("code"), verifier, "nonce-expected"); err == nil {
		t.Error("Exchange with mismatched nonce: expected error, got nil")
	}
}

func TestGoogle_NewGoogleFailsOnUnreachableIssuer(t *testing.T) {
	_, err := users.NewGoogle(context.Background(), "http://127.0.0.1:1", "c", "s", redirectURL)
	if err == nil || !strings.Contains(err.Error(), "http://127.0.0.1:1") {
		t.Errorf("NewGoogle unreachable = %v, want error naming the issuer", err)
	}
}
