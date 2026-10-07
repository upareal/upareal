# e05s03 — Monthly amounts

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S17

## 1. Business narrative

A user decides how much they allow or expect for each category in a month, for example "Combustível, janeiro, R$ 300,00" or "Salário, janeiro, R$ 8.000,00". Next month starts from the same plan, so they only change what's different. The budget records intent only. Comparing it with real money is planning's job (e08).

## 2. Actors

- **User**: on `/orcamento`.

## 3. Preconditions

- e05s02 is done.

## 4. Trigger

The user opens a month or changes an amount.

## 5. Main flow

1. `/orcamento` shows the **current month in São Paulo** time. `/orcamento?mes=2026-02` shows another month. Links lead to the previous and next months.
2. **Opening a month** for the first time copies every category amount from the **most recent earlier opened month** of the user, then marks the month as opened. Later visits don't copy again.
3. The page lists the active groups and categories, each with its amount for the month and an inline form to change it. It also shows totals per group, and separate totals for expenses and income.
4. Saving an amount replaces the one for that category and month.

## 6. Alternative flows

- **6a. No earlier opened month:** the month opens with every amount at zero.
- **6b. Negative or malformed amount:** 400 with "Valor inválido. Use o formato 1.234,56." (B4)
- **6c. Malformed month** (for example `2026-13`): 400.
- **6d. Another user's category ID:** 404 (X2).
- **6e. Months opened later aren't changed retroactively.** Editing January after February was opened doesn't change February.

## 7. Business rules

- **B3:** at most one amount per category and month. Months are calendar months in America/Sao_Paulo, written as `YYYY-MM`.
- **B4:** amounts are `int64` centavos and at least 0.
- An amount of zero is stored, so "deliberately zero" is kept distinct from "never set".

## 8. Data

Migration `budget/migrations/00002_amounts.sql`:

| Table | Columns |
|-------|---------|
| `budget_month` | `user_id TEXT`, `month TEXT CHECK (month GLOB '[0-9][0-9][0-9][0-9]-[01][0-9]')`, `opened_at TEXT NOT NULL`, `PRIMARY KEY (user_id, month)` |
| `budget_amount` | `user_id TEXT NOT NULL`, `category_id TEXT NOT NULL REFERENCES budget_category(id)`, `month TEXT NOT NULL`, `amount INTEGER NOT NULL CHECK (amount >= 0)`, `PRIMARY KEY (category_id, month)` |

Amounts are plaintext, as decided for all amounts.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `budget.ParseMonth(s) (Month, error)` / `budget.MonthOf(t) Month` / `Month.String()`, `Prev()`, `Next()` | `Month` is a validated `YYYY-MM` |
| `(*Service).OpenMonth(ctx, userID, Month) (map[string]int64, error)` | Opens the month if needed (copying amounts), and returns category ID → amount |
| `(*Service).SetAmount(ctx, userID, categoryID string, Month, amount int64) error` | Opens the month first |
| Errors | `ErrInvalidAmount`, `ErrInvalidMonth`, `ErrNotFound` |
| `GET /orcamento[?mes=YYYY-MM]`, `POST /orcamento/{mes}/{categoria}` (`valor`) | 303 back to the same month |

## 10. Non-functional

Opening a month is one transaction. It copies at most a few dozen rows.

## 11. Security & privacy

- Every query filters by `user_id`, and setting an amount checks that the category belongs to the user.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new. This reuses `internal/money` for BRL.

## 13. Requirements

#### ADDED: R1: Amount per category and month (B3, B4)
#### ADDED: R2: A new month copies the most recent earlier opened month
#### ADDED: R3: Month budget page with totals

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `budget.Month` | One validated type for "calendar month in São Paulo", so the e08 planning work can't mix month formats. |
| `budget_month` table | Tells "opened with zeros" apart from "never opened", so copying happens exactly once. |

## 15. Implementation steps

1. `Month` type, schema, `SetAmount` (B3, B4) and `OpenMonth` reads, with isolation. → verify: `go test -race ./budget/... -run 'TestMonth|TestMonthlyAmounts'`
2. The first open copies the latest earlier opened month, later opens never copy again, and a month opened with zeros stays zero. → verify: `go test -race ./budget/... -run TestCopyPreviousMonth`
3. The `/orcamento` page: month navigation, inline amount forms and totals. → verify: `go test -race ./web/... -run TestBudgetMonthPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **A month opened by accident** (for example, browsing far ahead) gets a copy of the plan. That's harmless: the amounts can be edited, and planning only reads them.

## 17. Acceptance criteria

```gherkin
Feature: Monthly amounts

  Scenario: Set an amount
    When the user sets "Combustível" in 2026-01 to "300,00"
    Then 2026-01 shows R$ 300,00 for Combustível

  Scenario: Next month starts from the plan
    Given 2026-01 has Combustível R$ 300,00
    When the user opens 2026-02 for the first time
    Then 2026-02 shows R$ 300,00 for Combustível

  Scenario: Copy happens once
    Given 2026-02 was opened
    When the user changes 2026-01 to R$ 350,00
    Then 2026-02 still shows R$ 300,00

  Scenario: Negative amount
    When the user sets "-10,00"
    Then it is rejected
```

## 18. Out of scope

- Spent, available, carry-over and to be assigned (e08).
- Goals.

## 19. Verification Script (Step-by-Step)

1. Run `make run`, sign in and open **Orçamento**. The current month opens.
2. Set Combustível to `300,00` and Salário to `8.000,00`. The totals update.
3. Click **Próximo mês**. Combustível already shows R$ 300,00.
4. Go back and change Combustível to `350,00`, then go forward again. The next month still shows R$ 300,00.
5. Try `-10,00`. You see "Valor inválido".
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S17.
- Decisions: `specs/planning-context.yaml` → `domain.budget` (amount per category per month, copy previous month), `domain.invariants.budget` (B3, B4).
- Code and tests carry `// story: e05s03`.
