// story: e06s05
package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/users"
)

type searchView struct {
	From, To, Account, Min, Max, Text string
	Accounts                          []ledger.Account
	Results                           []searchResultView
	Summary                           string
	Truncated                         bool
}

type searchResultView struct{ Link, When, Description, Accounts, Amount string }

func searchGet(svc *users.Service, ledg *ledger.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		qs := r.URL.Query()
		now := time.Now().In(saoPaulo)
		v := searchView{From: qs.Get("de"), To: qs.Get("ate"), Account: qs.Get("conta"), Min: qs.Get("min"), Max: qs.Get("max"), Text: qs.Get("texto")}
		if v.From == "" {
			v.From = now.AddDate(-1, 0, 0).Format("2006-01-02")
		}
		if v.To == "" {
			v.To = now.Format("2006-01-02")
		}
		accounts, err := ledg.Accounts(r.Context(), u.ID)
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as contas.")
			return
		}
		for _, a := range accounts {
			if a.SystemRole == "" {
				v.Accounts = append(v.Accounts, a)
			}
		}
		q, err := searchQuery(v)
		if err != nil {
			renderHTML(w, r, http.StatusBadRequest, searchPage(v, "Filtros inválidos. Use datas válidas (início antes do fim, até 5 anos) e valores no formato 1.234,56."))
			return
		}
		results, truncated, err := ledg.Search(r.Context(), u.ID, q)
		if errors.Is(err, ledger.ErrInvalidSearch) {
			renderHTML(w, r, http.StatusBadRequest, searchPage(v, "Filtros inválidos. Use datas válidas (início antes do fim, até 5 anos) e valores no formato 1.234,56."))
			return
		}
		if err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível buscar.")
			return
		}
		var total int64
		for _, res := range results {
			total += res.Amount
			v.Results = append(v.Results, searchResultView{
				Link: "/lancamentos/" + res.TransactionID + "?conta=" + res.LinkAccountID,
				When: res.At.In(saoPaulo).Format("02/01/2006 15:04"), Description: orNoDescription(res.Description),
				Accounts: strings.Join(res.Accounts, ", "), Amount: money.FormatBRL(res.Amount),
			})
		}
		plural := "s"
		if len(results) == 1 {
			plural = ""
		}
		v.Summary = fmt.Sprintf("%d resultado%s · Total %s", len(results), plural, money.FormatBRL(total))
		v.Truncated = truncated
		renderHTML(w, r, http.StatusOK, searchPage(v, ""))
	})
}

// searchQuery turns the form into a query: dates are whole days in São Paulo
// (the end date is inclusive).
func searchQuery(v searchView) (ledger.SearchQuery, error) {
	from, err := time.ParseInLocation("2006-01-02", v.From, saoPaulo)
	if err != nil {
		return ledger.SearchQuery{}, err
	}
	to, err := time.ParseInLocation("2006-01-02", v.To, saoPaulo)
	if err != nil {
		return ledger.SearchQuery{}, err
	}
	q := ledger.SearchQuery{From: from, To: to.AddDate(0, 0, 1), AccountID: v.Account, Text: v.Text}
	if q.Min, err = parseOptionalBRL(v.Min); err != nil {
		return ledger.SearchQuery{}, err
	}
	if q.Max, err = parseOptionalBRL(v.Max); err != nil {
		return ledger.SearchQuery{}, err
	}
	return q, nil
}

func orNoDescription(s string) string {
	if s == "" {
		return "(sem descrição)"
	}
	return s
}
