// story: e05s01
package web

import (
	"net/http"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/users"
)

func categories(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		groups, err := bud.Categories(r.Context(), u.ID)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as categorias.")
			return
		}
		renderHTML(w, r, http.StatusOK, categoriesPage(groups))
	})
}
