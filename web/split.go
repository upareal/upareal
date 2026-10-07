// story: e06s02
package web

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

const splitStartLines = 3

type splitView struct {
	AccountID, AccountName  string
	Total, Date, Time, Desc string
	Lines                   []splitLineView
	Expenses, Incomes       []ledger.Account
	Remaining               string
}

type splitLineView struct{ CounterID, Amount string }

func splitGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		now := time.Now().In(saoPaulo)
		v := splitView{Date: now.Format("2006-01-02"), Time: now.Format("15:04"), Lines: make([]splitLineView, splitStartLines)}
		renderSplit(w, r, ledg, u.ID, r.PathValue("id"), v, http.StatusOK, "")
	})
}

// splitLine is the HTMX fragment for one more empty line.
func splitLine(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		_, expenses, incomes, err := splitAccounts(r, ledg, u.ID, r.PathValue("id"))
		if err != nil {
			st, msg := ledgerError(err, "")
			http.Error(w, msg, st)
			return
		}
		renderHTML(w, r, http.StatusOK, splitLineRow(splitLineView{}, expenses, incomes))
	})
}

// splitRemaining is the HTMX fragment "Falta distribuir R$ X".
func splitRemaining(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		total, parts, err := parseSplit(r)
		msg := "Falta distribuir —"
		if err == nil {
			if rem, err := ledg.SplitRemaining(r.Context(), u.ID, total, parts); err == nil {
				msg = remainingText(rem)
			}
		}
		renderHTML(w, r, http.StatusOK, splitRemainingText(msg))
	})
}

func splitPost(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		v := splitView{Total: r.FormValue("total"), Date: r.FormValue("data"), Time: r.FormValue("hora"), Desc: r.FormValue("descricao")}
		for i, c := range r.Form["contra"] {
			v.Lines = append(v.Lines, splitLineView{CounterID: c, Amount: formAt(r.Form["parte"], i)})
		}
		at, err := time.ParseInLocation("2006-01-02 15:04", v.Date+" "+v.Time, saoPaulo)
		if err != nil {
			renderSplit(w, r, ledg, u.ID, id, v, http.StatusBadRequest, "Data ou hora inválida.")
			return
		}
		total, parts, err := parseSplit(r)
		if err != nil {
			renderSplit(w, r, ledg, u.ID, id, v, http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56 (maior que zero).")
			return
		}
		if len(parts) < 2 {
			renderSplit(w, r, ledg, u.ID, id, v, http.StatusBadRequest, "Informe pelo menos duas linhas.")
			return
		}
		_, err = ledg.RecordSplit(r.Context(), u.ID, ledger.SplitEntry{AccountID: id, Total: total, At: at, Description: v.Desc, Parts: parts})
		var mm ledger.SplitMismatch
		switch {
		case err == nil:
			http.Redirect(w, r, "/contas/"+id+"/lancamentos?mes="+url.QueryEscape(budget.MonthOf(at).String()), http.StatusSeeOther)
		case errors.As(err, &mm):
			renderSplit(w, r, ledg, u.ID, id, v, http.StatusBadRequest, remainingText(mm.Remaining))
		default:
			status, msg := entryError(err)
			renderSplit(w, r, ledg, u.ID, id, v, status, msg)
		}
	})
}

// parseSplit reads total and the non-empty lines; a half-filled line is an error.
func parseSplit(r *http.Request) (int64, []ledger.Part, error) {
	if err := r.ParseForm(); err != nil {
		return 0, nil, err
	}
	total, err := money.ParseBRL(r.FormValue("total"))
	if err != nil || total <= 0 {
		return 0, nil, errors.New("invalid total")
	}
	var parts []ledger.Part
	for i, c := range r.Form["contra"] {
		a := formAt(r.Form["parte"], i)
		if c == "" && a == "" {
			continue // spare empty row
		}
		amount, err := money.ParseBRL(a)
		if c == "" || err != nil || amount <= 0 {
			return 0, nil, errors.New("invalid line")
		}
		parts = append(parts, ledger.Part{CounterID: c, Amount: amount})
	}
	return total, parts, nil
}

func renderSplit(w http.ResponseWriter, r *http.Request, ledg *ledger.Service, userID, id string, v splitView, status int, errMsg string) {
	acc, expenses, incomes, err := splitAccounts(r, ledg, userID, id)
	if err != nil {
		st, msg := ledgerError(err, "")
		renderError(w, r, st, "Conta", msg)
		return
	}
	v.AccountID, v.AccountName, v.Expenses, v.Incomes = acc.ID, acc.Name, expenses, incomes
	for len(v.Lines) < 2 {
		v.Lines = append(v.Lines, splitLineView{})
	}
	if v.Remaining == "" {
		v.Remaining = "Falta distribuir —"
	}
	renderHTML(w, r, status, splitPage(v, errMsg))
}

// splitAccounts returns the paying account and the pickable user expense/income accounts.
func splitAccounts(r *http.Request, ledg *ledger.Service, userID, id string) (ledger.Account, []ledger.Account, []ledger.Account, error) {
	acc, all, err := findAnyAccount(r, ledg, userID, id)
	if err != nil {
		return ledger.Account{}, nil, nil, err
	}
	if acc.Type != "asset" && acc.Type != "liability" {
		return ledger.Account{}, nil, nil, ledger.ErrAccountNotFound
	}
	var expenses, incomes []ledger.Account
	for _, a := range all {
		if a.SystemRole != "" {
			continue
		}
		switch a.Type {
		case "expense":
			expenses = append(expenses, a)
		case "income":
			incomes = append(incomes, a)
		}
	}
	return acc, expenses, incomes, nil
}

func remainingText(rem int64) string {
	if rem < 0 {
		return "Distribuído a mais " + money.FormatBRL(-rem)
	}
	return "Falta distribuir " + money.FormatBRL(rem)
}

func formAt(values []string, i int) string {
	if i < len(values) {
		return values[i]
	}
	return ""
}
