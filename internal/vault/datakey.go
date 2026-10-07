// story: e02s01
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// wrapVersion identifies the master key generation that wrapped a data key.
// Rotation adds version 2; unwrap picks the key by this byte.
const wrapVersion byte = 1

// DataKey is one user's field-encryption key. It remembers its user ID so
// every Seal/Open binds the right associated data.
type DataKey struct {
	userID string
	aead   cipher.AEAD
}

// NewDataKey creates a random data key for userID and returns it with its
// wrapped form (version ‖ nonce ‖ ciphertext ‖ tag, 61 bytes) for storage.
//
//	dk, wrapped, err := kr.NewDataKey(userID) // store wrapped with the user
func (k *Keyring) NewDataKey(userID string) (*DataKey, []byte, error) {
	if userID == "" {
		return nil, nil, errors.New("vault: NewDataKey needs a non-empty user ID to bind the key to")
	}
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		return nil, nil, fmt.Errorf("vault: generate data key: %w", err)
	}
	wrapper, err := newAEAD(k.wrapKey)
	if err != nil {
		return nil, nil, err
	}
	wrapped := wrapper.Seal([]byte{wrapVersion}, nil, raw, wrapAAD(wrapVersion, userID))
	dk, err := newDataKey(userID, raw)
	if err != nil {
		return nil, nil, err
	}
	return dk, wrapped, nil
}

// UnwrapDataKey restores the data key that NewDataKey wrapped for userID.
// It fails on a different user, a different master key, tampering or an
// unknown version.
func (k *Keyring) UnwrapDataKey(userID string, wrapped []byte) (*DataKey, error) {
	if len(wrapped) == 0 {
		return nil, errors.New("vault: wrapped data key is empty")
	}
	if v := wrapped[0]; v != wrapVersion {
		return nil, fmt.Errorf("vault: wrapped data key has unknown version %d; this build knows version %d", v, wrapVersion)
	}
	wrapper, err := newAEAD(k.wrapKey)
	if err != nil {
		return nil, err
	}
	raw, err := wrapper.Open(nil, nil, wrapped[1:], wrapAAD(wrapVersion, userID))
	if err != nil {
		return nil, errors.New("vault: cannot unwrap data key (wrong user, wrong master key, or corrupted)")
	}
	return newDataKey(userID, raw)
}

// wrapAAD binds a wrapped key to its version and owner.
func wrapAAD(version byte, userID string) []byte {
	return append([]byte{version}, userID...)
}

func newDataKey(userID string, raw []byte) (*DataKey, error) {
	aead, err := newAEAD(raw)
	if err != nil {
		return nil, err
	}
	return &DataKey{userID: userID, aead: aead}, nil
}

// newAEAD returns AES-256-GCM that generates a random 96-bit nonce per Seal
// and prepends it to the ciphertext.
func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault: aes: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("vault: gcm: %w", err)
	}
	return aead, nil
}
