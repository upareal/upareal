# e02s01 — Encrypted field round trip with a database-copy proof

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S02

## 1. Business narrative

Users trust upareal with their financial life. If the database file or a backup leaks, an attacker must find no readable names, emails or descriptions. This story builds the single helper that every module uses to encrypt personal text. It proves the guarantee with a test that reads the raw SQLite file.

## 2. Actors

- **Module code** (users, ledger, planning): seals and opens personal text, and computes lookup indexes.
- **Operator**: provides the master key through configuration.

## 3. Preconditions

- e01 is done. `internal/db.Open` exists.
- Go 1.26 stdlib provides `crypto/hkdf` and `cipher.NewGCMWithRandomNonce`.

## 4. Trigger

Module code needs to store or read a personal text field.

## 5. Main flow

1. At startup, the app parses the master key: base64, exactly 32 bytes.
2. It derives two purpose keys from the master key with HKDF-SHA256: a **wrap key** (`upareal/v1/wrap`) and an **index key** (`upareal/v1/index`).
3. When a user is created, the module asks for a new random 32-byte **data key**. It stores the data key only in **wrapped** form: encrypted with the wrap key, prefixed with the key version, and bound to the user ID.
4. To write a field, the module unwraps the user's data key and seals the text with AES-256-GCM. The AAD is `userID + 0x00 + fieldName`.
5. To read a field, the module opens it with the same user ID and field name.
6. To look a value up without decrypting (email at sign-in, import IDs), the module stores `HMAC-SHA256(indexKey, purpose + 0x00 + value)`.

## 6. Alternative flows

- **6a. Master key malformed** (not base64, or not 32 bytes): parsing fails with an error that states the expected format. The key itself never appears in the error.
- **6b. Wrong user or field**: opening a ciphertext with a different user ID or field name fails. A swapped value can't be read silently.
- **6c. Tampered ciphertext or wrapped key**: open or unwrap fails.
- **6d. Unknown key version** in a wrapped key: unwrap fails with an error naming the version.

## 7. Business rules

- Each user has a separate data key. Deleting the wrapped key makes that user's data unreadable, including in backups (crypto-shredding, used in e08 and e09).
- The index key is global. Lookups such as "find user by email" work across users.
- Callers normalize values before indexing (for example, lowercase emails). The helper indexes the bytes it receives.
- Amounts and dates are not encrypted (`specs/planning-context.yaml`).

## 8. Data

| Value | Format |
|-------|--------|
| Master key | base64 (std) of 32 random bytes |
| Wrapped data key | `version (1 byte) ‖ nonce (12) ‖ ciphertext(32) ‖ tag (16)` = 61 bytes |
| Sealed field | `nonce (12) ‖ ciphertext ‖ tag (16)` |
| Blind index | 32 bytes (HMAC-SHA256) |

No tables are added in this story. Modules add their own columns from e03 on.

## 9. Interfaces

Package `internal/vault`. It isn't named `crypto`, to avoid shadowing the stdlib `crypto/...` imports.

| Function | Contract |
|----------|----------|
| `ParseMasterKey(b64 string) ([]byte, error)` | Validates and decodes. |
| `NewKeyring(master []byte) (*Keyring, error)` | Derives the wrap and index keys. |
| `(*Keyring).NewDataKey(userID string) (*DataKey, []byte, error)` | Returns a usable key and its wrapped bytes. |
| `(*Keyring).UnwrapDataKey(userID string, wrapped []byte) (*DataKey, error)` | |
| `(*DataKey).Seal(field, plaintext string) ([]byte, error)` | The data key remembers its user ID. |
| `(*DataKey).Open(field string, sealed []byte) (string, error)` | |
| `(*Keyring).BlindIndex(purpose, value string) []byte` | |

## 10. Non-functional

- Only the stdlib is used: `crypto/aes`, `crypto/cipher`, `crypto/hkdf`, `crypto/hmac`, `crypto/sha256`, `crypto/rand`.
- Each seal or open costs microseconds, so no caching is needed at personal-budget scale.

## 11. Security & privacy

- AES-256-GCM with random 96-bit nonces (`NewGCMWithRandomNonce`). With one key per user, the number of messages per key stays far below the 2^32 random-nonce limit.
- Separate keys per purpose (HKDF info labels) keep the wrap key and the index key independent.
- The AAD binds each ciphertext to its user and field, so ciphertexts can't be swapped across users or columns. Swaps between rows of the same user and field stay possible. That's acceptable for v1 and is listed in Risks.
- Errors and logs never contain key material or plaintext.
- **Threat model limit:** this protects a leaked file, snapshot or backup. It does not protect against a compromised running host (`specs/planning-context.yaml`).

## 12. Dependencies (slopcheck)

| Package | Tag | Evidence |
|---------|-----|----------|
| stdlib `crypto/hkdf` | [OK] | `func Key[Hash hash.Hash](h func() Hash, secret, salt []byte, info string, keyLength int) ([]byte, error)` (go doc, go1.26.5) |
| stdlib `crypto/cipher.NewGCMWithRandomNonce` | [OK] | "generates a random 96-bit nonce, which is prepended to the ciphertext" (go doc) |

