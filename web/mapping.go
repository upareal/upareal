// story: e08s01 e08s05
package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

type mappingView struct {
	Unmapped string
	Rows     []mappingRowView
	Error    string
}

type mappingRowView struct {
	AccountID, Name, Kind, CategoryID string
	Groups                            []budget.Group // only groups of the account's kind
}

func mappingGet(svc *users.Service, plan *planning.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderMapping(w, r, plan, bud, u.ID, http.StatusOK, "")
	})
}

func mappingPost(svc *users.Service, plan *planning.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		err := plan.SetMapping(r.Context(), u.ID, r.PathValue("conta"), r.FormValue("categoria"))
		switch {
		case err == nil:
			http.Redirect(w, r, "/planejamento/mapa", http.StatusSeeOther)
		case errors.Is(err, planning.ErrKindMismatch):
			renderMapping(w, r, plan, bud, u.ID, http.StatusBadRequest, "A conta só pode ir para uma categoria do mesmo tipo: receitas em receitas; despesas e contas fora do orçamento em despesas.")
		case errors.Is(err, planning.ErrNotMappable), errors.Is(err, planning.ErrNotFound):
			renderMapping(w, r, plan, bud, u.ID, http.StatusNotFound, "Conta ou categoria não encontrada.")
		default:
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível salvar.")
		}
	})
}

func renderMapping(w http.ResponseWriter, r *http.Request, plan *planning.Service, bud *budget.Service, userID string, status int, errMsg string) {
	mappings, err := plan.Mappings(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar o mapa.")
		return
	}
	groups, err := bud.Categories(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as categorias.")
		return
	}
	byKind := map[string][]budget.Group{}
	for _, g := range groups {
		byKind[g.Kind] = append(byKind[g.Kind], g)
	}
	v := mappingView{Error: errMsg}
	unmapped := 0
	for _, m := range mappings {
		if m.CategoryID == "" {
			unmapped++
		}
		v.Rows = append(v.Rows, mappingRowView{AccountID: m.Account.ID, Name: m.Account.Name, Kind: m.Kind,
			CategoryID: m.CategoryID, Groups: byKind[m.Kind]})
	}
	v.Unmapped = fmt.Sprintf("%d sem categoria", unmapped)
	renderHTML(w, r, status, mappingPage(v))
}
