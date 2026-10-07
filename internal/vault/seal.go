// story: e02s01
package vault

import (
	"errors"
	"fmt"
)

// Seal encrypts plaintext for this key's user with AES-256-GCM. The result
// (nonce ‖ ciphertext ‖ tag) only opens with the same user and field name.
// Row binding is deferred (specs/planning-context.yaml).
//
//	sealed, err := dk.Seal("name", "Maria Silva")
func (d *DataKey) Seal(field, plaintext string) ([]byte, error) {
	if field == "" {
		return nil, errors.New("vault: Seal needs a non-empty field name to bind the value to")
	}
	return d.aead.Seal(nil, nil, []byte(plaintext), d.fieldAAD(field)), nil
}

// Open decrypts a value sealed by Seal for the same user and field.
//
//	name, err := dk.Open("name", sealed)
func (d *DataKey) Open(field string, sealed []byte) (string, error) {
	plaintext, err := d.aead.Open(nil, nil, sealed, d.fieldAAD(field))
	if err != nil {
		return "", fmt.Errorf("vault: cannot open field %q (wrong user, wrong field, or corrupted)", field)
	}
	return string(plaintext), nil
}

// fieldAAD binds a sealed value to its owner and column: userID ‖ 0x00 ‖ field.
func (d *DataKey) fieldAAD(field string) []byte {
	aad := make([]byte, 0, len(d.userID)+1+len(field))
	aad = append(aad, d.userID...)
	aad = append(aad, 0)
	return append(aad, field...)
}
