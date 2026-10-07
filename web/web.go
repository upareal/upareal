// story: e01s01 e03s01 e03s02

// Package web renders all HTML pages and HTMX fragments and wires the HTTP routes.
package web

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/upareal/upareal/users"
)

const healthzTimeout = 2 * time.Second

//go:embed static
var staticFiles embed.FS

// Deps are the modules the web layer renders. web owns no data.
type Deps struct {
	DB     *sql.DB
	Users  *users.Service
	Google *users.Google
}

// NewHandler returns the app's HTTP handler.
//
//	srv := &http.Server{Handler: web.NewHandler(web.Deps{DB: conn, Users: svc, Google: g})}
func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(d.DB))
	mux.Handle("GET /static/", staticHandler())
	mux.HandleFunc("GET /{$}", home(d.Users))
	authLimit := newIPLimiter()
	mux.Handle("GET /auth/login", authLimit.limit(login(d.Google)))
	mux.Handle("GET /auth/callback", authLimit.limit(callback(d.Users, d.Google)))
	mux.HandleFunc("POST /auth/logout", logout(d.Users, false))
	mux.HandleFunc("POST /auth/logout-all", logout(d.Users, true))
	// CSRF: reject cross-origin unsafe methods (Sec-Fetch-Site / Origin checks).
	return http.NewCrossOriginProtection().Handler(mux)
}

// home greets the signed-in user by first name, or offers Google sign-in.
func home(svc *users.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, signedIn, err := currentUser(r, svc)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Algo deu errado. Tente novamente em instantes.")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = homePage(signedIn, firstName(u.Name)).Render(r.Context(), w)
	}
}

func firstName(name string) string {
	if f := strings.Fields(name); len(f) > 0 {
		return f[0]
	}
	return ""
}

// staticHandler serves only files embedded under web/static.
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err) // "static" is embedded at compile time; failure is a build bug
	}
	return http.StripPrefix("/static/", http.FileServerFS(sub))
}

// healthz answers 200 "ok" when SQLite runs SELECT 1, else 503.
func healthz(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthzTimeout)
		defer cancel()
		var one int
		if err := conn.QueryRowContext(ctx, "SELECT 1").Scan(&one); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	}
}
