// story: e04s01
package web

import (
	"net/http"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

func categories(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return appPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User, signedIn bool) {
		if !signedIn {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		groups, err := ledg.Categories(r.Context(), u.ID)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as categorias.")
			return
		}
		renderHTML(w, r, http.StatusOK, categoriesPage(groups))
	})
}
