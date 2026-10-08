// story: e09s01
package web

import (
	"fmt"
	"net/http"

	"github.com/upareal/upareal/users"
)

type adminUserView struct {
	Name, Email, CreatedAt, LastSeenAt string
	Admin                              bool
}

// adminUsers lists users for admins. Anyone else gets the same 404 as an
// unknown URL, so the page's existence is not revealed.
func adminUsers(svc *users.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		if !u.Admin {
			http.NotFound(w, r)
			return
		}
		list, err := svc.ListUsers(r.Context())
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar os usuários.")
			return
		}
		rows := make([]adminUserView, 0, len(list))
		for _, s := range list {
			v := adminUserView{Name: s.Name, Email: s.Email, CreatedAt: s.CreatedAt.In(saoPaulo).Format("02/01/2006"), LastSeenAt: "—", Admin: s.Admin}
			if !s.LastSeenAt.IsZero() {
				v.LastSeenAt = s.LastSeenAt.In(saoPaulo).Format("02/01/2006")
			}
			rows = append(rows, v)
		}
		renderHTML(w, r, http.StatusOK, adminUsersPage(fmt.Sprintf("%d usuários", len(rows)), rows))
	})
}
