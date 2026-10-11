// story: e16s01
package web

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/internal/money"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

var shortMonths = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

var targetStatusLabels = map[planning.TargetStatus]string{
	planning.TargetReached: "Atingida",
	planning.TargetOverdue: "Vencida",
	planning.TargetOnTrack: "No ritmo",
	planning.TargetBehind:  "Abaixo do ritmo",
}

func targetSet(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		due, err := budget.ParseMonth(r.FormValue("mes"))
		if err != nil {
			renderCategoryEdit(w, r, bud, u.ID, id, http.StatusBadRequest, "Informe o mês (AAAA-MM).")
			return
		}
		amount, err := money.ParseBRL(r.FormValue("valor"))
		if err != nil {
			err = budget.ErrInvalidAmount
		} else {
			err = bud.SetTarget(r.Context(), u.ID, id, amount, due)
		}
		if err != nil {
			status, msg := targetError(err)
			renderCategoryEdit(w, r, bud, u.ID, id, status, msg)
			return
		}
		http.Redirect(w, r, "/categorias/"+id+"#meta", http.StatusSeeOther)
	})
}

func targetRemove(svc *users.Service, bud *budget.Service) http.HandlerFunc {
	return signedInPage(svc, func(w http.ResponseWriter, r *http.Request, u users.User) {
		id := r.PathValue("id")
		if err := bud.RemoveTarget(r.Context(), u.ID, id); err != nil {
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível remover a meta.")
			return
		}
		http.Redirect(w, r, "/categorias/"+id+"#meta", http.StatusSeeOther)
	})
}

func targetError(err error) (int, string) {
	switch {
	case errors.Is(err, budget.ErrInvalidAmount):
		return http.StatusBadRequest, "Informe um valor maior que zero."
	case errors.Is(err, budget.ErrTargetPast):
		return http.StatusBadRequest, "Escolha o mês atual ou um mês futuro."
	case errors.Is(err, budget.ErrTargetIncome):
		return http.StatusBadRequest, "Metas são só para categorias de despesa."
	}
	return budgetError(err)
}

// targetRow fills the meta fields of an Orçamento row.
func targetRow(row *monthRowView, tp *planning.TargetPlan) {
	due := tp.Due.Start()
	row.Target = fmt.Sprintf("Meta %s até %s/%d · %d%%", money.FormatBRL(tp.Amount), shortMonths[due.Month()-1], due.Year(), tp.Progress)
	row.TargetStatus = targetStatusLabels[tp.Status]
	row.TargetAlert = tp.Status == planning.TargetOverdue || tp.Status == planning.TargetBehind
	if tp.Needed > 0 {
		row.TargetNeeded = "Guarde " + money.FormatBRL(tp.Needed) + " este mês"
	}
	if tp.Status == planning.TargetBehind {
		row.TargetSuggest = brlInput(tp.Needed)
	}
}
