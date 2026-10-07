# e08s03 — Carry-over and overspending

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S08

## 1. Business narrative

An envelope doesn't empty itself at the end of the month. If R$ 50 of Combustível was left in January, February starts with it. If January was overspent by R$ 20, February starts R$ 20 short. **A distribuir** becomes the real "money with no job yet". It includes the balances the user started with, and it never forgets past months. The user can always check: A distribuir plus everything available in the categories equals the money in on-budget accounts.

## 2. Actors

- **User**: on `/orcamento`.

## 3. Preconditions

- e08s02 is done.

## 4. Trigger

The user opens any month in `/orcamento`.

## 5. Main flow

For month *m*, `planning.Month` computes:

1. **Expense category:**
   - `CarryOver` = the sum of (amount − spent) over all months before *m*. A positive value is a leftover and a negative value is overspending.
   - `Available` = `Amount + CarryOver − Spent`.
2. **Money in** (cumulative through *m*) = the income received on all income accounts, plus the **Saldo inicial** inflows (opening balances and balance adjustments).
3. **A distribuir** = `Money in − every amount assigned to expense categories through m − Sem categoria spending through m`. Spending without a category isn't covered by any envelope, so it comes straight out of A distribuir.
4. The page shows **"Do mês anterior"** on each expense row with a non-zero carry-over (in red when negative).

## 6. Alternative flows

- **6a. Editing a past month** (an amount or a transaction) changes every later month on the next page load, because nothing is stored (P4).
- **6b. A month that was never opened has no amounts**, so its spending is uncovered and carries as overspending. Opening a month copies the latest opened month (e05s03).

## 7. Business rules

- **Overspending carries in the category.** It reduces the same envelope next month rather than A distribuir. This is the simplest rule that keeps the identity below.
- **Identity:** `A distribuir + Σ Available (expense categories) = Σ balances of on-budget asset and liability accounts`. This holds when there are no off-budget accounts (e08s05) and no archived categories with history (e08s06).
- **P4** Everything here is computed, never stored.

## 8. Data

No schema change. There's one new budget query, a sum of amounts per category before a month.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*budget.Service).AmountsBefore(ctx, userID string, m Month) (map[string]int64, error)` | Sum per category of the amounts in months before *m* |
| `CategoryPlan.CarryOver int64` | New field |
| `MonthPlan.Received` | Now includes Saldo inicial inflows (field kept; documented as "money in") |
| `MonthPlan.ToBeAssigned` | Now cumulative, minus Sem categoria spending |

## 10. Non-functional

One more aggregate query on budget and one on ledger (the activity before *m*). There are no loops over months.

## 11. Security & privacy

Unchanged: everything is scoped by user, and the isolation test remains.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Leftovers and overspending carry into the category's next month
#### MODIFIED: A distribuir
**Before (e08s02):** income received this month − assigned this month.
**After:** cumulative money in (income + Saldo inicial) − cumulative assigned − cumulative Sem categoria spending.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `planning.Month` | It stays the single place for the formulas. The identity test guards them as a whole. |

## 15. Implementation steps

1. `budget.AmountsBefore`. → verify: `go test -race ./budget/... -run TestAmountsBefore`
2. `planning.Month` adds carry-over, cumulative A distribuir, Saldo inicial and Sem categoria spending, and the identity test. → verify: `go test -race ./planning/... -run 'TestMonth|TestCarry'`
3. Orçamento shows "Do mês anterior". → verify: `go test -race ./web/... -run TestBudgetMonthPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Changing how A distribuir is defined.** The identity test pins the whole model.
- **Archived or deleted categories** take their carry-over with them. e08s06 handles this through merging.

## 17. Acceptance criteria

```gherkin
Feature: Carry-over

  Scenario: Leftover
    Given Combustível R$ 300,00 in January and R$ 250,00 spent
    And Combustível R$ 300,00 in February
    Then February shows Do mês anterior R$ 50,00 and Disponível R$ 350,00

  Scenario: Overspending
    Given Combustível R$ 300,00 in January and R$ 320,00 spent
    Then February shows Do mês anterior -R$ 20,00

  Scenario: Identity
    Then A distribuir + all Disponível = the on-budget account balances
```

## 18. Out of scope

- Card payment envelopes (e08s04) and off-budget accounts (e08s05).
- A YNAB-style option where overspending resets the category and reduces A distribuir.

## 19. Verification Script (Step-by-Step)

1. Create a checking account with an opening balance of R$ 1.000,00 dated last month, and map Posto Shell to Combustível.
2. Last month: set Combustível to 300,00 and record 250,00 at Posto Shell. A distribuir shows R$ 700,00.
3. This month: Combustível shows **Do mês anterior R$ 50,00** and Disponível R$ 350,00 (the amount copies over as 300,00). A distribuir shows R$ 400,00.
4. Check the identity: A distribuir (400) + Disponível (350) = R$ 750,00, which is the account balance.
5. Record 100,00 more last month. This month's carry-over shows **-R$ 50,00** (in red).
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S08.
- Decisions: `specs/planning-context.yaml` → `domain.planning` ("available = amount + carry-over - spent; leftovers carry over; overspending reduces next month"; "editing past months recomputes carry-over"), P4.
- Code and tests carry `// story: e08s03`.
