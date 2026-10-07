// story: e07s04
package importer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aclindsa/ofxgo"
	"golang.org/x/text/encoding/charmap"
)

// ErrUnreadableOFX means the file is not an OFX statement ofxgo can read.
var ErrUnreadableOFX = errors.New("importer: unreadable OFX")

// ParseOFX reads OFX 1.x (SGML) and 2.x (XML) bank and credit card
// statements. Non-UTF-8 bytes are converted from Windows-1252 first (ofxgo
// ignores CHARSET). FITID becomes the import id; dates without a zone are
// São Paulo local time.
func ParseOFX(r io.Reader) ([]Line, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("importer: read: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	if !utf8.Valid(data) {
		if data, err = charmap.Windows1252.NewDecoder().Bytes(data); err != nil {
			return nil, fmt.Errorf("%w: charset: %v", ErrUnreadableOFX, err)
		}
	}
	resp, err := ofxgo.ParseResponse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadableOFX, err)
	}
	var trns []ofxgo.Transaction
	for _, m := range resp.Bank {
		if st, ok := m.(*ofxgo.StatementResponse); ok && st.BankTranList != nil {
			trns = append(trns, st.BankTranList.Transactions...)
		}
	}
	for _, m := range resp.CreditCard {
		if st, ok := m.(*ofxgo.CCStatementResponse); ok && st.BankTranList != nil {
			trns = append(trns, st.BankTranList.Transactions...)
		}
	}
	if len(trns) == 0 {
		return nil, ErrNoLines
	}
	if len(trns) > MaxLines {
		return nil, ErrTooLarge
	}
	lines := make([]Line, 0, len(trns))
	var errs Errors
	for i, t := range trns {
		l, reason := ofxLine(t)
		if reason != "" {
			errs = append(errs, Error{Line: i + 1, Reason: reason})
			continue
		}
		lines = append(lines, l)
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return lines, nil
}

// ofxLine converts one STMTTRN; reason is empty when it is valid.
func ofxLine(t ofxgo.Transaction) (Line, string) {
	id := strings.TrimSpace(string(t.FiTID))
	if id == "" {
		return Line{}, "lançamento sem FITID"
	}
	cents := new(big.Rat).Mul(&t.TrnAmt.Rat, big.NewRat(100, 1))
	if !cents.IsInt() || !cents.Num().IsInt64() || cents.Num().Int64() == 0 {
		return Line{}, fmt.Sprintf("FITID %s: valor %s inválido (centavos inteiros, diferente de zero)", id, t.TrnAmt.String())
	}
	desc := strings.TrimSpace(string(t.Name))
	if memo := strings.TrimSpace(string(t.Memo)); memo != "" && memo != desc {
		desc = strings.TrimSpace(desc + " " + memo)
	}
	if utf8.RuneCountInString(desc) > maxDescription {
		desc = string([]rune(desc)[:maxDescription])
	}
	return Line{ImportID: id, At: localIfZoneless(t.DtPosted.Time), Description: desc, Amount: cents.Num().Int64()}, ""
}

// localIfZoneless reinterprets a date ofxgo parsed without a zone (it
// defaults to GMT) as São Paulo wall-clock time.
// ponytail: an explicit "[0:GMT]" looks the same and is shifted 3h; handle in
// a bank-specific parser if a bank ever sends it.
func localIfZoneless(t time.Time) time.Time {
	if name, offset := t.Zone(); name == "GMT" && offset == 0 {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, saoPaulo)
	}
	return t
}
