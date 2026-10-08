// story: e09s01 e09s02
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/upareal/upareal/users"
)

type adminUserView struct {
	ID, Name, Email, CreatedAt, LastSeenAt string
	Admin, Disabled                        bool
}

// adminOnly runs render for admins. Anyone else gets the same 404 as an
// unknown URL, so admin pages are not revealed.
func adminOnly(svc *users.Service, render func(w http.ResponseWriter, r *http.Request, admin users.User)) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		if !u.Admin {
			http.NotFound(w, r)
			return
		}
		render(w, r, u)
	})
}

func adminUsers(svc *users.Service) http.HandlerFunc {
	return adminOnly(svc, func(w http.ResponseWriter, r *http.Request, _ users.User) {
		renderAdminUsers(w, r, svc, http.StatusOK, "")
	})
}

// adminUserAction disables, re-enables or deletes a user (e09s02). Deleting
// purges planning, budget and ledger before the user row.
func adminUserAction(d Deps, action string) http.HandlerFunc {
	return adminOnly(d.Users, func(w http.ResponseWriter, r *http.Request, admin users.User) {
		id := r.PathValue("id")
		var err error
		switch action {
		case "desativar":
			err = d.Users.DisableUser(r.Context(), id)
		case "reativar":
			err = d.Users.EnableUser(r.Context(), id)
		case "excluir":
			if r.FormValue("confirmo") != "on" {
				renderAdminUsers(w, r, d.Users, http.StatusBadRequest, "Marque a caixa para confirmar a exclusão.")
				return
			}
			err = d.Users.DeleteUser(r.Context(), id, purgeUserData(d))
		}
		switch {
		case err == nil:
			slog.InfoContext(r.Context(), "admin action", "action", action, "admin_id", admin.ID, "user_id", id)
			http.Redirect(w, r, "/admin/usuarios", http.StatusSeeOther)
		case errors.Is(err, users.ErrProtectedUser):
			renderAdminUsers(w, r, d.Users, http.StatusBadRequest, "Administradores não podem ser desativados nem excluídos.")
		case errors.Is(err, users.ErrNotFound):
			renderError(w, r, http.StatusNotFound, "Usuário", "Usuário não encontrado.")
		default:
			slog.ErrorContext(r.Context(), "admin action failed", "action", action, "admin_id", admin.ID, "user_id", id, "err", err)
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível concluir. Tente de novo.")
		}
	})
}

// purgeUserData deletes a user's data in every module, joining module first.
func purgeUserData(d Deps) func(context.Context, string) error {
	return func(ctx context.Context, userID string) error {
		if err := d.Planning.DeleteUserData(ctx, userID); err != nil {
			return err
		}
		if err := d.Budget.DeleteUserData(ctx, userID); err != nil {
			return err
		}
		return d.Ledger.DeleteUserData(ctx, userID)
	}
}

func renderAdminUsers(w http.ResponseWriter, r *http.Request, svc *users.Service, status int, errMsg string) {
	list, err := svc.ListUsers(r.Context())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar os usuários.")
		return
	}
	rows := make([]adminUserView, 0, len(list))
	for _, s := range list {
		v := adminUserView{ID: s.ID, Name: s.Name, Email: s.Email, CreatedAt: s.CreatedAt.In(saoPaulo).Format("02/01/2006"),
			LastSeenAt: "—", Admin: s.Admin, Disabled: s.Disabled}
		if !s.LastSeenAt.IsZero() {
			v.LastSeenAt = s.LastSeenAt.In(saoPaulo).Format("02/01/2006")
		}
		rows = append(rows, v)
	}
	renderHTML(w, r, status, adminUsersPage(fmt.Sprintf("%d usuários", len(rows)), rows, errMsg))
}
