// story: e14s01
package web

import (
	"cmp"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/upareal/upareal/budget"
	"github.com/upareal/upareal/ledger"
	"github.com/upareal/upareal/planning"
	"github.com/upareal/upareal/users"
)

const aiMaxTransactions = 2000

// aiExport downloads a pt-BR Markdown summary of 1–12 months for the user to
// give to an LLM. Read-only: months are peeked, never opened. No IDs or
// emails; amounts are plain numbers (1234.56).
func aiExport(d Deps) http.HandlerFunc {
	return signedInPage(d.Users, func(w http.ResponseWriter, r *http.Request, u users.User) {
		n := 3
		switch v := r.URL.Query().Get("meses"); v {
		case "1", "6", "12":
			n, _ = strconv.Atoi(v)
		}
		withTx := r.URL.Query().Get("lancamentos") == "on"
		md, err := buildAIExport(r, d, u.ID, n, withTx)
		if err != nil {
			slog.ErrorContext(r.Context(), "ai export failed", "user_id", u.ID, "err", err)
			renderError(w, r, http.StatusInternalServerError, "Erro inesperado", "Não foi possível gerar o arquivo. Tente de novo.")
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="upareal-analise-`+budget.MonthOf(time.Now()).String()+`.md"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(md))
		slog.InfoContext(r.Context(), "ai export", "user_id", u.ID, "meses", n, "lancamentos", withTx)
	})
}

func buildAIExport(r *http.Request, d Deps, userID string, n int, withTx bool) (string, error) {
	ctx := r.Context()
	cur := budget.MonthOf(time.Now())
	months := []budget.Month{cur}
	for len(months) < n {
		months = append([]budget.Month{months[0].Prev()}, months...)
	}
	start, end := months[0].Start(), cur.Next().Start()
	groups, err := d.Budget.Categories(ctx, userID)
	if err != nil {
		return "", err
	}
	ex, err := d.Ledger.ExportUserData(ctx, userID)
	if err != nil {
		return "", err
	}
	names := map[string]string{}
	for _, a := range ex.Accounts {
		names[a.ID] = a.Name
	}
	type tx struct {
		at time.Time
		t  ledger.ExportTransaction
	}
	var period []tx
	balances := map[string]int64{}
	for _, t := range ex.Transactions {
		at, err := time.Parse(time.RFC3339, t.OccurredAt)
		if err != nil {
			return "", fmt.Errorf("transaction time: %w", err)
		}
		if t.DeletedAt != "" || !at.Before(end) {
			continue
		}
		for _, l := range t.Legs {
			balances[l.AccountID] += l.AmountCentavos
		}
		if !at.Before(start) {
			period = append(period, tx{at, t})
		}
	}

	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	p("# upareal — dados para análise")
	p("")
	p("Período: %s a %s (%d meses). Exportado em %s.", months[0], cur, n, time.Now().In(saoPaulo).Format("2006-01-02"))
	p("Valores em reais, como números simples (1234.56).")
	p("")
	p("## Como ler estes dados")
	p("")
	p("- O upareal usa orçamento por envelopes: cada real que entra recebe uma categoria.")
	p("- **Planejado**: valor que a pessoa reservou para a categoria no mês.")
	p("- **Do mês anterior**: sobra (positivo) ou estouro (negativo) acumulado dos meses anteriores na categoria.")
	p("- **Gasto**: o que saiu nas contas ligadas à categoria no mês (estornos reduzem o gasto).")
	p("- **Disponível** = planejado + do mês anterior − gasto. Negativo significa categoria estourada.")
	p("- **A distribuir**: dinheiro que entrou (receitas e saldos iniciais) e ainda não foi planejado, acumulado. Negativo significa que se planejou mais do que se tem.")
	p("- **Sem categoria**: gastos e receitas em contas ainda não ligadas a uma categoria.")
	p("- **Cartões**: gastos no cartão já saem da categoria; \"para pagar\" é a dívida do cartão, já reservada. \"Sem cobertura\" é a parte da dívida que o dinheiro disponível não cobre.")
	p("- **Contas fora do orçamento** (investimentos, por exemplo): só contam quando o dinheiro entra ou sai delas para uma conta do orçamento.")
	p("- Saldo negativo em cartão ou dívida é o valor devido.")
	p("")
	p("## Sugestões de perguntas")
	p("")
	p("- Em quais categorias eu gasto mais do que planejo, e com que frequência?")
	p("- Para onde vai a maior parte do meu dinheiro?")
	p("- Quanto eu poderia guardar por mês sem estourar o orçamento?")
	p("- Há gastos recorrentes que eu poderia cortar ou renegociar?")
	p("- Meu planejamento é realista comparado ao que eu realmente gasto?")
	p("")
	p("## Contas")
	p("")
	p("| Conta | Tipo | No orçamento | Saldo em %s |", end.Add(-time.Second).In(saoPaulo).Format("2006-01-02"))
	p("|---|---|---|---|")
	for _, a := range ex.Accounts {
		if (a.Type != "asset" && a.Type != "liability") || (a.ArchivedAt != "" && balances[a.ID] == 0) {
			continue
		}
		kind := map[string]string{"asset": "conta", "liability": "dívida"}[a.Type]
		if a.IsCard {
			kind = "cartão de crédito"
		}
		p("| %s | %s | %s | %s |", cell(a.Name), kind, map[bool]string{true: "sim", false: "não"}[a.OnBudget], plain(balances[a.ID]))
	}

	for _, m := range months {
		plan, opened, err := d.Planning.PeekMonth(ctx, userID, m)
		if err != nil {
			return "", err
		}
		writeAIMonth(&b, m, plan, opened, groups)
	}

	if withTx {
		p("")
		p("## Lançamentos")
		p("")
		slices.SortStableFunc(period, func(a, b tx) int { return a.at.Compare(b.at) })
		if len(period) > aiMaxTransactions {
			p("_Mostrando os %d lançamentos mais recentes de %d no período._", aiMaxTransactions, len(period))
			p("")
			period = period[len(period)-aiMaxTransactions:]
		}
		if len(period) == 0 {
			p("Nenhum lançamento no período.")
		} else {
			p("| Data | Descrição | De | Para | Valor |")
			p("|---|---|---|---|---|")
			for _, t := range period {
				from, to, total := legSides(t.t.Legs, names)
				p("| %s | %s | %s | %s | %s |", t.at.In(saoPaulo).Format("2006-01-02"), cell(t.t.Description), cell(from), cell(to), plain(total))
			}
		}
	}
	return b.String(), nil
}

