// story: e05s01 e05s02
package web

import (
	"errors"
	"net/http"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/users"
)

func categories(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderCategories(w, r, bud, u.ID, http.StatusOK, "")
	})
}

func renderCategories(w http.ResponseWriter, r *http.Request, bud *budget.Service, userID string, status int, errMsg string) {
	groups, err := bud.Categories(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as categorias.")
		return
	}
	renderHTML(w, r, status, categoriesPage(groups, errMsg))
}

// budgetAction runs a change from the categories page, then shows it again.
func budgetAction(svc *users.Service, bud *budget.Service, do func(r *http.Request, userID string) error) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		if err := do(r, u.ID); err != nil {
			status, msg := budgetError(err)
			renderCategories(w, r, bud, u.ID, status, msg)
			return
		}
		http.Redirect(w, r, "/categorias", http.StatusSeeOther)
	})
}

func groupEditGet(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		g, err := findGroup(r, bud, u.ID, r.PathValue("id"))
		if err != nil {
			status, msg := budgetError(err)
			renderError(w, r, status, "Grupo", msg)
			return
		}
		renderHTML(w, r, http.StatusOK, groupEditPage(g, ""))
	})
}

func groupEditPost(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		if err := bud.RenameGroup(r.Context(), u.ID, id, r.FormValue("nome")); err != nil {
			status, msg := budgetError(err)
			g, findErr := findGroup(r, bud, u.ID, id)
			if findErr != nil {
				renderError(w, r, status, "Grupo", msg)
				return
			}
			renderHTML(w, r, status, groupEditPage(g, msg))
			return
		}
		http.Redirect(w, r, "/categorias", http.StatusSeeOther)
	})
}

func categoryEditGet(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderCategoryEdit(w, r, bud, u.ID, r.PathValue("id"), http.StatusOK, "")
	})
}

func categoryEditPost(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		err := bud.RenameCategory(r.Context(), u.ID, id, r.FormValue("nome"))
		if err == nil && r.FormValue("grupo") != "" {
			err = bud.SetCategoryGroup(r.Context(), u.ID, id, r.FormValue("grupo"))
		}
		if err != nil {
			status, msg := budgetError(err)
			renderCategoryEdit(w, r, bud, u.ID, id, status, msg)
			return
		}
		http.Redirect(w, r, "/categorias", http.StatusSeeOther)
	})
}

func renderCategoryEdit(w http.ResponseWriter, r *http.Request, bud *budget.Service, userID, id string, status int, errMsg string) {
	groups, err := bud.Categories(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as categorias.")
		return
	}
	for _, g := range groups {
		for _, c := range g.Categories {
			if c.ID != id {
				continue
			}
			var sameKind []budget.Group
			for _, o := range groups {
				if o.Kind == g.Kind {
					sameKind = append(sameKind, o)
				}
			}
			renderHTML(w, r, status, categoryEditPage(c, g, sameKind, errMsg))
			return
		}
	}
	renderError(w, r, http.StatusNotFound, "Categoria", "Categoria não encontrada.")
}

func findGroup(r *http.Request, bud *budget.Service, userID, id string) (budget.Group, error) {
	groups, err := bud.Categories(r.Context(), userID)
	if err != nil {
		return budget.Group{}, err
	}
	for _, g := range groups {
		if g.ID == id {
			return g, nil
		}
	}
	return budget.Group{}, budget.ErrNotFound
}

// budgetError maps budget errors to an HTTP status and a pt-BR message.
func budgetError(err error) (int, string) {
	switch {
	case errors.Is(err, budget.ErrInvalidName):
		return http.StatusBadRequest, "Informe um nome com até 100 caracteres."
	case errors.Is(err, budget.ErrInvalidKind):
		return http.StatusBadRequest, "Escolha Despesas ou Receitas."
	case errors.Is(err, budget.ErrKindMismatch):
		return http.StatusBadRequest, "A categoria só pode ir para um grupo do mesmo tipo."
	case errors.Is(err, budget.ErrNotFound):
		return http.StatusNotFound, "Não encontrado."
	}
	return http.StatusInternalServerError, "Algo deu errado. Tente novamente em instantes."
}
