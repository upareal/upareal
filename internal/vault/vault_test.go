// story: e02s01
package vault_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/upareal/upareal/internal/vault"
)

func randomKeyB64(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func TestParseMasterKey_Accepts32Bytes(t *testing.T) {
	b64 := randomKeyB64(t, 32)
	key, err := vault.ParseMasterKey(b64)
	if err != nil {
		t.Fatalf("ParseMasterKey: %v", err)
	}
	want, _ := base64.StdEncoding.DecodeString(b64)
	if !bytes.Equal(key, want) {
		t.Error("ParseMasterKey returned different bytes than encoded")
	}
}

func TestParseMasterKey_RejectsMalformedWithoutLeakingInput(t *testing.T) {
	for name, input := range map[string]string{
		"empty":      "",
		"31 bytes":   randomKeyB64(t, 31),
		"33 bytes":   randomKeyB64(t, 33),
		"not base64": "this-is-not-base64-!!!-secretvalue",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := vault.ParseMasterKey(input)
			if err == nil {
				t.Fatal("ParseMasterKey: expected error, got nil")
			}
			if input != "" && strings.Contains(err.Error(), input) {
				t.Errorf("error leaks the key: %q", err)
			}
			if !strings.Contains(err.Error(), "32") {
				t.Errorf("error %q does not state the expected 32-byte size", err)
			}
		})
	}
}
