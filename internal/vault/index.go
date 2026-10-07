// story: e02s01
package vault

import (
	"crypto/hmac"
	"crypto/sha256"
)

// BlindIndex returns HMAC-SHA256(indexKey, purpose ‖ 0x00 ‖ value): a 32-byte
// lookup key that finds a row without decrypting and reveals nothing about value.
// purpose is a package constant (e.g. "email"); callers normalize value first.
//
//	idx := kr.BlindIndex("email", strings.ToLower(strings.TrimSpace(email)))
func (k *Keyring) BlindIndex(purpose, value string) []byte {
	mac := hmac.New(sha256.New, k.indexKey)
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return mac.Sum(nil)
}
