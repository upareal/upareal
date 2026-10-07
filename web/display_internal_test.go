// story: e04s02
package web

import "testing"

// Stored balances are debit-positive; people read debts as amounts owed and
// income as amounts earned.
func TestDisplayBalance(t *testing.T) {
	for _, tc := range []struct {
		typ    string
		stored int64
		want   string
	}{
		{"asset", 250000, "R$ 2.500,00"},
		{"asset", -5000, "-R$ 50,00"}, // overdrawn
		{"liability", -80000, "R$ 800,00 a pagar"},
		{"liability", 1000, "R$ 10,00 a receber"}, // card with credit
		{"liability", 0, "R$ 0,00"},
		{"income", -500000, "R$ 5.000,00"},
		{"expense", 15000, "R$ 150,00"},
	} {
		if got := displayBalance(tc.typ, tc.stored); got != tc.want {
			t.Errorf("displayBalance(%s, %d) = %q, want %q", tc.typ, tc.stored, got, tc.want)
		}
	}
}