func writeAIMonth(b *strings.Builder, m budget.Month, plan planning.MonthPlan, opened bool, groups []budget.Group) {
	p := func(format string, a ...any) { fmt.Fprintf(b, format+"\n", a...) }
	p("")
	p("## Orçamento — %s", m)
	p("")
	if !opened {
		p("_O orçamento deste mês não foi aberto: valores planejados são 0._")
		p("")
	}
	p("A distribuir (acumulado): %s · Entradas acumuladas: %s", plain(plan.ToBeAssigned), plain(plan.Received))
	p("")
	var rows, income []string
	for _, g := range groups {
		for _, c := range g.Categories {
			cp := plan.Categories[c.ID]
			if g.Kind == "income" {
				if cp.Received != 0 || cp.Amount != 0 {
					income = append(income, fmt.Sprintf("- %s: recebido %s (previsto %s)", c.Name, plain(cp.Received), plain(cp.Amount)))
				}
				continue
			}
			if cp.Amount == 0 && cp.CarryOver == 0 && cp.Spent == 0 && cp.Available == 0 {
				continue
			}
			rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |", cell(g.Name), cell(c.Name), plain(cp.Amount), plain(cp.CarryOver), plain(cp.Spent), plain(cp.Available)))
		}
	}
	if len(rows) == 0 {
		p("Nenhuma categoria de despesa com valores neste mês.")
	} else {
		p("| Grupo | Categoria | Planejado | Do mês anterior | Gasto | Disponível |")
		p("|---|---|---|---|---|---|")
		for _, row := range rows {
			p("%s", row)
		}
	}
	p("")
	if len(income) > 0 {
		p("Receitas:")
		for _, l := range income {
			p("%s", l)
		}
		p("")
	}
	p("Sem categoria: gasto %s, recebido %s", plain(plan.UnmappedSpent), plain(plan.UnmappedReceived))
	for _, c := range plan.Cards {
		p("Cartão %s: para pagar %s", c.Name, plain(c.Due))
	}
	if plan.Uncovered > 0 {
		p("Sem cobertura nos cartões: %s", plain(plan.Uncovered))
	}
}

// legSides describes where money came from (negative legs) and went to
// (positive legs); amounts are shown per account only for splits.
func legSides(legs []ledger.ExportLeg, names map[string]string) (from, to string, total int64) {
	var out, in []ledger.ExportLeg
	for _, l := range legs {
		if l.AmountCentavos < 0 {
			out = append(out, l)
		} else {
			in = append(in, l)
			total += l.AmountCentavos
		}
	}
	side := func(ls []ledger.ExportLeg) string {
		slices.SortFunc(ls, func(a, b ledger.ExportLeg) int { return cmp.Compare(names[a.AccountID], names[b.AccountID]) })
		parts := make([]string, len(ls))
		for i, l := range ls {
			parts[i] = names[l.AccountID]
			if len(ls) > 1 {
				parts[i] += " (" + plain(max(l.AmountCentavos, -l.AmountCentavos)) + ")"
			}
		}
		return strings.Join(parts, ", ")
	}
	return side(out), side(in), total
}

// plain formats centavos as a plain decimal number: -123456 → "-1234.56".
func plain(c int64) string {
	sign := ""
	abs := uint64(c)
	if c < 0 {
		sign, abs = "-", uint64(-(c+1))+1 // safe for math.MinInt64
	}
	return fmt.Sprintf("%s%d.%02d", sign, abs/100, abs%100)
}

// cell makes user text safe inside a Markdown table cell.
func cell(s string) string {
	return strings.NewReplacer("|", `\|`, "\r", " ", "\n", " ").Replace(s)
}
