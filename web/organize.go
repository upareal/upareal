// story: e04s03
package web

import (
	"net/http"

	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

func accountEditGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		a, err := activeAccount(r, ledg, u.ID, r.PathValue("id"))
		if err != nil {
			status, msg := ledgerError(err, "")
			renderError(w, r, status, "Conta", msg)
			return
		}
		renderHTML(w, r, http.StatusOK, accountEditPage(a, ""))
	})
}

func accountEditPost(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		upd := ledger.AccountUpdate{
			Name: r.FormValue("nome"), Type: r.FormValue("tipo"),
			OnBudget: r.FormValue("orcamento") == "on", IsCard: r.FormValue("cartao") == "on",
		}
		if _, err := ledg.UpdateAccount(r.Context(), u.ID, id, upd); err != nil {
			status, msg := ledgerError(err, "Não foi possível salvar. Verifique o nome (até 100 caracteres) e o tipo.")
			a, findErr := activeAccount(r, ledg, u.ID, id)
			if findErr != nil {
				renderError(w, r, status, "Conta", msg)
				return
			}
			renderHTML(w, r, status, accountEditPage(a, msg))
			return
		}
		http.Redirect(w, r, "/contas", http.StatusSeeOther)
	})
}

// accountAction runs a one-click account change, then shows the accounts page.
func accountAction(svc *users.Service, ledg *ledger.Service, do func(r *http.Request, userID, id string) error) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		if err := do(r, u.ID, r.PathValue("id")); err != nil {
			status, msg := ledgerError(err, "Não foi possível concluir a ação.")
			renderAccounts(w, r, ledg, u.ID, status, msg)
			return
		}
		http.Redirect(w, r, "/contas", http.StatusSeeOther)
	})
}

// activeAccount finds one of userID's active, user-editable accounts.
func activeAccount(r *http.Request, ledg *ledger.Service, userID, id string) (accountView, error) {
	accounts, err := ledg.Accounts(r.Context(), userID)
	if err != nil {
		return accountView{}, err
	}
	for _, a := range accounts {
		if a.ID == id && a.SystemRole == "" {
			return accountView{ID: a.ID, Name: a.Name, Type: a.Type, OffBudget: !a.OnBudget, IsCard: a.IsCard}, nil
		}
	}
	return accountView{}, ledger.ErrAccountNotFound
}
