# e10s01 — Export my data

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S21

## 1. Business narrative

LGPD (art. 18) gives every person the right to access their data and take it elsewhere. In **Meus dados**, the user downloads one file with everything upareal keeps about them: profile, accounts, every transaction with its legs and edit history, import lines, the budget, and the account-to-category mappings. All text is readable.

## 2. Actors

- **User**: signed in, on `/conta/dados`.

## 3. Preconditions

- e01 to e09 are done.

## 4. Trigger

The user clicks **Baixar meus dados**.

## 5. Main flow

1. Each module exports its own rows for the user, decrypted, with JSON tags:
   - `users.ExportUserData(ctx, id)`: id, name, email, created_at, terms_version, terms_accepted_at, disabled_at;
   - `ledger.ExportUserData`:
     - accounts (including archived and system accounts);
     - transactions (including deleted ones), each with legs `{account_id, amount_centavos}`;
     - history entries, each with its decrypted snapshot JSON;
     - import lines, each with its decrypted `raw` JSON;
   - `budget.ExportUserData`: groups, categories (including archived), opened months, amounts;
   - `planning.ExportUserData`: mappings.
2. `GET /conta/dados/exportar` answers `200` with:
   - `Content-Type: application/json`;
   - `Content-Disposition: attachment; filename="upareal-dados-AAAA-MM-DD.json"` (the São Paulo date);
   - `Cache-Control: no-store`;
   - a body of `{"exported_at", "users", "ledger", "budget", "planning"}`.
3. `/conta/dados` explains what's in the file, has the download button, and is linked from the home page as **Meus dados**.

## 6. Alternative flows

- **6a. Signed out:** redirected home.
- **6b. An export error:** 500 with no partial file. The whole document is built in memory before anything is written.

## 7. Business rules

- **Only the user's own data, every module (X2).** Each module exports only its own tables.
- **Not exported:** wrapped data keys, blind-index hashes (`sub_idx`, import `id_idx`) and session tokens. These are internal security material, not information about the person.
- Amounts are centavos (int64). Times are stored UTC RFC 3339 strings.
- Logs record `user_id` only.

## 8. Data

No schema change. New read queries in each module.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*users/ledger/budget/planning.Service).ExportUserData(ctx, userID string) (<module>.Export, error)` | Decrypted; JSON tags |
| `GET /conta/dados`, `GET /conta/dados/exportar` | Signed in |

## 10. Non-functional

A person's data is small, so it's built in memory and needs no streaming.

## 11. Security & privacy

- The export only ever contains the requester's own data. Tests check that user B's data never appears.
- `no-store`, so no cache keeps the file.

## 12. Dependencies (slopcheck)

None new (`encoding/json`).

## 13. Requirements

#### ADDED: R1: Download all my data as one readable JSON file

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ExportUserData` per module | Each module knows its own tables and keys (X1). Completeness is enforced by a test that lists every `user_id` table from the live schema, the same guard as `DeleteUserData`. |

## 15. Implementation steps

1. `ledger.ExportUserData`, with a completeness check against every `ledger_` table and isolation. → verify: `go test -race ./ledger/... -run TestExportUserData`
2. `users`, `budget` and `planning` `ExportUserData`, with completeness and isolation. → verify: `go test -race ./users/... ./budget/... ./planning/... -run TestExportUserData`
3. Web: the `/conta/dados` page and the download, linked from home. → verify: `go test -race ./web/... -run TestMyData`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **A new table is left out of the export.** The completeness test fails until the table is exported or explicitly listed as security material.

## 17. Acceptance criteria

```gherkin
Feature: Export my data

  Scenario: Download
    Given Maria has accounts, transactions and a budget
    When she clicks Baixar meus dados
    Then she downloads upareal-dados-<date>.json with all of it readable
    And none of João's data
```

## 18. Out of scope

- CSV or other formats.
- Deleting the account (e10s02).

## 19. Verification Script (Step-by-Step)

1. Sign in and open **Meus dados** from the home page.
2. Click **Baixar meus dados**. A file `upareal-dados-<today>.json` downloads.
3. Open it. Check that your name and email, your account names, transaction descriptions and amounts (in centavos), your categories and amounts, and your mappings are readable.
4. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S21.
- Decisions: `specs/planning-context.yaml` → `domain.users` ("self-service export my data and delete my account in v1"), `privacy_security` (LGPD; logs only user IDs), X1 and X2.
- Code and tests carry `// story: e10s01`.
