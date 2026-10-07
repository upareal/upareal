// story: e04s01
package ledger_test

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestLedgerHasNoPlaintext proves a copy of the database reveals no account names.
func TestLedgerHasNoPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM ledger_account WHERE user_id = ?`, userA); n != wantSystem {
		t.Fatalf("accounts = %d; the scan below would prove nothing", n)
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
	if !bytes.Contains(raw, []byte("ledger_account")) {
		t.Fatal("control: schema text not found; the scan is not reading the database")
	}
	for _, name := range []string{"Saldo inicial", "A categorizar"} {
		if bytes.Contains(raw, []byte(name)) {
			t.Errorf("database files contain %q in plaintext", name)
		}
	}
}
