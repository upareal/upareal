// story: e04s01 e04s02 e06s01
package ledger_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/upareal/upareal/ledger"
)

// TestLedgerHasNoPlaintext proves a copy of the database reveals no account
// names or transaction descriptions.
func TestLedgerHasNoPlaintext(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.EnsureDefaults(ctx, userA); err != nil {
		t.Fatal(err)
	}
	acc, err := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Conta Itaú Personnalité", Type: "asset", Opening: 250000, OpeningAt: jan10})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.AdjustBalance(ctx, userA, acc.ID, 248000, jan10); err != nil {
		t.Fatal(err)
	}
	shell, err := f.svc.CreateAccount(ctx, userA, ledger.NewAccount{Name: "Posto Shell Ipiranga", Type: "expense"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RecordSimple(ctx, userA, ledger.SimpleEntry{AccountID: acc.ID, CounterID: shell.ID, Amount: 15000, At: jan10, Description: "Gasolina aditivada viagem"}); err != nil {
		t.Fatal(err)
	}
	if accounts, err := f.svc.Accounts(ctx, userA); err != nil || len(accounts) != wantSystem+2 || accounts[0].Name != "Conta Itaú Personnalité" {
		t.Fatalf("round trip = %+v, %v; the scan below would prove nothing", accounts, err)
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
	for _, name := range []string{"Saldo inicial", "A categorizar", "Conta Itaú Personnalité", "Ajuste de saldo", "Posto Shell Ipiranga", "Gasolina aditivada viagem"} {
		if bytes.Contains(raw, []byte(name)) {
			t.Errorf("database files contain %q in plaintext", name)
		}
	}
}
