// story: e04s02
package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

// saoPaulo is where days and months are counted (time/tzdata is embedded by cmd/upareal).
var saoPaulo = mustLoadLocation("America/Sao_Paulo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // zone data is embedded; failure is a build bug
	}
	return loc
}

// accountSection is one type group on the accounts page.
type accountSection struct {
	Title      string
	Adjustable bool // asset/liability show balances and the adjust form
	Rows       []accountView
}

type accountView struct {
	ID, Name, Type, Balance string
	OffBudget, IsCard       bool
}

var sectionOrder = []struct{ typ, title string }{
	{"asset", "Contas e dinheiro"}, {"liability", "Cartões e dívidas"},
	{"income", "Receitas"}, {"expense", "Despesas"},
}

func accountsGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		renderAccounts(w, r, ledg, u.ID, http.StatusOK, "")
	})
}

func accountsCreate(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		n := ledger.NewAccount{
			Name: r.FormValue("nome"), Type: r.FormValue("tipo"),
			OnBudget: r.FormValue("orcamento") == "on", IsCard: r.FormValue("cartao") == "on",
		}
		var err error
		if n.Opening, err = parseOptionalBRL(r.FormValue("saldo")); err != nil {
			renderAccounts(w, r, ledg, u.ID, http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56.")
			return
		}
		if n.Type == "liability" {
			n.Opening = -n.Opening // people type what they owe; the ledger stores debit-positive
		}
		if n.OpeningAt, err = parseDate(r.FormValue("data")); err != nil {
			renderAccounts(w, r, ledg, u.ID, http.StatusBadRequest, "Data inválida.")
			return
		}
		if _, err := ledg.CreateAccount(r.Context(), u.ID, n); err != nil {
			status, msg := ledgerError(err, "Não foi possível criar a conta. Verifique o nome (até 100 caracteres) e o tipo.")
			renderAccounts(w, r, ledg, u.ID, status, msg)
			return
		}
		http.Redirect(w, r, "/contas", http.StatusSeeOther)
	})
}

func accountsAdjust(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		target, err := money.ParseBRL(r.FormValue("saldo"))
		if err != nil {
			renderAccounts(w, r, ledg, u.ID, http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56.")
			return
		}
		id := r.PathValue("id")
		accountType, err := accountTypeOf(r, ledg, u.ID, id)
		if err != nil {
			status, msg := ledgerError(err, "Não foi possível ajustar o saldo.")
			renderAccounts(w, r, ledg, u.ID, status, msg)
			return
		}
		if accountType == "liability" {
			target = -target // people type what they owe; the ledger stores debit-positive
		}
		if err := ledg.AdjustBalance(r.Context(), u.ID, id, target, time.Now()); err != nil {
			status, msg := ledgerError(err, "Não foi possível ajustar o saldo.")
			renderAccounts(w, r, ledg, u.ID, status, msg)
			return
		}
		http.Redirect(w, r, "/contas", http.StatusSeeOther)
	})
}

func renderAccounts(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID string, status int, errMsg string) {
	accounts, err := ledg.Accounts(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as contas.")
		return
	}
	balances, err := ledg.Balances(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar os saldos.")
		return
	}
	sections := make([]accountSection, 0, len(sectionOrder))
	for _, so := range sectionOrder {
		sec := accountSection{Title: so.title, Adjustable: so.typ == "asset" || so.typ == "liability"}
		for _, a := range accounts {
			if a.Type != so.typ || a.SystemRole != "" {
				continue
			}
			sec.Rows = append(sec.Rows, accountView{
				ID: a.ID, Name: a.Name, Type: a.Type, Balance: displayBalance(a.Type, balances[a.ID]),
				OffBudget: sec.Adjustable && !a.OnBudget, IsCard: a.IsCard,
			})
		}
		sections = append(sections, sec)
	}
	renderHTML(w, r, status, accountsPage(sections, errMsg, time.Now().In(saoPaulo).Format("2006-01-02")))
}

// displayBalance turns a stored debit-positive balance into what people read:
// debts as amounts owed, income as amounts earned.
// ponytail: negating math.MinInt64 overflows; unreachable for real balances.
func displayBalance(accountType string, stored int64) string {
	switch accountType {
	case "income":
		return money.FormatBRL(-stored)
	case "liability":
		switch owed := -stored; {
		case owed > 0:
			return money.FormatBRL(owed) + " a pagar"
		case owed < 0:
			return money.FormatBRL(-owed) + " a receber"
		}
		return money.FormatBRL(0)
	}
	return money.FormatBRL(stored)
}

// accountTypeOf finds the type of one of userID's accounts on the server, so the
// sign of an adjustment never depends on client input.
func accountTypeOf(r *http.Request, ledg *ledger.Service, userID, accountID string) (string, error) {
	accounts, err := ledg.Accounts(r.Context(), userID)
	if err != nil {
		return "", err
	}
	for _, a := range accounts {
		if a.ID == accountID {
			return a.Type, nil
		}
	}
	return "", ledger.ErrAccountNotFound
}

func parseOptionalBRL(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return money.ParseBRL(s)
}

// parseDate reads an HTML date (yyyy-mm-dd) as midnight in São Paulo; empty means now.
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	return time.ParseInLocation("2006-01-02", s, saoPaulo)
}

// ledgerError maps ledger errors to an HTTP status and a pt-BR message.
func ledgerError(err error, invalidMsg string) (int, string) {
	switch {
	case errors.Is(err, ledger.ErrInvalidAccount):
		return http.StatusBadRequest, invalidMsg
	case errors.Is(err, ledger.ErrAccountNotFound):
		return http.StatusNotFound, "Conta não encontrada."
	}
	return http.StatusInternalServerError, "Algo deu errado. Tente novamente em instantes."
}

// signedInPage is appPage for pages that need a signed-in user; others go home.
func signedInPage(svc *users.Service, render func(w http.ResponseWriter, r *http.Request, u users.User)) http.HandlerFunc {
	return appPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User, signedIn bool) {
		if !signedIn {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		render(w, r, u)
	})
}
