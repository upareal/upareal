// story: e06s01
package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

type registerView struct {
	AccountID, AccountName, Month, Label, Prev, Next string
	CanRecord                                        bool // asset/liability accounts take simple entries
	Lines                                            []registerLineView
	Expenses, Incomes                                []ledger.Account
	Today, Now                                       string
}

type registerLineView struct {
	When, Description, Counterparts, Amount, Balance string
}

func registerGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		m := budget.MonthOf(time.Now())
		if s := r.URL.Query().Get("mes"); s != "" {
			var err error
			if m, err = budget.ParseMonth(s); err != nil {
				renderError(w, r, http.StatusBadRequest, "Mês inválido", "Use o formato AAAA-MM.")
				return
			}
		}
		renderRegister(w, r, ledg, u.ID, r.PathValue("id"), m, http.StatusOK, "")
	})
}

func registerPost(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
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
		_, err = ledg.RecordSimple(r.Context(), u.ID, ledger.SimpleEntry{
			AccountID: id, CounterID: r.FormValue("contra"), Amount: amount, At: at, Description: r.FormValue("descricao"),
		})
		if err != nil {
			status, msg := entryError(err)
			renderRegister(w, r, ledg, u.ID, id, m, status, msg)
			return
		}
		http.Redirect(w, r, "/contas/"+id+"/lancamentos?mes="+url.QueryEscape(m.String()), http.StatusSeeOther)
	})
}

func renderRegister(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID, id string, m budget.Month, status int, errMsg string) {
	acc, all, err := findAnyAccount(r, ledg, userID, id)
	if err != nil {
		st, msg := ledgerError(err, "")
		renderError(w, r, st, "Conta", msg)
		return
	}
	lines, err := ledg.Register(r.Context(), userID, id, m.Start(), m.Next().Start())
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar os lançamentos.")
		return
	}
	start, now := m.Start(), time.Now().In(saoPaulo)
	v := registerView{AccountID: acc.ID, AccountName: acc.Name, Month: m.String(),
		Label: fmt.Sprintf("%s de %d", monthNames[start.Month()-1], start.Year()), Prev: m.Prev().String(), Next: m.Next().String(),
		CanRecord: acc.Type == "asset" || acc.Type == "liability", Today: now.Format("2006-01-02"), Now: now.Format("15:04")}
	for _, l := range lines {
		v.Lines = append(v.Lines, registerLineView{
			When: l.At.In(saoPaulo).Format("02/01 15:04"), Description: l.Description,
			Counterparts: strings.Join(l.Counterparts, ", "),
			Amount:       displayChange(acc.Type, l.Amount), Balance: displayBalance(acc.Type, l.Balance),
		})
	}
	for _, a := range all {
		if a.SystemRole != "" {
			continue // "A categorizar" is reserved for imports
		}
		switch a.Type {
		case "expense":
			v.Expenses = append(v.Expenses, a)
		case "income":
			v.Incomes = append(v.Incomes, a)
		}
	}
	renderHTML(w, r, status, registerPage(v, errMsg))
}

// displayChange shows a debit-positive change the way the account type reads:
// money leaving an asset is negative; a card purchase increases what is owed.
func displayChange(accountType string, amount int64) string {
	if accountType == "liability" || accountType == "income" {
		return money.FormatBRL(-amount)
	}
	return money.FormatBRL(amount)
}

// findAnyAccount finds one of userID's accounts (active or archived) and
// returns the active accounts for pickers.
func findAnyAccount(r *http.Request, ledg *ledger.Service, userID, id string) (ledger.Account, []ledger.Account, error) {
	active, err := ledg.Accounts(r.Context(), userID)
	if err != nil {
		return ledger.Account{}, nil, err
	}
	archived, err := ledg.ArchivedAccounts(r.Context(), userID)
	if err != nil {
		return ledger.Account{}, nil, err
	}
	for _, a := range append(active, archived...) {
		if a.ID == id && a.SystemRole == "" {
			return a, active, nil
		}
	}
	return ledger.Account{}, nil, ledger.ErrAccountNotFound
}

// entryError maps entry errors to an HTTP status and a pt-BR message.
func entryError(err error) (int, string) {
	switch {
	case errors.Is(err, ledger.ErrInvalidEntry):
		return http.StatusBadRequest, "Lançamento inválido. Verifique a conta, o valor e a descrição (até 200 caracteres)."
	case errors.Is(err, ledger.ErrAccountNotPostable):
		return http.StatusBadRequest, "Esta conta não aceita lançamentos (arquivada?)."
	case errors.Is(err, ledger.ErrAccountNotFound):
		return http.StatusNotFound, "Conta não encontrada."
	}
	return http.StatusInternalServerError, "Algo deu errado. Tente novamente em instantes."
}
