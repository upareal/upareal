# e13s01 — Public stats page

**type:** feat · **risk:** P1 · **context:** domain · **bcps:** 3 · **scope:** — (new, user request 2026-10-08)

## 1. Business narrative

Anyone can open **Estatísticas** and see how upareal is being used: how many people signed up, how many accounts, transactions and imported lines they keep, how many categories are in use, and how much is planned this month. It's a first version, plain numbers and nothing more.

## 2. Actors

- **Visitor**: anyone. No sign-in is needed.

## 3. Preconditions

- e01 to e12 are done.

## 4. Trigger

The visitor opens `/estatisticas`. It's linked in the footer of every page.

## 5. Main flow

1. Each module has one `PublicStats(ctx)` that returns a struct of `int64` aggregates. It returns no rows, no IDs and nothing decrypted:
   - `users.PublicStats` → `Users`;
   - `ledger.PublicStats` → `Accounts` (excluding system accounts), `Transactions` (excluding deleted), `ImportedLines`;
   - `budget.PublicStats(ctx, month)` → `Categories` (active), `PlannedCentavos` (the sum of every amount in that month).
2. `GET /estatisticas` renders the numbers in pt-BR (`1.234`, `R$ 1.234,56`), for the current São Paulo month. The values are computed on each request.

## 6. Alternative flows

- **6a. An empty database:** every number is 0.

## 7. Business rules

- **A decided exception to "every query filters by user_id" (X2):**
  - only the `PublicStats` queries may aggregate across users;
  - they're marked `-- PUBLIC STATS: aggregate across users`;
  - they return only counts and sums, and a test guards that every result field is an `int64`.
- **No minimum user count:** the user decided on 2026-10-08 that totals show at any number of users. The privacy policy has to say that aggregate totals are published (added to the legal-text item).

## 8. Data

No schema change. One aggregate query per figure.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*users.Service).PublicStats(ctx) (users.Stats, error)` | `Stats{Users int64}` |
| `(*ledger.Service).PublicStats(ctx) (ledger.Stats, error)` | `Stats{Accounts, Transactions, ImportedLines int64}` |
| `(*budget.Service).PublicStats(ctx, budget.Month) (budget.Stats, error)` | `Stats{Categories, PlannedCentavos int64}` |
| `GET /estatisticas` | Public |

## 10. Non-functional

There's no cache: a handful of `COUNT` and `SUM` queries is cheap for SQLite. Add a cache if the page gets real traffic.

## 11. Security & privacy

- Only numbers leave the modules, and nothing is decrypted.
- The page is public, so it adds no session or cookie requirement.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Public page with aggregate counts and the month's planned total
#### MODIFIED: X2 (every query filters by user_id)
**Before:** no exception.
**After:** the `PublicStats` aggregate queries are the only exception, and they return numbers only.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `PublicStats` per module | Each module counts only its own tables (X1). The web layer just formats the numbers. |

## 15. Implementation steps

1. `PublicStats` in users, ledger and budget. The numbers are tested across two users, and a test checks the fields are `int64` only. → verify: `go test -race ./users/... ./ledger/... ./budget/... -run TestPublicStats`
2. Web: the public `/estatisticas` page, linked in the footer. → verify: `go test -race ./web/... -run TestPublicStats`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **With very few users, totals describe individuals.** The user accepted this.

## 17. Acceptance criteria

```gherkin
Feature: Public stats

  Scenario: Visitor sees the numbers
    Given 2 users with 3 accounts, 5 transactions and R$ 300,00 planned this month
    When a visitor opens /estatisticas without signing in
    Then they see 2 usuários, 3 contas, 5 lançamentos and R$ 300,00 planejados
```

## 18. Out of scope

- Charts, history over time, a cache.

## 19. Verification Script (Step-by-Step)

1. Open `/estatisticas` in a private window, without signing in. The numbers appear.
2. Sign in, create an account and a transaction, and set an amount for this month. Reload the stats: the numbers go up.
3. Check that the footer of every page links to Estatísticas.
4. Run `make preflight`. It ends green.

## 20. Traceability

- Decisions: X2 exception (above), recorded in `specs/planning-context.yaml`, `AGENTS.md` and `CONVENTIONS.md`.
- Code and tests carry `// story: e13s01`.
