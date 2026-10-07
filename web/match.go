// story: e07s03
package web

import (
	"errors"
	"net/http"

	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

type matchView struct {
	TxID, Conta, Outra, OutraName, Desc, Amount string
	Candidates                                  []queueLineView
	Error                                       string
}

// markTransfer turns a queued line into a transfer: directly when the other
// account has no matching line, otherwise via a confirmation page.
func markTransfer(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id, conta, outra := r.PathValue("id"), r.URL.Query().Get("conta"), r.FormValue("outra")
		candidates, err := ledg.TransferCandidates(r.Context(), u.ID, id, outra)
		if err != nil {
			status, msg := editError(err)
			renderEdit(w, r, ledg, u.ID, id, conta, nil, status, msg)
			return
		}
		if len(candidates) == 0 {
			if err := ledg.MarkTransfer(r.Context(), u.ID, id, outra, ""); err != nil {
				status, msg := editError(err)
				renderEdit(w, r, ledg, u.ID, id, conta, nil, status, msg)
				return
			}
			http.Redirect(w, r, "/revisao", http.StatusSeeOther)
			return
		}
		renderMatch(w, r, ledg, u.ID, id, conta, outra, candidates, http.StatusOK, "")
	})
}

func confirmTransfer(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id, conta, outra := r.PathValue("id"), r.URL.Query().Get("conta"), r.FormValue("outra")
		err := ledg.MarkTransfer(r.Context(), u.ID, id, outra, r.FormValue("par"))
		if errors.Is(err, ledger.ErrInvalidMatch) {
			candidates, _ := ledg.TransferCandidates(r.Context(), u.ID, id, outra)
			renderMatch(w, r, ledg, u.ID, id, conta, outra, candidates, http.StatusBadRequest, "Correspondente inválido. Escolha outro ou \"Não juntar\".")
			return
		}
		if err != nil {
			status, msg := editError(err)
			renderEdit(w, r, ledg, u.ID, id, conta, nil, status, msg)
			return
		}
		http.Redirect(w, r, "/revisao", http.StatusSeeOther)
	})
}

func renderMatch(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID, id, conta, outra string, candidates []ledger.QueuedLine, status int, errMsg string) {
	tx, err := ledg.Transaction(r.Context(), userID, id)
	if err != nil {
		st, msg := editError(err)
		renderError(w, r, st, "Lançamento", msg)
		return
	}
	other, _, err := findAnyAccount(r, ledg, userID, outra)
	if err != nil {
		renderError(w, r, http.StatusNotFound, "Conta", "Conta não encontrada.")
		return
	}
	v := matchView{TxID: id, Conta: conta, Outra: outra, OutraName: other.Name, Desc: orNoDescription(tx.Description),
		Amount: money.FormatBRL(legAmount(tx, conta)), Error: errMsg}
	for _, c := range candidates {
		v.Candidates = append(v.Candidates, queueLineView{ID: c.TransactionID, When: c.At.In(saoPaulo).Format("02/01/2006 15:04"),
			Description: orNoDescription(c.Description), Amount: money.FormatBRL(c.Amount)})
	}
	renderHTML(w, r, status, matchPage(v))
}
