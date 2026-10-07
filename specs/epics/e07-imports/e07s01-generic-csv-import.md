# e07s01 — Generic CSV import into the review queue

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 8 · **scope:** S13, S15

## 1. Business narrative

A user exports a statement from their bank, arranges it in upareal's documented CSV format, and uploads it to an account. Every line becomes a transaction waiting in a **review queue** against "A categorizar". Uploading the same file again adds nothing. They no longer type each purchase by hand.

## 2. Actors

- **User**: on an account page.

## 3. Preconditions

- e06 is done.
- The format is recorded in `specs/planning-context.yaml` → `domain.import`.

## 4. Trigger

The user opens **Importar** on an asset or liability account and uploads a `.csv` file.

## 5. Main flow

1. The format (help page `/ajuda/csv` with an example) is UTF-8 with `;` as separator. The **first line is a header, which is required but ignored**. Columns are **by position**:

   | # | Column | Format | Required |
   |---|--------|--------|----------|
   | 1 | `id` | text, unique per account | yes |
   | 2 | `data` | `dd/mm/aaaa` | yes |
   | 3 | `hora` | `hh:mm`; empty = 00:00 | no (may be empty) |
   | 4 | `descricao` | text up to 200 characters | no |
   | 5 | `valor` | `1.234,56`; negative = money left the account | yes, not zero |
   | 6 | `conta` | free text, kept encrypted for future auto-categorization | no (column may be omitted) |

   A UTF-8 BOM, which Excel adds, is accepted. Times are in São Paulo time.
2. **Validation is all-or-nothing.** If any line is invalid, nothing is imported, and the page lists every bad line with its number and the reason.
3. For each line whose `id` was **not imported into this account before**, one transaction is created through `postTransaction`:
   - `valor < 0` (money out): `A categorizar` (expense) `+|v|`, account `−|v|`
   - `valor > 0` (money in): `A categorizar` (income) `−v`, account `+v`
4. An import record keeps the HMAC blind index of `account ID + id` (for duplicate detection) and the sealed `conta` text.
5. The user lands on the **review queue** (`/revisao`): every transaction that still has a leg on an "A categorizar" account, newest first, with the account, date, description, amount and `conta` text. A summary shows "N importadas, M já existentes".

## 6. Alternative flows

- **6a. File over 5 MB, or over 10.000 data lines:** 400, before any parsing work.
- **6b. Fewer than 5 columns, bad date, time or amount, zero amount, empty `id`, or the same `id` twice in one file:** 400 with line numbers. Nothing is imported.
- **6c. Account that isn't an asset or liability, is archived, or belongs to another user:** 404 or 400.
- **6d. Importing more than 10 times in a minute:** 429 (rate limit per user).
- **6e. A previously imported line that was deleted:** still counts as already imported. Re-importing doesn't bring it back.

## 7. Business rules

- L1–L3 and L6 hold through `postTransaction`.
- The duplicate key is **per account**. The same `id` in another account's file is a different line.
- The `conta` text and descriptions are personal, so they're sealed.
- The queue is derived from the ledger and never stored, so assigning an account (e07s02) removes a line from it automatically.

## 8. Data

Migration `ledger/migrations/00004_imports.sql`:

