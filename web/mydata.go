// story: e10s01 e10s02
package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

// myDataExport is the downloadable file: every module's data of one user.
type myDataExport struct {
	ExportedAt string          `json:"exported_at"`
	Users      users.Export    `json:"users"`
	Ledger     ledger.Export   `json:"ledger"`
	Budget     budget.Export   `json:"budget"`
	Planning   planning.Export `json:"planning"`
}

func myDataPage(svc *users.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, _ users.User) {
		renderHTML(w, r, http.StatusOK, myDataTemplate(""))
	})
}

// myDataDownload builds the whole export in memory, so an error never sends
// a partial file (LGPD access and portability).
func myDataDownload(d Deps) http.HandlerFunc {
	return signedInPage(d.Users, func(w http.ResponseWriter, r *http.Request, u users.User) {
		now := time.Now()
		ex := myDataExport{ExportedAt: now.UTC().Format(time.RFC3339)}
		var err error
		if ex.Users, err = d.Users.ExportUserData(r.Context(), u.ID); err == nil {
			if ex.Ledger, err = d.Ledger.ExportUserData(r.Context(), u.ID); err == nil {
				if ex.Budget, err = d.Budget.ExportUserData(r.Context(), u.ID); err == nil {
					ex.Planning, err = d.Planning.ExportUserData(r.Context(), u.ID)
				}
			}
		}
		var buf bytes.Buffer
		if err == nil {
			enc := json.NewEncoder(&buf)
			enc.SetIndent("", "  ")
			err = enc.Encode(ex)
		}
		if err != nil {
			slog.ErrorContext(r.Context(), "export my data failed", "user_id", u.ID, "err", err)
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível gerar o arquivo. Tente de novo.")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="upareal-dados-`+now.In(saoPaulo).Format("2006-01-02")+`.json"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(buf.Bytes())
		slog.InfoContext(r.Context(), "exported my data", "user_id", u.ID)
	})
}

// deleteMyAccount erases the signed-in user everywhere after they type their
// email (LGPD art. 18, VI). The ID comes from the session, never the form.
func deleteMyAccount(d Deps) http.HandlerFunc {
	return signedInPage(d.Users, func(w http.ResponseWriter, r *http.Request, u users.User) {
		if !strings.EqualFold(strings.TrimSpace(r.FormValue("confirmar")), u.Email) {
			renderHTML(w, r, http.StatusBadRequest, myDataTemplate("Digite seu e-mail exatamente como aparece para confirmar."))
			return
		}
		err := d.Users.DeleteUser(r.Context(), u.ID, purgeUserData(d))
		switch {
		case errors.Is(err, users.ErrProtectedUser):
			renderHTML(w, r, http.StatusBadRequest, myDataTemplate("Administradores não podem excluir a própria conta. Remova seu e-mail de ADMIN_EMAILS antes."))
		case err != nil:
			slog.ErrorContext(r.Context(), "delete own account failed", "user_id", u.ID, "err", err)
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível excluir agora. Tente de novo: nada foi perdido pela metade.")
		default:
			clearCookie(w, sessionCookie, "/")
			slog.InfoContext(r.Context(), "user deleted own account", "user_id", u.ID)
			renderHTML(w, r, http.StatusOK, accountDeletedPage())
		}
	})
}
