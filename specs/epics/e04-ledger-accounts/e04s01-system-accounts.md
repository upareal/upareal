# e04s01 — Ledger system accounts on sign-up

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S07

> **Re-planned 2026-10-07** after the ledger, budget and planning split (`specs/planning-context.yaml` → `key_decisions`). The first version also put groups and categories in the ledger. Those move to `budget` in e05s01.

## 1. Business narrative

Every user's ledger needs a few accounts that the app itself relies on: one to balance opening balances, and two to hold imported lines until the user assigns them. They're created automatically and quietly on sign-in, so the user never has to set up plumbing.

## 2. Actors

- **User**: signs in. They never see the system accounts as editable items.

## 3. Preconditions

- e03 is done.
- Decisions: the ledger holds money movement only, with its own per-user data key; defaults come from a repeatable `EnsureDefaults`; new users get only the system accounts in the ledger.

## 4. Trigger

A successful sign-in (`/auth/callback`).

## 5. Main flow

1. After `users.SignIn`, the callback calls `ledger.EnsureDefaults(userID)` before creating the session.
2. In one write transaction, `EnsureDefaults` creates the user's ledger data key if it's missing, plus any missing system account:
   - `Saldo inicial` (equity, role `opening_balance`)
   - `A categorizar` (income, role `uncategorized_income`)
   - `A categorizar` (expense, role `uncategorized_expense`)
3. `ledger.Accounts(ctx, userID)` returns the user's active accounts, decrypted and ordered by type, then position. System accounts are marked with their role.

## 6. Alternative flows

- **6a. Repeated sign-in:** nothing changes.
- **6b. Failure in the middle:** the transaction rolls back and the next sign-in completes it. The callback shows the error page and creates no session.
- **6c. Concurrent first sign-ins:** `_txlock=immediate` serializes them, so exactly one set is created.

## 7. Business rules

- Each user has exactly one account per system role (unique index).
- Account names are personal text, so they're sealed with the ledger's per-user key.
- **L7** (system accounts can't be deleted or archived) is enforced when deletion and archiving exist (e04s03).

## 8. Data

Module `ledger`, goose table `goose_ledger`. The migration `00001_ledger.sql` is **rewritten**, not stacked: it was never merged or deployed. Local databases from the earlier version must be deleted.

| Table | Columns |
|-------|---------|
| `ledger_user_key` | `user_id TEXT PK`, `data_key BLOB NOT NULL`, `created_at TEXT NOT NULL` |
| `ledger_account` | `id TEXT PK`, `user_id TEXT NOT NULL`, `type TEXT NOT NULL CHECK (type IN ('asset','liability','income','expense','equity'))`, `name BLOB NOT NULL`, `position INTEGER NOT NULL`, `system_role TEXT CHECK (...)`, `archived_at TEXT`, `created_at TEXT NOT NULL`; unique `(user_id, system_role) WHERE system_role IS NOT NULL` |

There's no `ledger_group` table and no `group_id` column.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*ledger.Service).EnsureDefaults(ctx, userID) error` | Repeatable. Data key plus system accounts. |
| `(*ledger.Service).Accounts(ctx, userID) ([]ledger.Account, error)` | `Account{ID, Type, Name, SystemRole}`, active only, ordered by type (asset, liability, income, expense, equity), then position |
| `/categorias` | **Removed here.** It returns in e05s01, backed by `budget`. |

## 10. Non-functional

On later sign-ins, `EnsureDefaults` does one key read and one role list.

## 11. Security & privacy

- Names are sealed with field `"name"` and the ledger key, with AAD bound to the user ID.
- Isolation test: user B never gets user A's accounts.
- Proof test: the database copy contains no plaintext "Saldo inicial" or "A categorizar".

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Ledger system accounts
A user's first sign-in creates the ledger data key and the three system accounts. Repeated sign-ins change nothing.

#### ADDED: R2: Account listing
`Accounts` returns the user's active accounts, decrypted and ordered.

#### MODIFIED: Sign-in callback
**Before:** the callback signed in and created a session.
**After:** it runs `ledger.EnsureDefaults` first. A failure shows the error page and creates no session.

#### REMOVED: Groups and categories in the ledger
**Before:** the first version of e04s01 stored groups and categories in `ledger_group` and `ledger_account.group_id`.
**After:** (removed). Categories belong to `budget` (e05s01) and planning maps accounts to them (e08s01).

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.Service.userKey` | One place unwraps or creates the module's per-user key. Every ledger read and write needs it. |
| `ledger/defaults.go` | System accounts as plain data. |

## 15. Implementation steps

1. Rewrite the ledger migration and queries without groups. → verify: `make gen && go build ./... && go test -race ./internal/db/... ./cmd/upareal/...`
2. `EnsureDefaults` creates only the data key and system accounts, repeatably and safely under concurrency. → verify: `go test -race ./ledger/... -run TestEnsureDefaults`
3. `Accounts`: decrypted, ordered, active only, with user isolation. → verify: `go test -race ./ledger/... -run 'TestAccounts|TestIsolation'`
4. Web: the callback keeps `EnsureDefaults`; the `/categorias` page and the home link are removed until e05s01. → verify: `go test -race ./web/... -run 'TestGoogleSignIn|TestSystemAccountsOnSignIn'`
5. Proof: the database copy contains no plaintext system account names. → verify: `go test -race ./ledger/... -run TestLedgerHasNoPlaintext`
6. Full gate. → verify: `make preflight`

## 16. Risks

- **Stale local databases** from the first e04s01 version fail on startup (goose sees version 1 already applied with a different schema). Mitigation: delete `/tmp/upareal.db*`. Nothing was deployed.

## 17. Acceptance criteria

```gherkin
Feature: Ledger system accounts

  Scenario: First sign-in creates system accounts
    Given a user who has never signed in
    When they sign in
    Then their ledger has exactly "Saldo inicial" (equity), "A categorizar" (income) and "A categorizar" (expense)

  Scenario: Repeated sign-in changes nothing
    Given a user with system accounts
    When they sign in again
    Then they still have exactly three accounts

  Scenario: Isolation
    Given users A and B signed in
    When B lists accounts
    Then none of A's accounts are returned
```

## 18. Out of scope

- Creating user accounts, opening balances, transactions (e04s02).
- Organizing, archiving and deleting accounts, and L5–L7 enforcement (e04s03).
- Budget categories (e05s01).

## 19. Verification Script (Step-by-Step)

1. Run `rm -f /tmp/upareal.db*`, export the variables and run `make run`.
2. Sign in and accept the terms. The home page works, with no "Categorias" link yet.
3. Stop the app. If sqlite3 is installed, run `sqlite3 /tmp/upareal.db "select type, system_role from ledger_account"`. It shows the three system accounts.
4. Sign in again and repeat step 3. There are still three rows.
5. Run `strings /tmp/upareal.db | grep -i -E "saldo inicial|a categorizar"`. There's no output.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S07.
- Decisions: `specs/planning-context.yaml` → `domain.ledger`, `domain.invariants.ledger`, `key_decisions` (split, per-module key, EnsureDefaults, `_txlock=immediate`).
- Code and tests carry `// story: e04s01`.
