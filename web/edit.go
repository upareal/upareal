// story: e06s04
package web

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

// editView is the edit page of one transaction, seen from account Conta.
type editView struct {
	TxID, Conta, ContaName string
	Shape                  string // "entry", "transfer" or "system"
	Date, Time, Desc       string
	Lines                  []splitLineView
	Expenses, Incomes      []ledger.Account
	Holders                []ledger.Account // asset/liability accounts for transfers
	From, To, Amount       string
	History                []historyView
}

type historyView struct{ When, Action, Desc, Amount string }

func editGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderEdit(w, r, ledg, u.ID, r.PathValue("id"), r.URL.Query().Get("conta"), nil, http.StatusOK, "")
	})
}

func editPost(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id, conta := r.PathValue("id"), r.URL.Query().Get("conta")
		if tx, err := ledg.Transaction(r.Context(), u.ID, id); err != nil || legAmount(tx, conta) == 0 {
			renderError(w, r, http.StatusNotFound, "Lançamento", "Lançamento não encontrado nesta conta.")
			return
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", r.FormValue("data")+" "+r.FormValue("hora"), saoPaulo)
		if err != nil {
			renderEdit(w, r, ledg, u.ID, id, conta, r, http.StatusBadRequest, "Data ou hora inválida.")
			return
		}
		desc := r.FormValue("descricao")
		if r.Form.Has("destino") {
			err = editTransfer(r, ledg, u.ID, id, at, desc)
		} else {
			err = editEntry(r, ledg, u.ID, id, conta, at, desc)
		}
		if err != nil {
			status, msg := editError(err)
			renderEdit(w, r, ledg, u.ID, id, conta, r, status, msg)
			return
		}
		http.Redirect(w, r, "/contas/"+conta+"/lancamentos?mes="+url.QueryEscape(budget.MonthOf(at).String()), http.StatusSeeOther)
	})
}

func editDelete(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id, conta := r.PathValue("id"), r.URL.Query().Get("conta")
		tx, err := ledg.Transaction(r.Context(), u.ID, id)
		if err == nil {
			err = ledg.DeleteTransaction(r.Context(), u.ID, id)
		}
		if err != nil {
			status, msg := editError(err)
			renderError(w, r, status, "Lançamento", msg)
			return
		}
		http.Redirect(w, r, "/contas/"+conta+"/lancamentos?mes="+url.QueryEscape(budget.MonthOf(tx.At).String()), http.StatusSeeOther)
	})
}

// editEntry rebuilds an expense/income transaction from its lines; the total
// is derived from them, so a mismatch is impossible.
func editEntry(r *http.Request, ledg *ledger.Service, userID, id, conta string, at time.Time, desc string) error {
	parts, err := parseLines(r)
	if err != nil {
		return err
	}
	if len(parts) == 0 {
		return ledger.ErrInvalidEntry
	}
	remaining, err := ledg.SplitRemaining(r.Context(), userID, 0, parts)
	if err != nil {
		return err
	}
	return ledg.EditEntry(r.Context(), userID, id, ledger.SplitEntry{AccountID: conta, Total: -remaining, At: at, Description: desc, Parts: parts})
}

func editTransfer(r *http.Request, ledg *ledger.Service, userID, id string, at time.Time, desc string) error {
	amount, err := parsePositiveBRL(r.FormValue("valor"))
	if err != nil {
		return err
	}
	return ledg.EditTransfer(r.Context(), userID, id, ledger.TransferEntry{
		FromID: r.FormValue("origem"), ToID: r.FormValue("destino"), Amount: amount, At: at, Description: desc,
	})
}

