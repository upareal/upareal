// story: e06s03
package web

import (
	"net/http"
	"net/url"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

func transferPost(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		at, err := time.ParseInLocation("2006-01-02 15:04", r.FormValue("data")+" "+r.FormValue("hora"), saoPaulo)
		if err != nil {
			renderRegister(w, r, ledg, u.ID, id, budget.MonthOf(time.Now()), http.StatusBadRequest, "Data ou hora inválida.")
			return
		}
		m := budget.MonthOf(at)
		amount, err := money.ParseBRL(r.FormValue("valor"))
		if err != nil || amount <= 0 {
			renderRegister(w, r, ledg, u.ID, id, m, http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56 (maior que zero).")
			return
		}
		_, err = ledg.Transfer(r.Context(), u.ID, ledger.TransferEntry{
			FromID: id, ToID: r.FormValue("destino"), Amount: amount, At: at, Description: r.FormValue("descricao"),
		})
		if err != nil {
			status, msg := entryError(err)
			renderRegister(w, r, ledg, u.ID, id, m, status, msg)
			return
		}
		http.Redirect(w, r, "/contas/"+id+"/lancamentos?mes="+url.QueryEscape(m.String()), http.StatusSeeOther)
	})
}
