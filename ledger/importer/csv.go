// story: e07s01

// Package importer turns bank statement files into lines the ledger can post.
// Parsing is separate from posting: every format only produces []Line.
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	_ "time/tzdata" // America/Sao_Paulo must load everywhere
	"unicode/utf8"

	"github.com/upareal/upareal/internal/money"
)

// Limits for one uploaded file.
const (
	MaxBytes       = 5 << 20
	MaxLines       = 10000
	maxDescription = 200
	maxField       = 200
)

var (
	// ErrTooLarge means the file exceeds MaxBytes or MaxLines.
	ErrTooLarge = fmt.Errorf("importer: file too large (max %d MB, %d lines)", MaxBytes>>20, MaxLines)
	// ErrNoLines means the file has no data lines after the header.
	ErrNoLines = errors.New("importer: the file has no lines after the header")
)

var saoPaulo = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err) // zone data is embedded; failure is a build bug
	}
	return loc
}()

// Line is one statement line, as seen from the imported account.
type Line struct {
	ImportID    string    // the file's row id: unique per account
	At          time.Time // when it happened
	Description string
	Amount      int64  // centavos; negative = money left the account
	Account     string // free "conta" text, kept for future auto-categorization
}

// Error is one rejected line.
type Error struct {
	Line   int // 1-based line number in the file (the header is line 1)
	Reason string
}

// Errors lists every rejected line; nothing is imported when it is returned.
type Errors []Error

func (e Errors) Error() string {
	parts := make([]string, 0, len(e))
	for _, x := range e {
		parts = append(parts, fmt.Sprintf("linha %d: %s", x.Line, x.Reason))
	}
	return "importer: " + strings.Join(parts, "; ")
}

// ParseGenericCSV reads upareal's generic CSV: UTF-8, ';' separator, a
// required header line (ignored), then positional columns
// id;data;hora;descricao;valor;conta. hora and conta may be empty and conta
// may be omitted. Every bad line is reported; any error means no lines.
func ParseGenericCSV(r io.Reader) ([]Line, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("importer: read: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	data = bytes.TrimPrefix(data, []byte("\uFEFF")) // Excel's UTF-8 BOM

	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	var (
		lines []Line
		errs  Errors
		seen  = map[string]int{}
	)
	for n := 0; ; n++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				line = pe.Line
			}
			errs = append(errs, Error{Line: line, Reason: "linha mal formada"})
			continue
		}
		if n == 0 {
			continue // header: required, ignored
		}
		if n > MaxLines {
			return nil, ErrTooLarge
		}
		l, reason := parseRecord(rec)
		if id := strings.TrimSpace(rec[0]); id != "" { // duplicates count even when the other line is invalid
			if first, dup := seen[id]; dup && reason == "" {
				reason = fmt.Sprintf("id %q repetido (também na linha %d)", id, first)
			} else if !dup {
				seen[id] = line
			}
		}
		if reason != "" {
			errs = append(errs, Error{Line: line, Reason: reason})
			continue
		}
		lines = append(lines, l)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	if len(lines) == 0 {
		return nil, ErrNoLines
	}
	return lines, nil
}

// parseRecord validates one data line; reason is empty when it is valid.
func parseRecord(rec []string) (Line, string) {
	if len(rec) < 5 {
		return Line{}, "esperadas as colunas id;data;hora;descricao;valor[;conta]"
	}
	for _, f := range rec {
		if !utf8.ValidString(f) {
			return Line{}, "texto fora de UTF-8 (salve como \"CSV UTF-8\")"
		}
		if utf8.RuneCountInString(f) > maxField {
			return Line{}, fmt.Sprintf("campo com mais de %d caracteres", maxField)
		}
	}
	id, date, hour, desc, value := strings.TrimSpace(rec[0]), strings.TrimSpace(rec[1]), strings.TrimSpace(rec[2]), strings.TrimSpace(rec[3]), rec[4]
	if id == "" {
		return Line{}, "id vazio"
	}
	if hour == "" {
		hour = "00:00"
	}
	at, err := time.ParseInLocation("02/01/2006 15:04", date+" "+hour, saoPaulo)
	if err != nil {
		return Line{}, fmt.Sprintf("data/hora inválida %q %q (use dd/mm/aaaa e hh:mm)", date, hour)
	}
	if utf8.RuneCountInString(desc) > maxDescription {
		return Line{}, fmt.Sprintf("descrição com mais de %d caracteres", maxDescription)
	}
	amount, err := money.ParseBRL(value)
	if err != nil || amount == 0 {
		return Line{}, fmt.Sprintf("valor inválido %q (use 1.234,56; negativo = saída)", value)
	}
	l := Line{ImportID: id, At: at, Description: desc, Amount: amount}
	if len(rec) > 5 {
		l.Account = strings.TrimSpace(rec[5])
	}
	return l, ""
}
