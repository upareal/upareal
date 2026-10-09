// story: e01s01 e03s01 e03s02 e03s03 e04s01 e04s02 e04s03 e05s01 e05s02 e05s03 e06s01 e06s02 e06s03 e06s04 e06s05 e07s01 e07s02 e07s03 e08s01 e08s02 e08s06 e09s01 e09s02 e10s01 e10s02 e13s01 e12s03 e14s01

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

	"github.com/a-h/templ"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

const healthzTimeout = 2 * time.Second

//go:embed static
var staticFiles embed.FS

// Deps are the modules the web layer renders. web owns no data.
type Deps struct {
	DB       *sql.DB
	Users    *users.Service
	Ledger   *ledger.Service
	Budget   *budget.Service
	Planning *planning.Service
	Google   *users.Google
}

// NewHandler returns the app's HTTP handler.
//
//	srv := &http.Server{Handler: web.NewHandler(web.Deps{DB: conn, Users: svc, Google: g})}
func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz(d.DB))
	mux.Handle("GET /static/", staticHandler())
	mux.HandleFunc("GET /favicon.ico", favicon) // browsers request it at the root
	mux.HandleFunc("GET /{$}", home(d.Users))
	mux.HandleFunc("GET /planejamento/mapa", mappingGet(d.Users, d.Planning, d.Budget))
	mux.HandleFunc("POST /planejamento/mapa/{conta}", mappingPost(d.Users, d.Planning, d.Budget))
	mux.HandleFunc("GET /conta/dados", myDataPage(d.Users))
	mux.HandleFunc("GET /conta/dados/exportar", myDataDownload(d))
	mux.HandleFunc("POST /conta/excluir", deleteMyAccount(d))
	mux.HandleFunc("GET /conta/dados/ia", aiExport(d))
	mux.HandleFunc("GET /admin/usuarios", adminUsers(d.Users))
	for _, action := range []string{"desativar", "reativar", "excluir"} {
		mux.HandleFunc("POST /admin/usuarios/{id}/"+action, adminUserAction(d, action))
	}
	mux.HandleFunc("GET /orcamento", budgetMonthGet(d.Users, d.Budget, d.Planning))
	mux.HandleFunc("POST /orcamento/{mes}/{categoria}", budgetMonthSet(d.Users, d.Budget, d.Planning))
	mux.HandleFunc("GET /categorias", categories(d.Users, d.Budget))
	mux.HandleFunc("POST /categorias", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		_, err := d.Budget.CreateCategory(r.Context(), userID, r.FormValue("grupo"), r.FormValue("nome"))
		return err
	}))
	mux.HandleFunc("GET /categorias/{id}", categoryEditGet(d.Users, d.Budget))
	mux.HandleFunc("POST /categorias/{id}", categoryEditPost(d.Users, d.Budget))
	mux.HandleFunc("POST /categorias/{id}/mover", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		return d.Budget.MoveCategory(r.Context(), userID, r.PathValue("id"), r.URL.Query().Get("dir") == "up")
	}))
	mux.HandleFunc("POST /categorias/{id}/juntar", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		return d.Planning.MergeCategory(r.Context(), userID, r.PathValue("id"), r.FormValue("destino"))
	}))
	mux.HandleFunc("POST /categorias/{id}/arquivar", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		return d.Planning.ArchiveCategory(r.Context(), userID, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /categorias/{id}/restaurar", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		return d.Budget.RestoreCategory(r.Context(), userID, r.PathValue("id"))
	}))
	mux.HandleFunc("POST /grupos", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		_, err := d.Budget.CreateGroup(r.Context(), userID, r.FormValue("tipo"), r.FormValue("nome"))
		return err
	}))
	mux.HandleFunc("GET /grupos/{id}", groupEditGet(d.Users, d.Budget))
	mux.HandleFunc("POST /grupos/{id}", groupEditPost(d.Users, d.Budget))
	mux.HandleFunc("POST /grupos/{id}/mover", budgetAction(d.Users, d.Budget, func(r *http.Request, userID string) error {
		return d.Budget.MoveGroup(r.Context(), userID, r.PathValue("id"), r.URL.Query().Get("dir") == "up")
	}))
	mux.HandleFunc("GET /contas", accountsGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas", accountsCreate(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/ajuste", accountsAdjust(d.Users, d.Ledger))
	mux.HandleFunc("GET /contas/{id}/lancamentos", registerGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/lancamentos", registerPost(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/transferencia", transferPost(d.Users, d.Ledger))
	mux.HandleFunc("GET /buscar", searchGet(d.Users, d.Ledger))
	mux.HandleFunc("GET /contas/{id}/importar", importGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/importar", importPost(d.Users, d.Ledger, newIPLimiter()))
	mux.HandleFunc("GET /revisao", reviewGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /revisao/{id}", reviewAssign(d.Users, d.Ledger))
	mux.Handle("GET /ajuda/csv", templ.Handler(csvHelpPage()))
	mux.HandleFunc("GET /lancamentos/{id}", editGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /lancamentos/{id}", editPost(d.Users, d.Ledger))
	mux.HandleFunc("POST /lancamentos/{id}/excluir", editDelete(d.Users, d.Ledger))
	mux.HandleFunc("POST /lancamentos/{id}/transferencia", markTransfer(d.Users, d.Ledger))
	mux.HandleFunc("POST /lancamentos/{id}/transferencia/confirmar", confirmTransfer(d.Users, d.Ledger))
	mux.HandleFunc("GET /contas/{id}/dividido", splitGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/dividido", splitPost(d.Users, d.Ledger))
	mux.HandleFunc("GET /contas/{id}/dividido/linha", splitLine(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/dividido/restante", splitRemaining(d.Users, d.Ledger))
	mux.HandleFunc("GET /contas/{id}", accountEditGet(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}", accountEditPost(d.Users, d.Ledger))
	mux.HandleFunc("POST /contas/{id}/mover", accountAction(d.Users, d.Ledger, func(r *http.Request, userID, id string) error {
		return d.Ledger.MoveAccount(r.Context(), userID, id, r.URL.Query().Get("dir") == "up")
	}))
	mux.HandleFunc("POST /contas/{id}/arquivar", accountAction(d.Users, d.Ledger, func(r *http.Request, userID, id string) error {
		return d.Ledger.ArchiveAccount(r.Context(), userID, id)
	}))
	mux.HandleFunc("POST /contas/{id}/restaurar", accountAction(d.Users, d.Ledger, func(r *http.Request, userID, id string) error {
		return d.Ledger.RestoreAccount(r.Context(), userID, id)
	}))
	mux.HandleFunc("POST /contas/{id}/excluir", accountAction(d.Users, d.Ledger, func(r *http.Request, userID, id string) error {
		return d.Ledger.DeleteAccount(r.Context(), userID, id)
	}))
	mux.HandleFunc("GET /estatisticas", publicStats(d))
	mux.Handle("GET /privacidade", templ.Handler(privacyPage()))
	mux.Handle("GET /termos", templ.Handler(termsPage()))
	mux.HandleFunc("GET /termos/aceitar", acceptTermsForm(d.Users))
	mux.HandleFunc("POST /termos/aceitar", acceptTermsSubmit(d.Users))
	authLimit := newIPLimiter()
	mux.Handle("GET /auth/login", authLimit.limit(login(d.Google)))
	mux.Handle("GET /auth/callback", authLimit.limit(callback(d.Users, d.Ledger, d.Budget, d.Google)))
	mux.HandleFunc("POST /auth/logout", logout(d.Users, false))
	mux.HandleFunc("POST /auth/logout-all", logout(d.Users, true))
	// CSRF: reject cross-origin unsafe methods (Sec-Fetch-Site / Origin checks).
	return http.NewCrossOriginProtection().Handler(mux)
}

// home greets the signed-in user by first name, or offers Google sign-in.
func home(svc *users.Service) http.HandlerFunc {
	return appPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User, signedIn bool) {
		renderHTML(w, r, http.StatusOK, homePage(signedIn, firstName(u.Name), u.Admin))
	})
}

// renderHTML writes a templ component with status.
func renderHTML(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = c.Render(r.Context(), w)
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

// favicon serves the embedded icon at /favicon.ico (e12s03).
func favicon(w http.ResponseWriter, r *http.Request) {
	http.ServeFileFS(w, r, staticFiles, "static/favicon.ico")
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
