# e04s01 — Default grouped categories on sign-up

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S06

## 1. Business narrative

A new user should be able to start budgeting right away instead of building categories from nothing. On first sign-in, upareal gives them a sensible pt-BR set of category groups and categories, plus the hidden system accounts the ledger needs. They can see the set on a Categorias page.

## 2. Actors

- **User**: signs in and views their categories.

## 3. Preconditions

- e03 is done: sign-in, sessions and terms work.
- Decisions: each module has its own per-user data key; defaults come from a repeatable `EnsureDefaults` call; the default list is as proposed (`specs/planning-context.yaml`).

## 4. Trigger

A successful sign-in (`/auth/callback`), and later visits to `/categorias`.

## 5. Main flow

1. After `users.SignIn`, the callback calls `ledger.EnsureDefaults(userID)` before creating the session.
2. In one transaction, `EnsureDefaults`:
   1. creates the user's **ledger data key** (wrapped) if it's missing;
   2. if the user has **no groups**, creates the default groups with their categories (expense and income accounts) in the order of `ledger/defaults.go`;
   3. creates any missing **system accounts**: `Saldo inicial` (equity), `A categorizar` (income) and `A categorizar` (expense).
3. `/categorias` lists the user's groups in order, each with its categories in order. Names are decrypted, and system accounts are hidden.
4. The home page links to "Categorias".

## 6. Alternative flows

- **6a. Repeated sign-in:** `EnsureDefaults` changes nothing.
- **6b. A failed first sign-in:** the transaction rolls back and the next sign-in creates everything.
- **6c. Two first sign-ins at once:** `_txlock=immediate` makes the second transaction wait. It then sees the groups, and exactly one set exists.
- **6d. Signed out on `/categorias`:** the signed-out home page is shown (`appPage`). If terms aren't accepted, the user goes to `/termos/aceitar`.

## 7. Business rules

- Categories are `income` or `expense` accounts that belong to a group. Groups have a kind: `expense` or `income` (only "Receitas" is income).
- Each user has exactly one account per system role.
- Names are personal text, so they're always encrypted with the **ledger's** per-user data key.
- Ordering uses the plaintext `position` column.

## 8. Data

Module `ledger`, goose table `goose_ledger`, prefix `ledger_`:

| Table | Columns |
|-------|---------|
| `ledger_user_key` | `user_id TEXT PK`, `data_key BLOB NOT NULL` (wrapped), `created_at TEXT NOT NULL` |
| `ledger_group` | `id TEXT PK`, `user_id TEXT NOT NULL`, `kind TEXT NOT NULL CHECK (kind IN ('expense','income'))`, `name BLOB NOT NULL`, `position INTEGER NOT NULL`, `archived_at TEXT`, `created_at TEXT NOT NULL` |
| `ledger_account` | `id TEXT PK`, `user_id TEXT NOT NULL`, `type TEXT NOT NULL CHECK (type IN ('asset','liability','income','expense','equity'))`, `group_id TEXT REFERENCES ledger_group(id)`, `name BLOB NOT NULL`, `position INTEGER NOT NULL`, `system_role TEXT CHECK (system_role IN ('opening_balance','uncategorized_income','uncategorized_expense'))`, `archived_at TEXT`, `created_at TEXT NOT NULL`; `CHECK (group_id IS NULL OR type IN ('income','expense'))`; unique index on `(user_id, system_role) WHERE system_role IS NOT NULL` |

There's no foreign key to `users_user`, because cross-module references are never foreign keys.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.NewService(conn, kr)` | |
| `(*ledger.Service).EnsureDefaults(ctx, userID) error` | Repeatable |
| `(*ledger.Service).Categories(ctx, userID) ([]ledger.Group, error)` | `Group{ID, Kind, Name, Categories []Category{ID, Name}}`, in order, active only |
| `GET /categorias` | Signed-in app page listing groups and categories |
| `internal/db.Open` | DSN adds `_txlock=immediate` |

## 10. Non-functional

- `EnsureDefaults` is a single transaction (about 45 inserts on first run). On later sign-ins it does one key read and one count.

## 11. Security & privacy

