// story: e07s04
package importer_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger/importer"
)

func parseFixture(t *testing.T, name string) []importer.Line {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	lines, err := importer.ParseOFX(f)
	if err != nil {
		t.Fatalf("ParseOFX(%s): %v", name, err)
	}
	return lines
}

func TestOFX_SGMLBankStatementInWindows1252(t *testing.T) {
	lines := parseFixture(t, "bank_sgml_1252.ofx")
	if len(lines) != 2 {
		t.Fatalf("lines = %+v, want 2", lines)
	}
	padaria, salario := lines[0], lines[1]
	if padaria.ImportID != "2026013101" || padaria.Amount != -1250 || padaria.Description != "PADARIA SÃO JORGE COMPRA CARTÃO" {
		t.Errorf("padaria = %+v; want FITID 2026013101, -1250, accents converted from Windows-1252", padaria)
	}
	// A date without zone is São Paulo local time, not GMT.
	if !padaria.At.Equal(time.Date(2026, 1, 31, 0, 0, 0, 0, saoPaulo)) {
		t.Errorf("padaria at = %s, want 31/01/2026 00:00 São Paulo", padaria.At)
	}
	if salario.Amount != 800000 || !salario.At.Equal(time.Date(2026, 1, 15, 10, 30, 0, 0, saoPaulo)) {
		t.Errorf("salario = %+v; want +800000 at 15/01 10:30 [-3:BRT]", salario)
	}
}

func TestOFX_XMLCreditCardStatement(t *testing.T) {
	lines := parseFixture(t, "card_xml.ofx")
	if len(lines) != 2 {
		t.Fatalf("lines = %+v, want 2", lines)
	}
	if lines[0].ImportID != "CC001" || lines[0].Amount != -8990 || lines[0].Description != "IFOOD *RESTAURANTE" {
		t.Errorf("purchase = %+v; want CC001 -8990 IFOOD *RESTAURANTE", lines[0])
	}
	if lines[1].Amount != 50000 || !lines[1].At.Equal(time.Date(2026, 1, 25, 0, 0, 0, 0, saoPaulo)) {
		t.Errorf("payment = %+v; want +50000 on 25/01 00:00 São Paulo", lines[1])
	}
}

func TestOFX_RejectsBadInput(t *testing.T) {
	good, err := os.ReadFile("testdata/card_xml.ofx")
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]string{
		"not OFX":           "hello",
		"no transactions":   strings.Replace(strings.Replace(string(good), "<STMTTRN>", "<X>", -1), "</STMTTRN>", "</X>", -1),
		"sub-centavo":       strings.Replace(string(good), "-89.90", "-89.905", 1),
		"missing FITID":     strings.Replace(string(good), "<FITID>CC001</FITID>", "", 1),
	} {
		if _, err := importer.ParseOFX(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDetect_ChoosesOFXOrCSVByContent(t *testing.T) {
	ofx, _ := os.ReadFile("testdata/bank_sgml_1252.ofx")
	xml, _ := os.ReadFile("testdata/card_xml.ofx")
	csv := "\uFEFFid;data;hora;descricao;valor\nx;01/02/2026;;Pão;-1,00\n"
	for name, tc := range map[string]struct {
		in     string
		wantID string
	}{
		"OFX SGML": {string(ofx), "2026013101"},
		"OFX XML":  {"\n  " + string(xml), "CC001"},
		"CSV":      {csv, "x"},
	} {
		lines, err := importer.Parse(strings.NewReader(tc.in))
		if err != nil || len(lines) == 0 || lines[0].ImportID != tc.wantID {
			t.Errorf("%s: Parse = %+v, %v; want first id %q", name, lines, err, tc.wantID)
		}
	}
}
