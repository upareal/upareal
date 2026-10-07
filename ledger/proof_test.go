// story: e04s01
package ledger_test

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestLedgerHasNoPlaintext proves a copy of the database reveals no group,
// category or system account names.
func TestLedgerHasNoPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if groups, err := f.svc.Categories(ctx, userA); err != nil || len(groups) == 0 || groups[0].Name != "Moradia" {
		t.Fatalf("round trip = %v, %v; the scan below would prove nothing", groups, err)
	}
	if err := f.conn.Close(); err != nil {
		t.Fatal(err)
	}

	var raw []byte
	for _, p := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
		if b, err := os.ReadFile(p); err == nil {
			raw = append(raw, b...)
		}
	}
	if !bytes.Contains(raw, []byte("ledger_group")) {
		t.Fatal("control: schema text not found; the scan is not reading the database")
	}
	for _, name := range []string{"Moradia", "Aluguel ou financiamento", "Mercado", "Receitas", "Salário", "Saldo inicial", "A categorizar"} {
		if bytes.Contains(raw, []byte(name)) {
			t.Errorf("database files contain %q in plaintext", name)
		}
	}
}