| Table | Columns |
|-------|---------|
| `ledger_import_line` | `id TEXT PK`, `user_id TEXT NOT NULL`, `account_id TEXT NOT NULL REFERENCES ledger_account(id)`, `transaction_id TEXT NOT NULL REFERENCES ledger_transaction(id)`, `id_idx BLOB NOT NULL` (blind index of `account ID ‖ 0x00 ‖ id`), `raw BLOB NOT NULL` (sealed JSON `{conta}`, field `"import"`), `created_at TEXT NOT NULL`; `UNIQUE (account_id, id_idx)` |

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger/importer.Line{ImportID string; At time.Time; Description string; Amount int64; Account string}` | `Amount` is signed, as seen from the account |
| `importer.ParseGenericCSV(r io.Reader) ([]Line, error)` | `error` is `importer.Errors` (a list of `{Line int, Reason string}`), or a size error |
| `importer.MaxLines = 10000`, `importer.MaxBytes = 5 << 20` | |
| `(*ledger.Service).Import(ctx, userID, accountID string, lines []importer.Line) (ImportResult{Imported, Skipped int}, error)` | All-or-nothing |
| `(*ledger.Service).ReviewQueue(ctx, userID) ([]QueuedLine, error)` | `QueuedLine{TransactionID, AccountID, AccountName; At; Description, Conta string; Amount int64}` |
| `GET /contas/{id}/importar`, `POST /contas/{id}/importar` (multipart `arquivo`) | |
| `GET /revisao`, `GET /ajuda/csv` | |

## 10. Non-functional

- 10.000 lines import in one transaction. That's about 30.000 inserts, which takes seconds on SQLite.
- Uploads are capped by `http.MaxBytesReader` (5 MB).

## 11. Security & privacy

- Parsing reads at most 5 MB. CSV fields are length-checked.
- The duplicate key is a blind index, so the bank's row IDs are never stored in plaintext.
- Imports are rate limited per user.

## 12. Dependencies (slopcheck)

None new. This uses the standard library's `encoding/csv` with `Comma = ';'` and `FieldsPerRecord = -1`.

## 13. Requirements

#### ADDED: R1: Generic CSV import with a positional format and all-or-nothing validation
#### ADDED: R2: Duplicates are skipped per account, even after a delete
#### ADDED: R3: Review queue of transactions still on "A categorizar"

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger/importer` package | Parsing (bytes to lines) is separate from posting (lines to the ledger), so OFX and bank parsers (e07s04/s05) only produce `[]Line`. |
| `importer.Errors` | Carries every bad line number, so the user fixes the file in one go. |

**No `Parser` interface yet:** there's one format. The interface comes with the second one (e07s04).

## 15. Implementation steps

1. `ParseGenericCSV`: header skipped, positional columns, BOM, optional `hora` and `conta`, every line's error reported, limits. → verify: `go test -race ./ledger/importer/... -run TestGenericCSV`
2. Migration and `Import`: blind-index duplicate check per account, all-or-nothing, posting against A categorizar, sealed `conta`. → verify: `go test -race ./ledger/... -run TestImportDedupe`
3. `ReviewQueue` (derived), with isolation. → verify: `go test -race ./ledger/... -run TestReviewQueue`
4. Web: import page, help page, upload (5 MB, per-user rate limit) and the `/revisao` page. → verify: `go test -race ./web/... -run TestImportFlow`
5. Proof that the `conta` text and import IDs aren't stored in plaintext. → verify: `go test -race ./ledger/... -run TestLedgerHasNoPlaintext`
6. Full gate. → verify: `make preflight`

## 16. Risks

- **Excel saves `;` CSV in Windows-1252, not UTF-8.** The help page tells users to choose "CSV UTF-8". Invalid UTF-8 is rejected with a clear message. Detect: the `TestGenericCSV` invalid-UTF-8 case.

## 17. Acceptance criteria

```gherkin
Feature: CSV import

  Scenario: Import a statement
    Given a CSV with two lines (-150,00 and 8.000,00)
    When the user imports it into Itaú
    Then two transactions appear in the review queue against A categorizar
    And Itaú's balance changes by +7.850,00

  Scenario: Re-import adds nothing
    When the same file is imported again
    Then "0 importadas, 2 já existentes"

  Scenario: One bad line blocks the file
    Given line 3 has date "31/02/2026"
    When the user imports it
    Then nothing is imported and the page says "Linha 3"
```

## 18. Out of scope

- Assigning accounts or splitting queued lines (e07s02).
- Transfer matching (e07s03).
- OFX and bank formats (e07s04/s05).
- Auto-categorization from `conta` (later).

## 19. Verification Script (Step-by-Step)

1. Open **Ajuda** (`/ajuda/csv`) and copy the example into `extrato.csv`, saved as UTF-8.
2. In **Contas → Itaú → Importar**, upload it. You land on **Revisão** with the lines, and "2 importadas".
3. Upload the same file again. You see "0 importadas, 2 já existentes".
4. Change one date to `31/02/2026` and upload. Nothing is imported, and the error names the line.
5. Itaú's register shows the imported lines against "A categorizar".
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S13, S15.
- Decisions: `specs/planning-context.yaml` → `domain.import` (generic CSV format, limits, all-or-nothing, dedupe), `privacy_security` (blind indexes for import IDs).
- Code and tests carry `// story: e07s01`.
