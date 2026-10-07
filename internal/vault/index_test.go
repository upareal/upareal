// story: e02s01
package vault_test

import (
	"bytes"
	"testing"

	"github.com/upareal/upareal/internal/vault"
)

func TestBlindIndex_DeterministicPerMasterKey(t *testing.T) {
	kr, master := newKeyring(t)
	same, _ := vault.NewKeyring(master)
	a := kr.BlindIndex("email", "maria@example.com")
	b := same.BlindIndex("email", "maria@example.com")
	if len(a) != 32 {
		t.Fatalf("index length = %d, want 32", len(a))
	}
	if !bytes.Equal(a, b) {
		t.Error("same master, purpose and value gave different indexes")
	}
}

func TestBlindIndex_SeparatesInputs(t *testing.T) {
	kr, _ := newKeyring(t)
	other, _ := newKeyring(t)
	base := kr.BlindIndex("email", "maria@example.com")
	for name, idx := range map[string][]byte{
		"other value":                 kr.BlindIndex("email", "joao@example.com"),
		"other purpose":               kr.BlindIndex("import-id", "maria@example.com"),
		"other master":                other.BlindIndex("email", "maria@example.com"),
		"case is not normalized here": kr.BlindIndex("email", "Maria@example.com"),
	} {
		if bytes.Equal(base, idx) {
			t.Errorf("%s: index collides with the base index", name)
		}
	}
	if bytes.Contains(base, []byte("maria")) {
		t.Error("index contains the plaintext")
	}
}
