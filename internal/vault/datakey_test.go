// story: e02s01
package vault_test

import (
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/vault"
)

func newKeyring(t *testing.T) (*vault.Keyring, []byte) {
	t.Helper()
	master, err := vault.ParseMasterKey(randomKeyB64(t, 32))
	if err != nil {
		t.Fatal(err)
	}
	kr, err := vault.NewKeyring(master)
	if err != nil {
		t.Fatal(err)
	}
	return kr, master
}

func TestDataKeyWrap_UnwrapsForSameUserAndMaster(t *testing.T) {
	kr, master := newKeyring(t)
	_, wrapped, err := kr.NewDataKey("u1")
	if err != nil {
		t.Fatalf("NewDataKey: %v", err)
	}
	if len(wrapped) != 61 || wrapped[0] != 1 {
		t.Fatalf("wrapped key = %d bytes, version %d; want 61 bytes, version 1", len(wrapped), wrapped[0])
	}
	sameMaster, _ := vault.NewKeyring(master)
	if _, err := sameMaster.UnwrapDataKey("u1", wrapped); err != nil {
		t.Errorf("UnwrapDataKey with a keyring from the same master: %v", err)
	}
}

func TestDataKeyWrap_EachCallIsRandom(t *testing.T) {
	kr, _ := newKeyring(t)
	_, w1, _ := kr.NewDataKey("u1")
	_, w2, _ := kr.NewDataKey("u1")
	if string(w1) == string(w2) {
		t.Error("two NewDataKey calls produced identical wrapped keys")
	}
}

func TestDataKeyWrap_RejectsMisuse(t *testing.T) {
	kr, _ := newKeyring(t)
	otherMaster, _ := newKeyring(t)
	_, wrapped, err := kr.NewDataKey("u1")
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), wrapped...)
	tampered[len(tampered)-1] ^= 0x01
	unknownVersion := append([]byte(nil), wrapped...)
	unknownVersion[0] = 9

	for name, tc := range map[string]struct {
		kr      *vault.Keyring
		userID  string
		wrapped []byte
	}{
		"wrong user":      {kr, "u2", wrapped},
		"wrong master":    {otherMaster, "u1", wrapped},
		"tampered":        {kr, "u1", tampered},
		"unknown version": {kr, "u1", unknownVersion},
		"truncated":       {kr, "u1", wrapped[:20]},
		"empty":           {kr, "u1", nil},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.kr.UnwrapDataKey(tc.userID, tc.wrapped); err == nil {
				t.Error("UnwrapDataKey: expected error, got nil")
			}
		})
	}

	t.Run("unknown version is named", func(t *testing.T) {
		_, err := kr.UnwrapDataKey("u1", unknownVersion)
		if err == nil || !strings.Contains(err.Error(), "version 9") {
			t.Errorf("error = %v, want it to name version 9", err)
		}
	})
}

func TestDataKeyWrap_RequiresUserID(t *testing.T) {
	kr, _ := newKeyring(t)
	if _, _, err := kr.NewDataKey(""); err == nil {
		t.Error("NewDataKey(\"\"): expected error, got nil")
	}
}
