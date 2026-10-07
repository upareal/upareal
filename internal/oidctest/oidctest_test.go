// story: e03s01
package oidctest_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/upareal/upareal/internal/oidctest"
)

// authorize follows the provider's authorize redirect and returns the callback query.
func authorize(t *testing.T, authURL string) url.Values {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil || res.StatusCode != http.StatusFound {
		t.Fatalf("authorize = %d %q, want 302 redirect", res.StatusCode, res.Header.Get("Location"))
	}
	return loc.Query()
}

func TestFakeProvider_IssuesTokensRealClientsAccept(t *testing.T) {
	ctx := context.Background()
	fake := oidctest.New(t, "client-1")
	fake.SetUser(oidctest.Claims{Subject: "g-123", Email: "maria@example.com", Name: "Maria Silva", EmailVerified: true})

	provider, err := oidc.NewProvider(ctx, fake.URL)
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	conf := oauth2.Config{ClientID: "client-1", ClientSecret: "s", Endpoint: provider.Endpoint(),
		RedirectURL: "http://app.test/auth/callback", Scopes: []string{oidc.ScopeOpenID, "email", "profile"}}
	verifier := oauth2.GenerateVerifier()

	q := authorize(t, conf.AuthCodeURL("state-1", oidc.Nonce("nonce-1"), oauth2.S256ChallengeOption(verifier)))
	if q.Get("state") != "state-1" || q.Get("code") == "" {
		t.Fatalf("callback query = %v, want state-1 and a code", q)
	}

	tok, err := conf.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	idToken, err := provider.Verifier(&oidc.Config{ClientID: "client-1"}).Verify(ctx, raw)
	if err != nil {
		t.Fatalf("verify id_token: %v", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		t.Fatal(err)
	}
	if idToken.Subject != "g-123" || idToken.Nonce != "nonce-1" || claims.Email != "maria@example.com" ||
		!claims.EmailVerified || claims.Name != "Maria Silva" {
		t.Errorf("token = sub %q nonce %q %+v; want configured user and nonce-1", idToken.Subject, idToken.Nonce, claims)
	}
}

func TestFakeProvider_RejectsWrongPKCEVerifier(t *testing.T) {
	ctx := context.Background()
	fake := oidctest.New(t, "client-1")
	provider, err := oidc.NewProvider(ctx, fake.URL)
	if err != nil {
		t.Fatal(err)
	}
	conf := oauth2.Config{ClientID: "client-1", ClientSecret: "s", Endpoint: provider.Endpoint(), RedirectURL: "http://app.test/cb"}
	q := authorize(t, conf.AuthCodeURL("s", oauth2.S256ChallengeOption(oauth2.GenerateVerifier())))
	if _, err := conf.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(oauth2.GenerateVerifier())); err == nil {
		t.Error("exchange with a different PKCE verifier succeeded; want error")
	}
}
