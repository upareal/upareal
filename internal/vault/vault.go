// story: e02s01

// Package vault encrypts personal text so a leaked database file or backup is
// unreadable. Every module MUST write sensitive text through this package.
//
// Key hierarchy: master key (config) → HKDF → wrap key + index key.
// Each user has a random data key, stored only wrapped by the wrap key.
// Threat model: protects data at rest, not a compromised running host.
package vault

import (
	"encoding/base64"
	"fmt"
)

// keySize is the size of every symmetric key in this package (AES-256, HMAC-SHA256).
const keySize = 32

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
