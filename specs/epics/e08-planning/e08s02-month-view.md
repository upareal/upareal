# e08s02 — Planning view for a month

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 8 · **scope:** S08, S18

## 1. Business narrative

The user opens **Orçamento** for a month and sees, per category, what was planned, what was really spent and what's left. At the top, **"A distribuir"** shows the income received that month minus everything assigned to expense categories. Money spent or received on accounts without a category appears under **"Sem categoria"**, which includes imported lines that haven't been categorized yet.

## 2. Actors

- **User**: on `/orcamento`.

## 3. Preconditions

- e05s03 (amounts per month) and e08s01 (mapping) are done.

## 4. Trigger

The user opens `/orcamento[?mes=YYYY-MM]`.

## 5. Main flow

1. `ledger.Activity(ctx, userID, from, to)` returns, per account, the sum of its legs on non-deleted transactions with `occurred_at` in `[from, to)` (the São Paulo month). The values are debit-positive.
2. `planning.Month(ctx, userID, month)` combines:
   - the budget amounts of the month (`budget.OpenMonth`);
   - the mappings (only mappings to an active category of the same kind count, as in X1);
   - the month's activity on **every** income and expense account, including archived and system accounts.
3. For each category it computes:
   - **Expense:** `Spent` = the sum of activity on its accounts (a refund lowers it). `Available` = `Amount − Spent`, and it goes negative when overspent.
   - **Income:** `Received` = minus the sum of activity on its accounts.
4. Totals:
   - **Sem categoria:** the spending and income on unmapped accounts (this covers P3, since no money is left out).
   - **Income received** = all income accounts, mapped or not.
   - **Assigned** = the sum of expense category amounts.
   - **A distribuir** = `Income received − Assigned`. In this story it's for this month only. e08s03 makes it cumulative.
5. The **Orçamento** page keeps its amount forms, and each row now also shows its numbers: expense rows show **Gasto** and **Disponível** (in red when negative), income rows show **Recebido**. The header shows **A distribuir**. A **Sem categoria** section links to the **Mapa**.

## 6. Alternative flows

- **6a. A month with no activity:** spent, available and received equal the amounts, or zero.
- **6b. A transfer between two asset accounts:** it touches no income or expense account, so it changes nothing here.

## 7. Business rules

- **P3** Unmapped money is shown as Sem categoria, never dropped.
- **P4** Everything here is computed on every request. Nothing is stored.
- Months are São Paulo calendar months (B3), and a transaction belongs to the month of its `occurred_at` in São Paulo.

## 8. Data

No schema change. There's one new ledger query, `ActivityBetween` (a sum per account over a range).

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*ledger.Service).Activity(ctx, userID string, from, to time.Time) (map[string]int64, error)` | Debit-positive sum per account. Accounts with no legs are left out. |
| `(*planning.Service).Month(ctx, userID string, m budget.Month) (MonthPlan, error)` | `MonthPlan{Categories map[string]CategoryPlan; UnmappedSpent, UnmappedReceived, Received, Assigned, ToBeAssigned int64}`; `CategoryPlan{Amount, Spent, Available, Received int64}` |
| `GET /orcamento` | Same route, more columns |

## 10. Non-functional

A month view runs 4 module reads plus one aggregate query. There's no N+1.

## 11. Security & privacy

- Every query filters by `user_id`, and planning works only from IDs and sums. The isolation test covers it.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Spent, available and received per category for a month
#### ADDED: R2: To be assigned = income received − assigned (this month)
#### ADDED: R3: Sem categoria totals include unmapped and uncategorized (A categorizar) money

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `planning.Month` | The single place the envelope formulas live. e08s03 to e08s05 extend it, and the web layer only formats its output. |

## 15. Implementation steps

1. `ledger.Activity`: covers the range bounds, deleted transactions and isolation. → verify: `go test -race ./ledger/... -run TestActivity`
2. `planning.Month`: spent, available, received, Sem categoria and A distribuir, plus isolation. → verify: `go test -race ./planning/... -run TestMonth`
3. The Orçamento page shows the new columns, A distribuir and Sem categoria. → verify: `go test -race ./web/... -run TestBudgetMonthPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **The sign convention.** Expense activity is positive when spent, and income activity is negative when received. Tests pin both, including a refund.

## 17. Acceptance criteria

```gherkin
Feature: Month view

  Scenario: Spent and available
    Given Combustível has R$ 300,00 in January and Posto Shell maps to it
    When R$ 120,00 is spent at Posto Shell in January
    Then Combustível shows Gasto R$ 120,00 and Disponível R$ 180,00

  Scenario: A distribuir
    Given R$ 8.000,00 salary received in January and R$ 300,00 assigned
    Then A distribuir is R$ 7.700,00

  Scenario: Uncategorized import
    Given an imported line of −R$ 50,00 still in the review queue
    Then Sem categoria shows R$ 50,00 spent
```

## 18. Out of scope

- Carry-over, overspending from earlier months, and a cumulative A distribuir (e08s03).
- Credit card payment envelopes (e08s04).
- Off-budget accounts (e08s05).

## 19. Verification Script (Step-by-Step)

1. Map Posto Shell to **Combustível** and Salário to **Salário** (Mapa).
2. In **Orçamento** for this month, set Combustível to 300,00.
3. Record an expense of 120,00 at Posto Shell and an income of 8.000,00 from Salário (Lançamentos).
4. In Orçamento, check that Combustível shows Gasto R$ 120,00 and Disponível R$ 180,00, Salário shows Recebido R$ 8.000,00, and A distribuir shows R$ 7.700,00.
5. Record 50,00 on an unmapped expense account. Sem categoria should show R$ 50,00.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S08, S18.
- Decisions: `specs/planning-context.yaml` → `domain.planning` (spent, to be assigned, available), invariants P3 and P4.
- Code and tests carry `// story: e08s02`.