// renderEdit shows the edit page; when form is not nil its values are kept.
func renderEdit(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID, id, conta string, form *http.Request, status int, errMsg string) {
	tx, err := ledg.Transaction(r.Context(), userID, id)
	if err != nil {
		st, msg := editError(err)
		renderError(w, r, st, "Lançamento", msg)
		return
	}
	acc, all, err := findAnyAccount(r, ledg, userID, conta)
	if err != nil || legAmount(tx, conta) == 0 {
		renderError(w, r, http.StatusNotFound, "Lançamento", "Lançamento não encontrado nesta conta.")
		return
	}
	v := editView{TxID: tx.ID, Conta: conta, ContaName: acc.Name, Desc: tx.Description,
		Date: tx.At.In(saoPaulo).Format("2006-01-02"), Time: tx.At.In(saoPaulo).Format("15:04"), Shape: txShape(tx, conta)}
	for _, a := range all {
		if a.SystemRole != "" {
			continue
		}
		switch a.Type {
		case "expense":
			v.Expenses = append(v.Expenses, a)
		case "income":
			v.Incomes = append(v.Incomes, a)
		default:
			v.Holders = append(v.Holders, a)
		}
	}
	for _, l := range tx.Legs {
		switch {
		case v.Shape == "transfer" && l.Amount < 0:
			v.From, v.Amount = l.AccountID, brlInput(-l.Amount)
		case v.Shape == "transfer":
			v.To = l.AccountID
		case l.AccountID != conta:
			amount := l.Amount
			if amount < 0 {
				amount = -amount
			}
			v.Lines = append(v.Lines, splitLineView{CounterID: l.AccountID, Amount: brlInput(amount)})
		}
	}
	if form != nil {
		keepFormValues(&v, form)
	}
	history, err := ledg.History(r.Context(), userID, id)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar o histórico.")
		return
	}
	for _, h := range history {
		action := "Editado"
		if h.Action == "delete" {
			action = "Excluído"
		}
		v.History = append(v.History, historyView{When: h.At.In(saoPaulo).Format("02/01/2006 15:04"), Action: action,
			Desc: h.Previous.Description, Amount: displayChange(acc.Type, legAmount(h.Previous, conta))})
	}
	renderHTML(w, r, status, editPage(v, errMsg))
}

// txShape says which form edits tx as seen from conta.
func txShape(tx ledger.Transaction, conta string) string {
	var others []ledger.TxLeg
	for _, l := range tx.Legs {
		if l.SystemRole != "" {
			return "system"
		}
		if l.AccountID != conta {
			others = append(others, l)
		}
	}
	if len(others) == 1 && (others[0].AccountType == "asset" || others[0].AccountType == "liability") {
		return "transfer"
	}
	return "entry"
}

func legAmount(tx ledger.Transaction, accountID string) int64 {
	var sum int64
	for _, l := range tx.Legs {
		if l.AccountID == accountID {
			sum += l.Amount
		}
	}
	return sum
}

func keepFormValues(v *editView, r *http.Request) {
	v.Date, v.Time, v.Desc = r.FormValue("data"), r.FormValue("hora"), r.FormValue("descricao")
	if r.Form.Has("destino") {
		v.From, v.To, v.Amount = r.FormValue("origem"), r.FormValue("destino"), r.FormValue("valor")
		return
	}
	v.Lines = nil
	for i, c := range r.Form["contra"] {
		v.Lines = append(v.Lines, splitLineView{CounterID: c, Amount: formAt(r.Form["parte"], i)})
	}
}

// editError maps edit errors to an HTTP status and a pt-BR message.
func editError(err error) (int, string) {
	var mm ledger.SplitMismatch
	switch {
	case errors.Is(err, ledger.ErrTransactionNotFound):
		return http.StatusNotFound, "Lançamento não encontrado."
	case errors.Is(err, ledger.ErrNotEditable):
		return http.StatusBadRequest, "Este lançamento não pode ser editado; exclua-o ou use Ajustar saldo."
	case errors.As(err, &mm):
		return http.StatusBadRequest, remainingText(mm.Remaining)
	case errors.Is(err, errInvalidAmount), errors.Is(err, ledger.ErrZeroLeg):
		return http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56 (maior que zero)."
	}
	return entryError(err)
}
