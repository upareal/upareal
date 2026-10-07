// story: e01s01

// Package web renders all HTML pages and HTMX fragments and wires the HTTP routes.
package web

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/a-h/templ"
)

const healthzTimeout = 2 * time.Second

//go:embed static
var staticFiles embed.FS

// NewHandler returns the app's HTTP handler. Modules are passed in; web owns no data.
//
//	srv := &http.Server{Handler: web.NewHandler(conn)}
func NewHandler(conn *sql.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(conn))
	mux.Handle("GET /static/", staticHandler())
	mux.Handle("GET /{$}", templ.Handler(homePage()))
	return mux
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
