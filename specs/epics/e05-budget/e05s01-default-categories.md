# e05s01 — Default grouped categories and categories page

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S06

## 1. Business narrative

A new user starts budgeting immediately with a sensible pt-BR set of category groups and categories, such as "Moradia → Aluguel ou financiamento". The categories describe **intent**: what the user plans to spend or earn. They don't touch real money, which lives in the ledger.

## 2. Actors

- **User**: signs in and views categories.

## 3. Preconditions

- e04 is done.
- Decisions: the `budget` module owns categories and groups, with its own per-user data key. Defaults come from a repeatable `EnsureDefaults`. The default list is 10 groups and 37 categories (`specs/planning-context.yaml`).

## 4. Trigger

A successful sign-in, and visits to `/categorias`.

## 5. Main flow

1. The sign-in callback calls `budget.EnsureDefaults(userID)` after `ledger.EnsureDefaults` and before creating the session.
2. In one write transaction, `EnsureDefaults` creates the budget data key if it's missing. If the user has **no groups**, it also creates the default groups and categories in the order of `budget/defaults.go`.
3. `/categorias` lists the active groups in order, each with its active categories in order, with names decrypted.
4. The home page links to "Categorias".

## 6. Alternative flows

- **6a. Repeated sign-in:** nothing changes.
- **6b. Concurrent first sign-ins:** `_txlock=immediate` lets exactly one set be created.
- **6c. Signed out:** `/categorias` redirects to `/`.

## 7. Business rules

- **B1** There is one level of groups. A category belongs to exactly one group.
- **B2** A category's kind comes from its group and is never stored separately, so it can't disagree.
- Names are personal text, sealed with the **budget's** per-user key.

## 8. Data

Module `budget`, goose table `goose_budget`, prefix `budget_`:

| Table | Columns |
|-------|---------|
| `budget_user_key` | `user_id TEXT PK`, `data_key BLOB NOT NULL`, `created_at TEXT NOT NULL` |
| `budget_group` | `id TEXT PK`, `user_id TEXT NOT NULL`, `kind TEXT NOT NULL CHECK (kind IN ('expense','income'))`, `name BLOB NOT NULL`, `position INTEGER NOT NULL`, `archived_at TEXT`, `created_at TEXT NOT NULL` |
| `budget_category` | `id TEXT PK`, `user_id TEXT NOT NULL`, `group_id TEXT NOT NULL REFERENCES budget_group(id)`, `name BLOB NOT NULL`, `position INTEGER NOT NULL`, `archived_at TEXT`, `created_at TEXT NOT NULL` |

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `budget.NewService(conn, kr)` | |
| `(*budget.Service).EnsureDefaults(ctx, userID) error` | Repeatable |
| `(*budget.Service).Categories(ctx, userID) ([]budget.Group, error)` | `Group{ID, Kind, Name, Categories []Category{ID, Name}}` |
| `GET /categorias` | Signed-in page |

## 10. Non-functional

On a repeated sign-in, the check costs one key read and one count.

## 11. Security & privacy

- Names are sealed with field `"name"` using the budget key.
- An isolation test confirms user B never sees user A's groups.
- A proof test confirms the database file contains no plaintext default names.

## 12. Dependencies (slopcheck)

None new. This restores the code removed from the ledger in e04s01 (commits `9fb3c68`, `c9f1cdc`, `4612388`) into the new module.

## 13. Requirements

#### ADDED: R1: Budget module with default grouped categories
#### ADDED: R2: Categories page
#### MODIFIED: Sign-in callback
**Before:** ran `ledger.EnsureDefaults` before the session.
**After:** also runs `budget.EnsureDefaults`. A failure shows the error page and creates no session.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `budget.Service` | Owns the budget tables and the budget key. web and planning never touch them directly. |
| `budget/defaults.go` | The default list kept as data. |

**Known duplication:** `inTx`, `userKey`, `stamp` and `newSealedRow` (~40 lines) repeat what `ledger` has. Each module keeps its own copy so it stays splittable without a shared helper package. It will be extracted if a third module (planning) needs the same code.

## 15. Implementation steps

1. Budget schema, sqlc, `EnsureDefaults` (key, groups and categories; repeatable and concurrency-safe), and `run()` migrates `budget`. → verify: `go test -race ./budget/... -run TestEnsureDefaults`
2. `Categories`: ordered, decrypted, active only. Includes the isolation test and the no-plaintext proof. → verify: `go test -race ./budget/... -run 'TestCategories|TestIsolation|TestBudgetHasNoPlaintext'`
3. Web: the callback ensures budget defaults, `/categorias` is served, and home links to it. → verify: `go test -race ./web/... -run TestCategoriesPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- None new. The code was already proven in the first e04s01, including concurrency.

## 17. Acceptance criteria

```gherkin
Feature: Budget defaults

  Scenario: First sign-in seeds budget defaults
    When a new user signs in
    Then they have 10 groups and 37 categories in the budget

  Scenario: Categories page
    When the user opens /categorias
    Then "Moradia" comes first with "Aluguel ou financiamento" first inside it
    And "Receitas" comes last

  Scenario: Isolation
    Given users A and B with defaults
    When B lists categories
    Then none of A's IDs are returned
```

## 18. Out of scope

- Creating, renaming and moving categories (e05s02).
- Monthly amounts (e05s03).
- Mapping ledger accounts to categories (e08s01).

## 19. Verification Script (Step-by-Step)

1. Run `make run`, sign in (an existing user works too, because `EnsureDefaults` fills in what's missing), and open **Categorias**.
2. Confirm the groups appear in order, from Moradia to Receitas, each with its categories.
3. Sign out and back in. There are no duplicates.
4. Run `strings /tmp/upareal.db | grep -i -E "moradia|mercado|aluguel"`. It prints nothing.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S06.
- Decisions: `specs/planning-context.yaml` → `domain.budget`, `domain.invariants.budget` (B1, B2), `key_decisions` (split, per-module key, EnsureDefaults).
- Code and tests carry `// story: e05s01`.
