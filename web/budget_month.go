// story: e05s03 e08s02 e08s03 e08s04 e16s01
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
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

var monthNames = [...]string{"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
	"Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}

// monthView is the month budget page.
type monthView struct {
	Month, Label, Prev, Next  string
	Groups                    []monthGroupView
	ExpenseTotal, IncomeTotal string
	ToBeAssigned              string
	UnmappedSpent             string // "" when nothing is Sem categoria
	UnmappedReceived          string
	Cards                     []cardView
	Uncovered                 string // "" when every card is covered
}

type cardView struct{ Name, Due string }

type monthGroupView struct {
	Name, Total string
	Rows        []monthRowView
}

type monthRowView struct {
	CategoryID, Name, Value string
	Income                  bool
	Spent, Available        string // expense
	Overspent               bool
	CarryOver               string // expense; "" when zero
	CarriedOverspent        bool
	Received                string // income
	// e16s01: savings target; Target is "" without one
	Target, TargetStatus, TargetNeeded string
	TargetAlert                        bool   // overdue or behind
	TargetSuggest                      string // amount for Usar sugestão; "" for no button
}

func budgetMonthGet(svc *users.Service, bud *budget.Service, plan *planning.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		m := budget.MonthOf(time.Now())
		if s := r.URL.Query().Get("mes"); s != "" {
			var err error
			if m, err = budget.ParseMonth(s); err != nil {
				renderError(w, r, http.StatusBadRequest, "Mês inválido", "Use o formato AAAA-MM.")
				return
			}
		}
		renderBudgetMonth(w, r, bud, plan, u.ID, m, http.StatusOK, "")
	})
}

func budgetMonthSet(svc *users.Service, bud *budget.Service, plan *planning.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		m, err := budget.ParseMonth(r.PathValue("mes"))
		if err != nil {
			renderError(w, r, http.StatusBadRequest, "Mês inválido", "Use o formato AAAA-MM.")
			return
		}
		amount, err := money.ParseBRL(r.FormValue("valor"))
		if err == nil {
			err = bud.SetAmount(r.Context(), u.ID, r.PathValue("categoria"), m, amount)
		}
		switch {
		case err == nil:
			http.Redirect(w, r, "/orcamento?mes="+url.QueryEscape(m.String()), http.StatusSeeOther)
		case errors.Is(err, budget.ErrNotFound):
			renderError(w, r, http.StatusNotFound, "Categoria", "Categoria não encontrada.")
		default: // malformed or negative (B4)
			renderBudgetMonth(w, r, bud, plan, u.ID, m, http.StatusBadRequest, "Valor inválido. Use o formato 1.234,56.")
		}
	})
}

func renderBudgetMonth(w http.ResponseWriter, r *http.Request, bud *budget.Service, plan *planning.Service, userID string, m budget.Month, status int, errMsg string) {
	groups, err := bud.Categories(r.Context(), userID)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar o orçamento.")
		return
	}
	p, err := plan.Month(r.Context(), userID, m)
	if err != nil {
		renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível carregar o orçamento.")
		return
	}
	start := m.Start()
	v := monthView{Month: m.String(), Label: fmt.Sprintf("%s de %d", monthNames[start.Month()-1], start.Year()),
		Prev: m.Prev().String(), Next: m.Next().String()}
	var expense, income int64
	for _, g := range groups {
		gv := monthGroupView{Name: g.Name}
		var total int64
		for _, c := range g.Categories {
			cp := p.Categories[c.ID]
			total += cp.Amount
			gv.Rows = append(gv.Rows, monthRowView{CategoryID: c.ID, Name: c.Name, Value: brlInput(cp.Amount),
				Income: g.Kind == "income", Spent: money.FormatBRL(cp.Spent), Available: money.FormatBRL(cp.Available),
				Overspent: cp.Available < 0, Received: money.FormatBRL(cp.Received), CarriedOverspent: cp.CarryOver < 0})
			if cp.CarryOver != 0 {
				gv.Rows[len(gv.Rows)-1].CarryOver = money.FormatBRL(cp.CarryOver)
			}
			if cp.Target != nil {
				targetRow(&gv.Rows[len(gv.Rows)-1], cp.Target)
			}
		}
		gv.Total = money.FormatBRL(total)
		if g.Kind == "income" {
			income += total
		} else {
			expense += total
		}
		v.Groups = append(v.Groups, gv)
	}
	v.ExpenseTotal, v.IncomeTotal = money.FormatBRL(expense), money.FormatBRL(income)
	v.ToBeAssigned = money.FormatBRL(p.ToBeAssigned)
	for _, c := range p.Cards {
		v.Cards = append(v.Cards, cardView{Name: c.Name, Due: money.FormatBRL(c.Due)})
	}
	if p.Uncovered > 0 {
		v.Uncovered = money.FormatBRL(p.Uncovered)
	}
	if p.UnmappedSpent != 0 || p.UnmappedReceived != 0 {
		v.UnmappedSpent, v.UnmappedReceived = money.FormatBRL(p.UnmappedSpent), money.FormatBRL(p.UnmappedReceived)
	}
	renderHTML(w, r, status, budgetMonthPage(v, errMsg))
}

// brlInput formats centavos for an input field: "1.234,56" (no currency symbol).
func brlInput(c int64) string { return strings.TrimPrefix(money.FormatBRL(c), "R$ ") }
