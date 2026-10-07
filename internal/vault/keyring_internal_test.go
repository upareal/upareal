// story: e02s01
package vault

import (
	"bytes"
	"testing"
)

// Key separation is a security property the public API cannot show, so this
// test inspects the derived keys directly.
func TestKeyring_DerivesSeparateDeterministicKeys(t *testing.T) {
	master := bytes.Repeat([]byte{7}, keySize)
	a, err := NewKeyring(master)
	if err != nil {
		t.Fatalf("NewKeyring: %v", err)
	}
	b, _ := NewKeyring(master)
	other, _ := NewKeyring(bytes.Repeat([]byte{8}, keySize))

	if len(a.wrapKey) != keySize || len(a.indexKey) != keySize {
		t.Fatalf("derived key sizes = %d, %d; want %d", len(a.wrapKey), len(a.indexKey), keySize)
	}
	if bytes.Equal(a.wrapKey, a.indexKey) {
		t.Error("wrap key equals index key; purposes must be separated")
	}
	if bytes.Equal(a.wrapKey, master) || bytes.Equal(a.indexKey, master) {
		t.Error("a derived key equals the master key")
	}
	if !bytes.Equal(a.wrapKey, b.wrapKey) || !bytes.Equal(a.indexKey, b.indexKey) {
		t.Error("same master key derived different keys; derivation must be deterministic")
	}
	if bytes.Equal(a.wrapKey, other.wrapKey) {
		t.Error("different master keys derived the same wrap key")
	}
}

func TestKeyring_RejectsWrongMasterSize(t *testing.T) {
	for _, n := range []int{0, 31, 33} {
		if _, err := NewKeyring(make([]byte, n)); err == nil {
			t.Errorf("NewKeyring(%d bytes): expected error, got nil", n)
		}
	}
}