No third-party packages.

## 13. Requirements

#### ADDED: R1 — Master key validation
A malformed master key is rejected without revealing its value.

#### ADDED: R2 — Per-user data keys
Each user has a random data key, stored only wrapped and bound to the user ID, with a key version.

#### ADDED: R3 — Field encryption
Personal text is sealed with AES-256-GCM and bound to the user and field.

#### ADDED: R4 — Blind index
Equal (purpose, value) pairs produce equal indexes. Different purposes or values produce different indexes.

#### ADDED: R5 — No plaintext at rest
A database file written through the helper contains none of the plaintext values.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `Keyring` | Hides key derivation and wrapping behind 3 methods. Callers never handle raw purpose keys. |
| `DataKey` | Carries its user ID, so every seal and open automatically binds the right AAD. Callers can't forget it. |

No interfaces are added. The concrete types are the only implementation.

**Interpretation (default chosen):** the master key is **not** added to `config.Load` in this story. Nothing in the running app encrypts yet. `MASTER_KEY` becomes a required config value in e03s01, together with the first encrypted column, and that story also decides how local dev and compose supply a dev key.

## 15. Implementation steps

1. `ParseMasterKey` validates base64 and 32 bytes, with errors that never include the input. → verify: `go test -race ./internal/vault/... -run TestParseMasterKey`
2. `NewKeyring` derives the wrap and index keys with HKDF. Different labels give different keys, and the same master key gives the same keys. → verify: `go test -race ./internal/vault/... -run TestKeyring`
3. `NewDataKey` / `UnwrapDataKey` wrap with the version byte, bound to the user ID. Wrong user, tampering or an unknown version fails. → verify: `go test -race ./internal/vault/... -run TestDataKeyWrap`
4. `Seal` / `Open` work with AES-GCM and AAD (user, field). Round trip passes, including empty and unicode strings. Wrong field, wrong user or tampering fails. → verify: `go test -race ./internal/vault/... -run TestSealOpen`
5. `BlindIndex` is deterministic and purpose-separated, and differs across master keys. → verify: `go test -race ./internal/vault/... -run TestBlindIndex`
6. Proof: write sealed rows and indexes to a temp SQLite file via `db.Open`, close it, then scan the raw `.db`, `-wal` and `-shm` bytes for the plaintexts. → verify: `go test -race ./internal/vault/... -run TestDatabaseCopyHasNoPlaintext`
7. Full gate. → verify: `make preflight`

## 16. Risks

- **Swaps between rows of the same user and field are not detected.** Decision (2026-10-07): deferred, not needed for v1. Revisit before e05 stores descriptions (`specs/planning-context.yaml`).
- **Losing the master key loses all data.** Mitigation: operations require an offline backup (README, e10 runbook).
- **The plaintext proof may pass for the wrong reason** (for example, the rows were never written). Mitigation: the test first asserts the rows round-trip through SQL, then scans the file.

## 17. Acceptance criteria

```gherkin
Feature: Encrypted personal fields

  Scenario: Round trip
    Given a keyring from a valid master key and a data key for user "u1"
    When "Maria Silva" is sealed as field "name"
    Then opening it for user "u1" and field "name" returns "Maria Silva"

  Scenario: Ciphertext bound to user and field
    Given a value sealed for user "u1" and field "name"
    When it is opened as field "email" or with user "u2"'s data key
    Then opening fails

  Scenario: Malformed master key
    Given a master key that is 31 bytes or not base64
    When it is parsed
    Then parsing fails and the error does not contain the key

  Scenario: Database copy has no plaintext
    Given rows written with sealed names, emails and descriptions plus blind indexes
    When the SQLite file and its WAL files are read as raw bytes
    Then none of the plaintext values appear
```

## 18. Out of scope

- Adding `MASTER_KEY` to config, compose or `make run` (e03s01).
- Storing wrapped keys or encrypted columns in module tables (e03 onward).
- Master key rotation (only the version byte is stored now).
- Crypto-shredding flows (e08s02, e09s02).
- SSM loading (e10s01).

## 19. Verification Script (Step-by-Step)

1. Run `go test -race -v ./internal/vault/...`. All tests pass, including `TestDatabaseCopyHasNoPlaintext`.
2. Run `go test -race -run TestDatabaseCopyHasNoPlaintext -v ./internal/vault/...` and read the log. It names the plaintext values it searched for and the files it scanned.
3. Optional manual check: run `UPAREAL_KEEP_DB=1 go test -run TestDatabaseCopyHasNoPlaintext -v ./internal/vault/...`. The test prints the kept DB path. Then run `strings <path> | grep -i -E "maria|silva|mercado"` and expect no output.
4. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S02.
- Decisions: `specs/planning-context.yaml` → `privacy_security`, `key_decisions` (field-level encryption, HKDF).
- Prior art: `specs/product/PRIOR_ART_LATEST.md` → field encryption (build on stdlib).
- Code and tests carry `// story: e02s01`.
