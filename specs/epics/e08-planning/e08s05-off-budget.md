# e08s05 — Off-budget accounts in planning

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S08, S24

## 1. Business narrative

Investments, a car loan or an FGTS balance are accounts the user tracks but doesn't budget day to day. Their own movements (yield, an opening balance, a purchase paid straight from the investment) must not inflate or drain the budget. Money that crosses the line does count: sending R$ 1.000 from checking to an investment is spending, under **Investimentos** if the user mapped the account there or under Sem categoria otherwise. Redeeming R$ 300 back into checking is money to assign.

## 2. Actors

- **User**: on `/planejamento/mapa` and `/orcamento`.

## 3. Preconditions

- e08s01 to e08s04 are done. Off-budget accounts are asset or liability accounts with `on_budget = 0` (e04s02).

## 4. Trigger

The user opens any month, or maps an off-budget account.

## 5. Main flow

1. `ledger.BudgetFlows(ctx, userID, from, to)` returns, per account that **isn't on-budget**, the sum of its positive legs (`Debit`) and of its negative legs (`Credit`). Only live transactions in `[from, to)` that **also have a leg on an on-budget account** count.
2. `planning.Month` uses these flows (instead of raw activity) for categories, Sem categoria and money in:
   - **expense account:** `Debit + Credit` is spent (mapped category or Sem categoria).
   - **income or equity account:** `−(Debit + Credit)` is money in.
   - **off-budget asset or liability account:**
     - `Debit` (money that left the budget) is spent: in its mapped expense category, otherwise Sem categoria.
     - `−Credit` (money that came back) is money in, and so becomes money to assign.
3. **Mapping:** an off-budget asset or liability account can be mapped to an **expense** category (P2 extended). `Mappings` and the Mapa page list it.
4. Card balances and Cash still come from the raw activity of on-budget accounts.

## 6. Alternative flows

- **6a. An off-budget account's opening balance, yield or its own expenses** touch no on-budget account, so they're ignored.
- **6b. Switching an account between on- and off-budget** recomputes history using the current flag (P4). A mapping that no longer fits the account's kind is ignored (X1).
- **6c. Mapping an off-budget account to an income category:** `ErrKindMismatch`.

## 7. Business rules

- **Identity (now with off-budget accounts):** `A distribuir + Σ Disponível + Σ Para pagar = Dinheiro`. It holds for any mix of on- and off-budget accounts, because every transaction that touches the budget is counted through its non-budget legs.
- **P2** (extended): income → income category; expense → expense category; off-budget asset or liability → expense category.

## 8. Data

No schema change. There's one new ledger query, `BudgetFlows`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*ledger.Service).BudgetFlows(ctx, userID string, from, to time.Time) (map[string]ledger.Flow, error)` | `Flow{Debit, Credit int64}` (Debit ≥ 0, Credit ≤ 0), for non-on-budget accounts only |
| `SetMapping` / `Mappings` | Also accept and list off-budget asset and liability accounts |

## 10. Non-functional

One aggregate query per range replaces `Activity` in the tally. `Activity` stays for balances.

## 11. Security & privacy

Unchanged. The query filters by `user_id`, and there's an isolation test.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Only flows across the budget boundary count in planning
#### ADDED: R2: Off-budget accounts map to expense categories; outflows are spent, inflows are money to assign

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.BudgetFlows` | The ledger knows which accounts are on-budget, so it decides what crosses the boundary. Planning just classifies the result. |

## 15. Implementation steps

1. `ledger.BudgetFlows`. → verify: `go test -race ./ledger/... -run TestBudgetFlows`
2. Planning: off-budget mapping, tally from the flows, and the identity with off-budget accounts. → verify: `go test -race ./planning/...`
3. The Mapa lists off-budget accounts, and Orçamento shows their spending. → verify: `go test -race ./web/... -run 'TestMappingPage|TestBudgetMonthPage'`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **A transaction that has an on-budget leg, an off-budget leg and an expense leg.** Both non-budget legs count. That's correct, because both are where the budget money went.

## 17. Acceptance criteria

```gherkin
Feature: Off-budget accounts

  Scenario: Investing is spending
    Given "Investimento XP" off-budget mapped to "Investimentos"
    When R$ 1.000,00 is transferred from Itaú to it
    Then Investimentos shows Gasto R$ 1.000,00

  Scenario: Redeeming is money to assign
    When R$ 300,00 is transferred back to Itaú
    Then A distribuir grows by R$ 300,00

  Scenario: Off-budget life is ignored
    Given the investment's opening balance and yield
    Then neither changes A distribuir
```

## 18. Out of scope

- Net worth view (balances page already lists off-budget accounts).
- Goals.

## 19. Verification Script (Step-by-Step)

1. Create **Investimento XP** in Contas (asset, **without** "conta do orçamento"), with an opening balance of R$ 5.000,00. A distribuir doesn't change.
2. In **Mapa**, Investimento XP appears and offers only expense categories. Map it to **Investimentos**.
3. Transfer 1.000,00 from Itaú to Investimento XP. In Orçamento, Investimentos shows Gasto R$ 1.000,00.
4. Transfer 300,00 from Investimento XP back to Itaú. A distribuir grows by R$ 300,00.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S08, S24.
- Decisions: `specs/planning-context.yaml` → `domain.ledger` (on_budget flag; off-budget only in balances), `domain.planning` ("off-budget transfers need a category; inflows from off-budget become money to assign"), P2.
- Code and tests carry `// story: e08s05`.
