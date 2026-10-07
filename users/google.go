// story: e03s01
package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const oidcTimeout = 10 * time.Second

// Google signs people in with OpenID Connect (authorization code + PKCE S256 +
// state + nonce). The ID token is verified by go-oidc: signature (JWKS),
// issuer, audience and expiry.
type Google struct {
	conf     oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewGoogle discovers the provider at issuerURL (https://accounts.google.com
// in production) and returns a client for clientID.
//
//	g, err := users.NewGoogle(ctx, cfg.OIDCIssuerURL, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.BaseURL+"/auth/callback")
func NewGoogle(ctx context.Context, issuerURL, clientID, clientSecret, redirectURL string) (*Google, error) {
	ctx, cancel := context.WithTimeout(ctx, oidcTimeout)
	defer cancel()
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("users: discover OIDC issuer %s (check OIDC_ISSUER_URL and network): %w", issuerURL, err)
	}
	return &Google{
		conf: oauth2.Config{
			ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
			Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// AuthCodeURL is where to send the browser. The caller keeps state, nonce and
// verifier (oauth2.GenerateVerifier) until the callback.
func (g *Google) AuthCodeURL(state, nonce, verifier string) string {
	return g.conf.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

// Exchange redeems the callback code and returns the verified identity.
// It does not judge EmailVerified; SignIn refuses unverified emails.
func (g *Google) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, oidcTimeout)
	defer cancel()
	tok, err := g.conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("users: exchange code: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("users: token response has no id_token")
	}
	idToken, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("users: verify id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, errors.New("users: id_token nonce does not match this sign-in")
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("users: read id_token claims: %w", err)
	}
	return Identity{Subject: idToken.Subject, Email: claims.Email, Name: claims.Name, EmailVerified: claims.EmailVerified}, nil
}
