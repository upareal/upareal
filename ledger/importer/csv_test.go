// story: e07s01
package importer_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/upareal/upareal/ledger/importer"
)

var saoPaulo, _ = time.LoadLocation("America/Sao_Paulo")

func TestGenericCSV_ParsesPositionalColumns(t *testing.T) {
	in := "\uFEFFid;data;hora;descricao;valor;conta\n" + // BOM from Excel; header ignored
		"abc123;31/01/2026;22:30;SUPERMERCADO X;-150,00;Cartão final 1234\n" +
		"def456;01/02/2026;;SALARIO ACME;8.000,00;\n" +
		"ghi789;02/02/2026;08:15;PIX MARIA;-20,00\n" // conta column omitted
	lines, err := importer.ParseGenericCSV(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseGenericCSV: %v", err)
	}
	want := []importer.Line{
		{ImportID: "abc123", At: time.Date(2026, 1, 31, 22, 30, 0, 0, saoPaulo), Description: "SUPERMERCADO X", Amount: -15000, Account: "Cartão final 1234"},
		{ImportID: "def456", At: time.Date(2026, 2, 1, 0, 0, 0, 0, saoPaulo), Description: "SALARIO ACME", Amount: 800000},
		{ImportID: "ghi789", At: time.Date(2026, 2, 2, 8, 15, 0, 0, saoPaulo), Description: "PIX MARIA", Amount: -2000},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %+v, want %d", lines, len(want))
	}
	for i, w := range want {
		g := lines[i]
		if g.ImportID != w.ImportID || !g.At.Equal(w.At) || g.Description != w.Description || g.Amount != w.Amount || g.Account != w.Account {
			t.Errorf("line %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestGenericCSV_ReportsEveryBadLine(t *testing.T) {
	in := "cabecalho qualquer\n" +
		"a;31/02/2026;;x;-1,00\n" + // line 2: invalid date
		"b;01/02/2026;25:00;x;-1,00\n" + // line 3: invalid time
		"c;01/02/2026;;x;12,345\n" + // line 4: ambiguous amount
		"d;01/02/2026;;x;0\n" + // line 5: zero
		";01/02/2026;;x;-1,00\n" + // line 6: empty id
		"f;01/02/2026;;x\n" + // line 7: too few columns
		"a;01/02/2026;;x;-1,00\n" + // line 8: duplicate id in the file
		"ok;01/02/2026;;x;-1,00\n"
	_, err := importer.ParseGenericCSV(strings.NewReader(in))
	var errs importer.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("err = %v, want importer.Errors", err)
	}
	got := map[int]bool{}
	for _, e := range errs {
		got[e.Line] = true
		if e.Reason == "" {
			t.Errorf("line %d has no reason", e.Line)
		}
	}
	for _, line := range []int{2, 3, 4, 5, 6, 7, 8} {
		if !got[line] {
			t.Errorf("line %d not reported; got %+v", line, errs)
		}
	}
	if got[9] || got[1] {
		t.Errorf("valid line or header reported: %+v", errs)
	}
}

func TestGenericCSV_RejectsEmptyHeaderOnlyAndBadEncoding(t *testing.T) {
	for name, in := range map[string]string{
		"empty file":   "",
		"header only":  "id;data;hora;descricao;valor;conta\n",
		"windows-1252": "id;data;hora;descricao;valor\nx;01/02/2026;;P\xe3o;-1,00\n",
		"long text":    "h\nx;01/02/2026;;" + strings.Repeat("y", 201) + ";-1,00\n",
	} {
		if _, err := importer.ParseGenericCSV(strings.NewReader(in)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGenericCSV_LineLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("h\n")
	for i := range importer.MaxLines + 1 {
		b.WriteString("id")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString(time.Duration(i).String())
		b.WriteString(";01/02/2026;;x;-1,00\n")
	}
	_, err := importer.ParseGenericCSV(strings.NewReader(b.String()))
	if !errors.Is(err, importer.ErrTooLarge) {
		t.Errorf("%d lines = %v, want ErrTooLarge", importer.MaxLines+1, err)
	}
}
