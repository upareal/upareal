// story: e04s02

// Package money parses and formats Brazilian reais as int64 centavos.
// Money is never a float.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Digits with optional "." thousand groups, then optional "," and 1–2 decimals.
var brlNumber = regexp.MustCompile(`^(\d{1,3}(?:\.\d{3})+|\d+)(?:,(\d{1,2}))?$`)

// ParseBRL reads a pt-BR amount ("1.234,56", "1234", "-R$ 50,00") into centavos.
// Ambiguous input such as "12,345" or "1.23" is rejected, never guessed.
//
//	c, err := money.ParseBRL("1.234,56") // 123456
func ParseBRL(s string) (int64, error) {
	in := strings.TrimSpace(s)
	sign := ""
	if strings.HasPrefix(in, "-") {
		sign, in = "-", in[1:]
	}
	in = strings.TrimSpace(strings.TrimPrefix(in, "R$"))
	m := brlNumber.FindStringSubmatch(in)
	if m == nil {
		return 0, fmt.Errorf("money: %q is not a valid amount; use the format 1.234,56", s)
	}
	cents := (m[2] + "00")[:2]
	c, err := strconv.ParseInt(sign+strings.ReplaceAll(m[1], ".", "")+cents, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: %q is out of range", s)
	}
	return c, nil
}

// FormatBRL renders centavos as "R$ 1.234,56" or "-R$ 50,00".
func FormatBRL(c int64) string {
	sign := ""
	abs := uint64(c)
	if c < 0 {
		sign, abs = "-", uint64(-(c+1))+1 // safe for math.MinInt64
	}
	whole := strconv.FormatUint(abs/100, 10)
	var b strings.Builder
	for i, d := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, b.String(), abs%100)
}
