// story: e03s01
package web

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/upareal/upareal/users"
)

const (
	oauthCookie   = "upareal_oauth"   // state.nonce.verifier for one sign-in attempt
	sessionCookie = "upareal_session" // opaque session token
	oauthMaxAge   = 10 * time.Minute
	sessionMaxAge = 90 * 24 * time.Hour // absolute limit; server-side expiry in e03s02
)

// login starts a sign-in: fresh state, nonce and PKCE verifier kept in a
// short-lived cookie, then off to Google.
func login(google *users.Google) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, nonce, verifier := rand.Text(), rand.Text(), oauth2.GenerateVerifier()
		http.SetCookie(w, &http.Cookie{
			Name: oauthCookie, Value: state + "." + nonce + "." + verifier, Path: "/auth",
			MaxAge: int(oauthMaxAge.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, google.AuthCodeURL(state, nonce, verifier), http.StatusFound)
	}
}

// callback finishes a sign-in: check state, exchange the code, sign in,
// start a session.
func callback(svc *users.Service, google *users.Google) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearCookie(w, oauthCookie, "/auth")
		q := r.URL.Query()
		if q.Get("error") != "" { // e.g. access_denied: the user cancelled
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		state, nonce, verifier, ok := readOAuthCookie(r)
		if !ok || subtle.ConstantTimeCompare([]byte(state), []byte(q.Get("state"))) != 1 {
			renderError(w, r, http.StatusBadRequest, "Sessão de login inválida",
				"O login expirou ou não começou aqui. Tente entrar novamente.")
			return
		}
		id, err := google.Exchange(r.Context(), q.Get("code"), verifier, nonce)
		if err != nil {
			renderError(w, r, http.StatusUnauthorized, "Não foi possível entrar", "Não foi possível entrar. Tente novamente.")
			return
		}
		user, err := svc.SignIn(r.Context(), id)
		if errors.Is(err, users.ErrEmailNotVerified) {
			renderError(w, r, http.StatusForbidden, "E-mail não verificado",
				"Seu e-mail do Google ainda não foi verificado. Verifique-o no Google e tente novamente.")
			return
		}
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Algo deu errado. Tente novamente em instantes.")
			return
		}
		token, err := svc.CreateSession(r.Context(), user.ID)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Algo deu errado. Tente novamente em instantes.")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: sessionCookie, Value: token, Path: "/",
			MaxAge: int(sessionMaxAge.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func readOAuthCookie(r *http.Request) (state, nonce, verifier string, ok bool) {
	c, err := r.Cookie(oauthCookie)
	if err != nil {
		return "", "", "", false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// currentUser resolves the session cookie. ok is false when signed out.
func currentUser(r *http.Request, svc *users.Service) (u users.User, ok bool, err error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return users.User{}, false, nil
	}
	u, err = svc.UserBySession(r.Context(), c.Value)
	if errors.Is(err, users.ErrNoSession) {
		return users.User{}, false, nil
	}
	if err != nil {
		return users.User{}, false, err
	}
	return u, true, nil
}

func clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: path, MaxAge: -1, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func renderError(w http.ResponseWriter, r *http.Request, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = errorPage(title, message).Render(r.Context(), w)
}
