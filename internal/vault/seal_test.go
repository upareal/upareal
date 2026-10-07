// story: e02s01
package vault_test

import (
	"bytes"
	"testing"

	"github.com/upareal/upareal/internal/vault"
)

func TestSealOpen_RoundTrip(t *testing.T) {
	kr, _ := newKeyring(t)
	dk, wrapped, err := kr.NewDataKey("u1")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := kr.UnwrapDataKey("u1", wrapped)
	if err != nil {
		t.Fatal(err)
	}
	for name, plaintext := range map[string]string{
		"ascii":   "Maria Silva",
		"unicode": "Pão de Açúcar — café ☕",
		"empty":   "",
		"long":    string(bytes.Repeat([]byte("x"), 10_000)),
	} {
		t.Run(name, func(t *testing.T) {
			sealed, err := dk.Seal("name", plaintext)
			if err != nil {
				t.Fatalf("Seal: %v", err)
			}
			if plaintext != "" && bytes.Contains(sealed, []byte(plaintext)) {
				t.Fatal("sealed output contains the plaintext")
			}
			got, err := restored.Open("name", sealed)
			if err != nil {
				t.Fatalf("Open with unwrapped key: %v", err)
			}
			if got != plaintext {
				t.Errorf("Open = %q, want %q", got, plaintext)
			}
		})
	}
}

func TestSealOpen_IsRandomized(t *testing.T) {
	kr, _ := newKeyring(t)
	dk, _, _ := kr.NewDataKey("u1")
	a, _ := dk.Seal("name", "Maria")
	b, _ := dk.Seal("name", "Maria")
	if bytes.Equal(a, b) {
		t.Error("sealing the same value twice gave identical bytes; nonces must be random")
	}
}

func TestSealOpen_RejectsMisuse(t *testing.T) {
	kr, _ := newKeyring(t)
	u1, _, _ := kr.NewDataKey("u1")
	u2, _, _ := kr.NewDataKey("u2")
	sealed, err := u1.Seal("name", "Maria Silva")
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)/2] ^= 0x01

	for name, tc := range map[string]struct {
		dk     *vault.DataKey
		field  string
		sealed []byte
	}{
		"wrong field":    {u1, "email", sealed},
		"other user key": {u2, "name", sealed},
		"tampered":       {u1, "name", tampered},
		"truncated":      {u1, "name", sealed[:10]},
		"empty":          {u1, "name", nil},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.dk.Open(tc.field, tc.sealed); err == nil {
				t.Error("Open: expected error, got nil")
			}
		})
	}
}

func TestSealOpen_RequiresFieldName(t *testing.T) {
	kr, _ := newKeyring(t)
	dk, _, _ := kr.NewDataKey("u1")
	if _, err := dk.Seal("", "x"); err == nil {
		t.Error("Seal with empty field name: expected error, got nil")
	}
}
