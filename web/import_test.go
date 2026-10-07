// story: e07s01
package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const goodCSV = "id;data;hora;descricao;valor;conta\n" +
	"abc123;31/01/2026;22:30;SUPERMERCADO X;-150,00;Cartão final 1234\n" +
	"def456;01/02/2026;;SALARIO ACME;8.000,00;\n"

// uploadCSV posts content as the "arquivo" file of the import form.
func uploadCSV(t *testing.T, b browser, accountID, content string) (*http.Response, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("arquivo", "extrato.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(fw, content)
	_ = mw.Close()
	res, err := b.client.Post(b.base+"/contas/"+accountID+"/importar", mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)
	return res, string(body)
}

func importBook(t *testing.T, b browser) string {
	t.Helper()
	_, body := createAccount(t, b, url.Values{"nome": {"Itaú"}, "tipo": {"asset"}, "saldo": {"1.000,00"}, "data": {"2026-01-10"}, "orcamento": {"on"}})
	return accountID(t, body, "Itaú")
}

func TestImportFlow_ImportReimportAndQueue(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	if _, page := b.get(t, "/contas/"+itau+"/lancamentos"); !strings.Contains(page, `href="/contas/`+itau+`/importar"`) {
		t.Error("register has no link to import")
	}
	if res, page := b.get(t, "/contas/"+itau+"/importar"); res.StatusCode != http.StatusOK || !strings.Contains(page, `enctype="multipart/form-data"`) || !strings.Contains(page, `href="/ajuda/csv"`) {
		t.Fatalf("import page = %d; want an upload form linking to the help page", res.StatusCode)
	}

	res, body := uploadCSV(t, b, itau, goodCSV)
	if res.Request.URL.Path != "/revisao" {
		t.Fatalf("after import ended at %s, want /revisao", res.Request.URL)
	}
	for _, want := range []string{"2 importadas, 0 já existentes", "SUPERMERCADO X", "SALARIO ACME", "Cartão final 1234", "Itaú"} {
		if !strings.Contains(body, want) {
			t.Errorf("review queue missing %q", want)
		}
	}
	if _, body = uploadCSV(t, b, itau, goodCSV); !strings.Contains(body, "0 importadas, 2 já existentes") {
		t.Error("re-import did not report 2 existing lines")
	}
}

func TestImportFlow_BadFilesImportNothing(t *testing.T) {
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	bad := goodCSV + "ghi789;31/02/2026;;DATA RUIM;-1,00\n"
	res, body := uploadCSV(t, b, itau, bad)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "Linha 4") {
		t.Errorf("bad line = %d, want 400 naming Linha 4", res.StatusCode)
	}
	if _, q := b.get(t, "/revisao"); strings.Contains(q, "SUPERMERCADO X") {
		t.Error("a file with a bad line imported something")
	}
	huge := goodCSV + strings.Repeat("x", 5<<20)
	if res, body := uploadCSV(t, b, itau, huge); res.StatusCode != http.StatusBadRequest || !strings.Contains(body, "grande demais") {
		t.Errorf("file over 5 MB = %d, want 400 'grande demais'", res.StatusCode)
	}
}

func TestImportFlow_HelpPageAndRateLimit(t *testing.T) {
	b := newBrowser(t)
	if res, page := b.get(t, "/ajuda/csv"); res.StatusCode != http.StatusOK || !strings.Contains(page, "id;data;hora;descricao;valor;conta") {
		t.Errorf("help page = %d; want the column order", res.StatusCode)
	}
	signIn(t, b)
	itau := importBook(t, b)
	var last int
	for range 11 {
		res, _ := uploadCSV(t, b, itau, goodCSV)
		last = res.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("11th import within a minute = %d, want 429", last)
	}
}