- Names are sealed with field `"name"` and the ledger data key. The AAD binds them to the user ID.
- Every query filters by `user_id`. Isolation test: user B never sees user A's groups.
- Proof test: the database file contains no plaintext default names (for example "Mercado").

## 12. Dependencies (slopcheck)

None new. This reuses `internal/vault`, goose, sqlc and `google/uuid`.

## 13. Requirements

#### ADDED: R1: Default grouped categories
A user's first sign-in creates the default groups, categories and system accounts.

#### ADDED: R2: Categories page
`/categorias` shows the user's active groups and categories in order.

#### MODIFIED: Sign-in callback
**Before:** the callback signed the user in and created a session.
**After:** the callback also runs `ledger.EnsureDefaults(userID)` before creating the session. A failure shows the error page and creates no session.

#### MODIFIED: Database transactions
**Before:** deferred SQLite transactions.
**After:** `_txlock=immediate`, so write transactions lock up front.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.Service` | Owns ledger tables and the ledger data key. Web never touches ledger tables. |
| `ledger/defaults.go` | The default list as plain data, editable without touching logic. |
| `ledger.userKey(ctx, q, userID)` | One place unwraps the module's per-user key. Every later ledger read and write needs it. |

## 15. Implementation steps

1. `db.Open` uses `_txlock=immediate`. Ledger schema (goose) plus sqlc queries. `run()` migrates `ledger` after `users`. → verify: `make gen && go build ./... && go test -race ./internal/db/... ./cmd/upareal/...`
2. `EnsureDefaults`: key, default groups and categories, and system accounts. It's repeatable, and concurrent calls produce exactly one set. → verify: `go test -race ./ledger/... -run TestEnsureDefaults`
3. `Categories`: ordered, decrypted, active only, system accounts hidden. Isolation between users. → verify: `go test -race ./ledger/... -run 'TestCategories|TestIsolation'`
4. Web: the callback ensures defaults; `/categorias` page; home link. → verify: `go test -race ./web/... -run TestCategoriesPage`
5. Proof: the database copy contains no plaintext default names. → verify: `go test -race ./ledger/... -run TestLedgerHasNoPlaintext`
6. Full gate. → verify: `make preflight`

## 16. Risks

- **Tests that use the full web flow get slower,** because each sign-in now seeds about 45 rows. Detect: the web test time. That's acceptable at milliseconds.
- **Changing the defaults later doesn't affect existing users.** That's intended. It's documented in `defaults.go`.

## 17. Acceptance criteria

```gherkin
Feature: Default categories

  Scenario: First sign-in seeds defaults
    Given a user who has never signed in
    When they sign in
    Then they have the 10 default groups with their categories
    And one "Saldo inicial", one income "A categorizar" and one expense "A categorizar"

  Scenario: Repeated sign-in changes nothing
    Given a user with default categories
    When they sign in again
    Then the number of groups and accounts is unchanged

  Scenario: Categories page
    Given a signed-in user who accepted the terms
    When they open /categorias
    Then they see "Moradia" first, with "Aluguel ou financiamento" first inside it
    And they do not see "Saldo inicial" or "A categorizar"

  Scenario: Isolation
    Given users A and B with defaults
    When B lists categories
    Then only B's groups are returned
```

## 18. Out of scope

- Creating, renaming and reordering categories (e04s03).
- Asset and liability accounts and balances (e04s02).
- Archive, merge and delete (e04s04, e07s05).

## 19. Verification Script (Step-by-Step)

1. Run `rm -f /tmp/upareal.db*`, export the variables and run `make run`.
2. Sign in and accept the terms. The home page shows a "Categorias" link.
3. Open **Categorias**. You see the groups Moradia, Alimentação, …, Receitas, each with its categories in the listed order. There's no "Saldo inicial" or "A categorizar".
4. Sign out and sign in again. The list is unchanged, with no duplicates.
5. Run `strings /tmp/upareal.db | grep -i -E "mercado|moradia|aluguel"`. There's no output.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S06.
- Decisions: `specs/planning-context.yaml` → `domain.ledger`, `key_decisions` (per-module key, EnsureDefaults, `_txlock=immediate`).
- Code and tests carry `// story: e04s01`.
