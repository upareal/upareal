// story: e14s01
package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func countRows(t *testing.T, b browser, table string) int {
	t.Helper()
	var n int
	if err := b.app.conn.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAIExport_MarkdownForTheLLM(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	now := time.Now().In(saoPaulo(t))
	mes := now.Format("2006-01")
	prev := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, saoPaulo(t)).Format("2006-01-02")
	createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {prev}, "orcamento": {"on"}})
	createAccount(t, b, url.Values{"nome": {"Posto Shell"}, "tipo": {"expense"}})
	_, accounts := createAccount(t, b, url.Values{"nome": {"Padaria"}, "tipo": {"expense"}})
	itau, shell, padaria := accountID(t, accounts, "Itaú"), accountID(t, accounts, "Posto Shell"), accountID(t, accounts, "Padaria")
	_, cats := b.get(t, "/categorias")
	fuel := linkID(t, cats, "/categorias/", "Combustível")
	b.postForm(t, "/planejamento/mapa/"+shell, url.Values{"categoria": {fuel}})
	b.postForm(t, "/orcamento/"+mes+"/"+fuel, url.Values{"valor": {"300,00"}})
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(shell, "120,00", mes+"-01", "10:00", "Gasolina"))
	b.postForm(t, "/contas/"+itau+"/lancamentos", entry(padaria, "30,00", mes+"-01", "11:00", "Pão | leite"))
	j := signInJoao(t, b)
	createAccount(t, j, url.Values{"nome": {"Conta do João"}, "tipo": {"asset"}})

	if _, page := b.get(t, "/conta/dados"); !strings.Contains(page, `action="/conta/dados/ia"`) || !strings.Contains(page, `name="meses"`) || !strings.Contains(page, `name="lancamentos"`) {
		t.Fatal("Meus dados has no Análise com IA form")
	}
	monthsBefore := countRows(t, b, "budget_month")
	res, md := b.get(t, "/conta/dados/ia?meses=3&lancamentos=on")
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/markdown") ||
		res.Header.Get("Content-Disposition") != `attachment; filename="upareal-analise-`+mes+`.md"` || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("export = %d %q %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Content-Disposition"))
	}
	for _, want := range []string{
		"## Como ler estes dados",
		"## Sugestões de perguntas",
		"## Contas",
		"| Itaú | conta | sim | 850.00 |",
		"## Orçamento — " + mes,
		"| Transporte | Combustível | 300.00 | 0.00 | 120.00 | 180.00 |",
		"Sem categoria: gasto 30.00",
		"## Lançamentos",
		"| " + mes + "-01 | Gasolina | Itaú | Posto Shell | 120.00 |",
		`Pão \| leite`,
	} {
		if !strings.Contains(md, want) {
			t.Errorf("export missing %q", want)
		}
	}
	if strings.Contains(md, "R$") || strings.Contains(md, "maria@example.com") || strings.Contains(md, "Conta do João") ||
		regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-`).MatchString(md) {
		t.Error("export has formatted money, an email, another user's data or IDs")
	}
	if after := countRows(t, b, "budget_month"); after != monthsBefore {
		t.Errorf("the export opened budget months: %d → %d", monthsBefore, after)
	}

	if _, md = b.get(t, "/conta/dados/ia?meses=99"); strings.Contains(md, "## Lançamentos") || !strings.Contains(md, "(3 meses)") {
		t.Error("without lancamentos=on and with an invalid meses: want no transactions and the 3-month default")
	}
}
