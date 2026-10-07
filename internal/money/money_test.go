// story: e04s02
package money_test

import (
	"math"
	"testing"

	"github.com/upareal/upareal/internal/money"
)

func TestBRL_ParseAcceptsPtBRForms(t *testing.T) {
	for in, want := range map[string]int64{
		"1.234,56":                  123456,
		"1234,56":                   123456,
		"1234":                      123400,
		"0,5":                       50,
		"0,05":                      5,
		"-50,00":                    -5000,
		"R$ 10,00":                  1000,
		"-R$ 1.000,00":              -100000,
		" 2.500,00 ":                250000,
		"1.000.000,00":              100000000,
		"0":                         0,
		"92.233.720.368.547.758,07": math.MaxInt64,
	} {
		got, err := money.ParseBRL(in)
		if err != nil || got != want {
			t.Errorf("ParseBRL(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

func TestBRL_ParseRejectsAmbiguousOrInvalid(t *testing.T) {
	for _, in := range []string{
		"", "   ", "abc", "12,345", "1,2,3", "1.23", "12.34,56", "1.2345,00",
		"10,00abc", "--5", "R$", "1 000,00",
		"92.233.720.368.547.758,08", // one centavo above int64
	} {
		if got, err := money.ParseBRL(in); err == nil {
			t.Errorf("ParseBRL(%q) = %d, nil; want error", in, got)
		}
	}
}

func TestBRL_Format(t *testing.T) {
	for in, want := range map[int64]string{
		0:             "R$ 0,00",
		5:             "R$ 0,05",
		123456:        "R$ 1.234,56",
		-5000:         "-R$ 50,00",
		100000000:     "R$ 1.000.000,00",
		math.MaxInt64: "R$ 92.233.720.368.547.758,07",
		math.MinInt64: "-R$ 92.233.720.368.547.758,08",
	} {
		if got := money.FormatBRL(in); got != want {
			t.Errorf("FormatBRL(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestBRL_RoundTrip(t *testing.T) {
	for _, c := range []int64{0, 1, -1, 99, 100, 123456, -987654321} {
		got, err := money.ParseBRL(money.FormatBRL(c))
		if err != nil || got != c {
			t.Errorf("round trip %d -> %q -> %d, %v", c, money.FormatBRL(c), got, err)
		}
	}
}
