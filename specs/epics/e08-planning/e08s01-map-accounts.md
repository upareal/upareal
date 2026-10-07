# e08s01 — Map ledger accounts to budget categories

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S18, S24

## 1. Business narrative

The ledger knows where money really went ("Posto Shell", "Posto Ipiranga"). The budget knows what was planned ("Combustível"). The user links them once: Posto Shell and Posto Ipiranga both feed Combustível. From then on, planning can compare plan against reality. Any account not linked yet shows up as "Sem categoria", so no money goes unnoticed.

## 2. Actors

- **User**: on `/planejamento/mapa`.

## 3. Preconditions

- e04 to e07 are done.
- Decisions: the mapping rules P1–P3 and X1, and v1 maps only income and expense accounts (`specs/planning-context.yaml`).

## 4. Trigger

The user opens **Mapa de categorias** and picks a category for an account.

## 5. Main flow

1. The page lists the user's active **expense** and **income** ledger accounts, leaving out system accounts. Each row shows its current category, or **"Sem categoria"**.
2. Each row has a picker containing **only categories of the same kind** (expense → expense categories, income → income categories), grouped by budget group, plus "Sem categoria". Saving stores the mapping.
3. `planning.SetMapping` checks that:
   - the account belongs to the user, is income or expense, and isn't a system account;
   - the category belongs to the user, is active, and matches the account's kind (**P2**).

   It then stores at most **one** category per account (**P1**). An empty category removes the mapping.
4. A counter at the top shows how many accounts are **Sem categoria** (**P3**).

## 6. Alternative flows

- **6a. Kind mismatch, unknown or foreign account or category:** 400 or 404, and nothing changes.
- **6b. The mapped category is archived or deleted, or the mapped account is archived or deleted:** listing ignores the mapping and the account shows "Sem categoria" (**X1**). The stale row is replaced the next time the user saves that account.

## 7. Business rules

- **P1** An account maps to at most one category. A category can have many accounts.
- **P2** Kinds must match.
- **P3** Unmapped accounts are shown as "Sem categoria".
- **X1** planning keeps IDs only. It reads ledger and budget only through their Go APIs and never queries their tables.

## 8. Data

Module `planning`, goose table `goose_planning`, prefix `planning_`:

| Table | Columns |
|-------|---------|
| `planning_mapping` | `user_id TEXT NOT NULL`, `account_id TEXT NOT NULL`, `category_id TEXT NOT NULL`, `updated_at TEXT NOT NULL`, `PRIMARY KEY (user_id, account_id)` |

There are no foreign keys, because the IDs belong to other modules. There's no personal text, so there's no data key.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `planning.NewService(conn, *ledger.Service, *budget.Service)` | |
| `(*planning.Service).SetMapping(ctx, userID, accountID, categoryID string) error` | `categoryID` `""` removes the mapping |
| `(*planning.Service).Mappings(ctx, userID) ([]Mapping, error)` | `Mapping{Account ledger.Account; CategoryID, CategoryName, GroupName string}`: one per active user income/expense account, with an empty category meaning Sem categoria |
| Errors | `ErrKindMismatch`, `ErrNotMappable`, `ErrNotFound` |
| `GET /planejamento/mapa`, `POST /planejamento/mapa/{conta}` (`categoria`) | |

## 10. Non-functional

A listing is two module reads (accounts and categories) plus one query.

## 11. Security & privacy

- Everything is scoped by user. planning stores no personal text.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Map an income/expense account to one category of the same kind
#### ADDED: R2: Show unmapped accounts as Sem categoria, including after their category is archived or deleted

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `planning.Service` | The only place that joins ledger and budget. It depends on their public services, so each can still split out (X1). |

## 15. Implementation steps

1. Planning module: migration, `SetMapping` (P1, P2, guards, isolation), and `run()` migrates planning. → verify: `go test -race ./planning/... -run TestMapping`
2. `Mappings`: unmapped shown as Sem categoria, and stale mappings ignored (X1, P3). → verify: `go test -race ./planning/... -run TestUnmapped`
3. Web: `/planejamento/mapa` with same-kind pickers and the Sem categoria count. → verify: `go test -race ./web/... -run TestMappingPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **planning depends on two module services.** That's intended: it's the joining module.

## 17. Acceptance criteria

```gherkin
Feature: Account to category mapping

  Scenario: Many accounts, one category
    When the user maps "Posto Shell" and "Posto Ipiranga" to "Combustível"
    Then both show Combustível

  Scenario: Kinds must match
    When the user maps "Salário" (income) to "Mercado" (expense category)
    Then it is rejected

  Scenario: Unmapped
    Given "Padaria" has no category
    Then it shows "Sem categoria" and the counter includes it
```

## 18. Out of scope

- Spent, available and to be assigned (e08s02).
- Mapping asset or liability accounts (open question; v1 doesn't).

## 19. Verification Script (Step-by-Step)

1. Create the expense accounts "Posto Shell" and "Posto Ipiranga" and the income account "Salário" (Contas).
2. Open **Planejamento → Mapa de categorias**. All three show "Sem categoria", and the counter says 3.
3. Map both postos to **Combustível**, and Salário to **Salário**. The counter drops to 0.
4. Check that the picker for Salário only lists income categories.
5. (Archiving categories comes in e08s06; the stale-mapping rule is covered by tests.)
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S18, S24.
- Decisions: `specs/planning-context.yaml` → `domain.planning`, `domain.invariants.planning` (P1–P3), `cross_module` (X1).
- Code and tests carry `// story: e08s01`.
