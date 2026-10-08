// story: e13s01
package web

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/money"
)

type statsView struct {
	Users, Accounts, Transactions, ImportedLines, Categories, Planned, Month string
}

// publicStats shows aggregate numbers to anyone; each module returns counts
// and sums only (the X2 exception). ponytail: computed per request, add a
// short cache if the page ever gets real traffic.
func publicStats(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		m := budget.MonthOf(time.Now())
		u, err1 := d.Users.PublicStats(ctx)
		lst, err2 := d.Ledger.PublicStats(ctx)
		bst, err3 := d.Budget.PublicStats(ctx, m)
		if err := errors.Join(err1, err2, err3); err != nil {
			slog.ErrorContext(ctx, "public stats failed", "err", err)
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar as estatísticas.")
			return
		}
		start := m.Start()
		renderHTML(w, r, http.StatusOK, statsPage(statsView{
			Users: count(u.Users), Accounts: count(lst.Accounts), Transactions: count(lst.Transactions),
			ImportedLines: count(lst.ImportedLines), Categories: count(bst.Categories),
			Planned: money.FormatBRL(bst.PlannedCentavos), Month: monthNames[start.Month()-1] + " de " + start.Format("2006"),
		}))
	}
}

// count formats n with pt-BR thousands separators: 1234 → "1.234".
func count(n int64) string { return strings.TrimSuffix(strings.TrimPrefix(money.FormatBRL(n*100), "R$ "), ",00") }
