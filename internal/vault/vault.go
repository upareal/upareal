// story: e02s01

// Package vault encrypts personal text so a leaked database file or backup is
// unreadable. Every module MUST write sensitive text through this package.
//
// Key hierarchy: master key (config) → HKDF → wrap key + index key.
// Each user has a random data key, stored only wrapped by the wrap key.
// Threat model: protects data at rest, not a compromised running host.
package vault

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// keySize is the size of every symmetric key in this package (AES-256, HMAC-SHA256).
const keySize = 32

// HKDF info labels. Changing one changes every derived key: never edit, add a v2 instead.
const (
	wrapKeyInfo  = "upareal/v1/wrap"
	indexKeyInfo = "upareal/v1/index"
)

// Keyring holds the purpose keys derived from the master key.
type Keyring struct {
	wrapKey  []byte // encrypts per-user data keys
	indexKey []byte // HMAC key for blind indexes
}

// NewKeyring derives the wrap and index keys from a 32-byte master key with
// HKDF-SHA256 and distinct info labels.
//
//	kr, err := vault.NewKeyring(master)
func NewKeyring(master []byte) (*Keyring, error) {
	if len(master) != keySize {
		return nil, fmt.Errorf("vault: master key is %d bytes; expected %d", len(master), keySize)
	}
	wrapKey, err := hkdf.Key(sha256.New, master, nil, wrapKeyInfo, keySize)
	if err != nil {
		return nil, fmt.Errorf("vault: derive wrap key: %w", err)
	}
	indexKey, err := hkdf.Key(sha256.New, master, nil, indexKeyInfo, keySize)
	if err != nil {
		return nil, fmt.Errorf("vault: derive index key: %w", err)
	}
	return &Keyring{wrapKey: wrapKey, indexKey: indexKey}, nil
}

// ParseMasterKey decodes a base64 (standard encoding) master key of exactly
// 32 bytes. Errors never include the input.
//
//	master, err := vault.ParseMasterKey(os.Getenv("MASTER_KEY"))
func ParseMasterKey(b64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("vault: master key is not valid base64; expected base64 of %d random bytes (e.g. `openssl rand -base64 %d`)", keySize, keySize)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("vault: master key decodes to %d bytes; expected %d (e.g. `openssl rand -base64 %d`)", len(key), keySize, keySize)
	}
	return key, nil
}
