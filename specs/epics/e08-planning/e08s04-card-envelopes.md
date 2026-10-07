# e08s04 — Credit card payment envelopes

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 8 · **scope:** S19

## 1. Business narrative

Paying with the card shouldn't feel like free money. When the user buys R$ 45 of fuel on the Nubank card, Combustível drops by R$ 45, and that R$ 45 of cash stays **reserved to pay Nubank**. **Orçamento** shows each card's **"Para pagar"**. When the cash can't cover the card bill, because a category was overspent or A distribuir is negative, the page flags **"Sem cobertura"** with the amount missing.

## 2. Actors

- **User**: on `/orcamento`.

## 3. Preconditions

- e08s03 is done (carry-over and the identity).

## 4. Trigger

The user opens any month.

## 5. Main flow

For month *m*, with everything cumulative through the end of *m*:

1. **Para pagar (per card)** is the card's debt, meaning minus its balance. Card spending was already taken from its categories, and A distribuir already excludes the debt, so this cash is set aside without any new rule.
2. **Dinheiro** (cash) is the sum of the balances of on-budget accounts that aren't cards.
3. **Reservado para cartões** = `Dinheiro − max(A distribuir, 0) − Σ max(Disponível, 0)`.
4. **Sem cobertura** = `Σ Para pagar − Reservado`, clamped to between 0 and Σ Para pagar.
5. Paying the card is a transfer from checking to the card. It lowers Para pagar and Dinheiro by the same amount, so categories and A distribuir don't change.
6. Orçamento gets a **Cartões de crédito** section: one row per card with debt (name and Para pagar), and a red **"Sem cobertura: R$ X"** line when X > 0.

## 6. Alternative flows

- **6a. Opening card debt** (an opening balance of −R$ 500): A distribuir drops by R$ 500. That's the honest result, since that cash is already owed.
- **6b. A card with no debt:** not listed. No cards at all: no section.

## 7. Business rules

- **Identity (now with cards):** `A distribuir + Σ Disponível + Σ Para pagar = Dinheiro`. This holds when there are no off-budget accounts (e08s05).
- **P4** Everything is computed, never stored.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `MonthPlan.Cards []CardPlan` | `CardPlan{AccountID, Name string; Due int64}`: cards with debt at the end of *m* |
| `MonthPlan.Cash int64` | On-budget non-card balances at the end of *m* |
| `MonthPlan.Uncovered int64` | Sem cobertura |

## 10. Non-functional

No new queries. The balances come from the activity sums already loaded.

## 11. Security & privacy

Unchanged.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Para pagar per card and the Sem cobertura flag
#### MODIFIED: Credit card decision (`specs/planning-context.yaml` → domain.planning)
**Before:** "a virtual payment envelope per card sets aside money equal to the budgeted card spending; unfunded card debt is flagged".
**After:** the payment envelope is the card's debt. It's already set aside, because card spending reduces categories and A distribuir excludes the debt. The unfunded part is the debt that cash can't back after A distribuir and the positive envelopes. Overspending stays in its category (e08s03), so the YNAB per-transaction "funded portion" isn't tracked.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `planning.Month` | Card numbers are derived from the same cumulative activity, so there's no second source of truth. |

## 15. Implementation steps

1. `planning.Month`: Cards, Cash, Uncovered, and the identity extended with cards. → verify: `go test -race ./planning/... -run 'TestCards|TestCarry'`
2. Orçamento gets the Cartões de crédito section. → verify: `go test -race ./web/... -run TestBudgetMonthPage`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **The UX differs from YNAB.** There's no separate payment category to assign money to. Covering an overspent category or increasing A distribuir is what clears Sem cobertura. This is documented in the page text.

## 17. Acceptance criteria

```gherkin
Feature: Card payment envelopes

  Scenario: Funded card spending
    Given Itaú R$ 1.000,00, Combustível R$ 300,00 assigned
    When R$ 45,00 is spent at Posto Shell on Nubank
    Then Nubank shows Para pagar R$ 45,00, A distribuir stays R$ 700,00, and Sem cobertura is not shown

  Scenario: Overspent on the card
    When R$ 400,00 is spent on Nubank from Combustível (R$ 300,00 assigned)
    Then Sem cobertura is R$ 100,00

  Scenario: Paying the bill
    When R$ 45,00 is transferred from Itaú to Nubank
    Then Para pagar is R$ 0,00 and Combustível is unchanged
```

## 18. Out of scope

- Off-budget accounts (e08s05).
- Per-card attribution of Sem cobertura when there are several cards (it's shown as a total).

## 19. Verification Script (Step-by-Step)

1. Create (or reuse) Itaú with an opening balance of R$ 1.000,00 and a **Nubank** card (Cartões e dívidas, cartão de crédito). Map Posto Shell to Combustível.
2. In Orçamento, set Combustível to 300,00 for this month. A distribuir shows R$ 700,00.
3. In Nubank's Lançamentos, record 45,00 at Posto Shell. In Orçamento:
   - **Cartões de crédito** shows Nubank Para pagar R$ 45,00.
   - A distribuir is still R$ 700,00.
   - Combustível shows Disponível R$ 255,00.
4. Record 355,00 more on Nubank at Posto Shell. Combustível shows −R$ 100,00, and **Sem cobertura: R$ 100,00** appears in red.
5. Set Combustível to 400,00. Sem cobertura disappears.
6. Transfer 400,00 from Itaú to Nubank. Nubank leaves the list, and Combustível stays at R$ 0,00.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S19.
- Decisions: `specs/planning-context.yaml` → `domain.planning` (credit cards, MODIFIED above), P4.
- Code and tests carry `// story: e08s04`.
