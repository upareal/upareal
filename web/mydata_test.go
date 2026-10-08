// story: e10s01
package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMyData_DownloadEverythingOfMineOnly(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	createAccount(t, b, url.Values{"nome": {"Itaú da Maria"}, "tipo": {"asset"}, "saldo": {"1.234,56"}, "data": {"2026-01-01"}, "orcamento": {"on"}})
	j := signInJoao(t, b)
	createAccount(t, j, url.Values{"nome": {"Conta do João"}, "tipo": {"asset"}})

	if _, home := b.get(t, "/"); !strings.Contains(home, `href="/conta/dados"`) {
		t.Error("home has no Meus dados link")
	}
	if _, page := b.get(t, "/conta/dados"); !strings.Contains(page, `href="/conta/dados/exportar"`) || !strings.Contains(page, "Baixar meus dados") {
		t.Fatal("Meus dados page has no download link")
	}
	res, body := b.get(t, "/conta/dados/exportar")
	today := time.Now().In(saoPaulo(t)).Format("2006-01-02")
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") ||
		res.Header.Get("Content-Disposition") != `attachment; filename="upareal-dados-`+today+`.json"` || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("export = %d %q %q %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Content-Disposition"), res.Header.Get("Cache-Control"))
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("export is not JSON: %v", err)
	}
	for _, key := range []string{"exported_at", "users", "ledger", "budget", "planning"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("export has no %q", key)
		}
	}
	for _, want := range []string{"maria@example.com", "Itaú da Maria", `"amount_centavos":123456`, "Combustível"} {
		if !strings.Contains(body, want) {
			t.Errorf("export missing %s", want)
		}
	}
	if strings.Contains(body, "Conta do João") || strings.Contains(body, "joao@example.com") {
		t.Error("export leaks João's data")
	}
	if res, _ := secondBrowser(b).noFollow().Get(b.base + "/conta/dados/exportar"); res.StatusCode != http.StatusSeeOther {
		t.Errorf("signed-out export = %d, want redirect home", res.StatusCode)
	}
}
