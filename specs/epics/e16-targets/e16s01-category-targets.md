# e16s01 — Savings targets (metas) for expense categories

**type:** feat · **risk:** P1 · **context:** budget, planning, ui · **bcps:** 5 · **scope:** "Goals and savings targets" (SCOPE_LATEST, deferred until now; user request 2026-10-10)

## 1. Business narrative

Some expenses come once a year: IPVA, IPTU, insurance. The user wants to reserve money every month so the full amount is there when the bill arrives. Envelopes already carry leftovers forward, but the user does the math by hand. A **meta** on a category says "R$ X by month Y". Each month, Orçamento shows how much to put in to stay on track and how far along the category is.

## 2. Actors

- **User**: signed in.

## 3. Preconditions

- e05 (budget) and e08 (planning: carry-over, Disponível) are done.

## 4. Trigger

The user sets a meta on the category page (`/categorias/{id}`) and opens Orçamento.

## 5. Main flow

1. On the edit page of an **expense** category, a "Meta" section has: Valor (R$, > 0) and Mês (`AAAA-MM`, the due month). **Salvar meta** creates or replaces the category's meta; **Remover meta** deletes it.
2. Orçamento, for each expense category with a meta, in month *m*:
   - shows "Meta R$ X até mmm/aaaa" and the progress: Disponível ÷ X, as a percentage clamped to 0–100;
   - **Guarde este mês** (needed) = ⌈(X − Do mês anterior) ÷ months left⌉, at least zero. *Months left* counts *m* through the due month, both included. *Do mês anterior* is the category's carry-over at the start of *m*.
   - a status:
     - **Atingida** when Disponível ≥ X;
     - otherwise **Vencida** when *m* is after the due month;
     - otherwise **No ritmo** when the month's amount ≥ needed, else **Abaixo do ritmo**.
   - a **Usar sugestão** button when the status is No ritmo or Abaixo do ritmo and the month's amount differs from needed. It posts the needed amount to the existing amount route (`POST /orcamento/{mes}/{categoria}`). Nothing changes until the user clicks.
3. A meta never repeats. After the due month it stays on the page as **Vencida** until the user edits or removes it.

## 6. Alternative flows

- **6a. Income category:** no Meta section; the service rejects it.
- **6b. Due month before the current month when saving:** rejected with "Escolha o mês atual ou um mês futuro."
- **6c. Amount ≤ 0 or malformed, or malformed month:** rejected with a pt-BR message; nothing changes.
- **6d. Merge:** the source category's meta is deleted with its amounts. The target keeps its own meta.
- **6e. Archived category:** its meta is kept but not shown; restoring the category brings it back.

## 7. Business rules

- **B5:** at most one meta per category; only expense categories; amount int64 centavos > 0; due month is a calendar month in America/Sao_Paulo.
- Needed, progress and status are computed, never stored (P4).
- The meta looks only at what remains in Disponível. Money spent from the category no longer counts as saved.
- Suggest only: the meta never changes a month's amount by itself.

## 8. Data

New budget migration `00003_targets.sql`:

```sql
CREATE TABLE budget_target (
    user_id     TEXT NOT NULL,
    category_id TEXT NOT NULL PRIMARY KEY REFERENCES budget_category (id),
    amount      INTEGER NOT NULL CHECK (amount > 0),
    due_month   TEXT NOT NULL CHECK (due_month GLOB '[0-9][0-9][0-9][0-9]-[01][0-9]'),
    created_at  TEXT NOT NULL
);
```

Amounts and months are plaintext, like `budget_amount`. Included in the data export (`targets`) and in account deletion.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*budget.Service).SetTarget(ctx, userID, categoryID string, amount int64, due Month) error` | Create or replace; `ErrInvalidAmount`, `ErrTargetIncome`, `ErrTargetPast`, `ErrNotFound` |
| `(*budget.Service).RemoveTarget(ctx, userID, categoryID string) error` | Idempotent |
| `(*budget.Service).Targets(ctx, userID string) (map[string]Target, error)` | Active categories only |
| `planning.CategoryPlan.Target *TargetPlan` | `{Amount, Needed int64; Due budget.Month; Progress int; Status}`; nil without a meta |
| `POST /categorias/{id}/meta`, `POST /categorias/{id}/meta/remover` | Signed in; redirect back to the category page |
| Orçamento | Meta line, status, Usar sugestão |
| `/ajuda/orcamento` | New "Metas" section |

## 11. Security & privacy

Every query filters by `user_id` (X2). No new sensitive text.

## 12. Dependencies

None new.

## 13. Requirements

#### ADDED: R1: An expense category can have a savings target (amount by a due month)
#### ADDED: R2: Orçamento shows the monthly amount needed, the progress and the status of each target, and suggests the amount

## 14. Design notes

| Unit | Reason for Depth |
|------|------------------|
| `budget/target.go` | The meta is user intent, so budget owns it (B5). |
| `planning/target.go` | Needed, progress and status depend on Disponível, which only planning knows (P4). |
| `web/` | Category page form, Orçamento display, help text. |

## 15. Implementation steps

1. budget: migration, SetTarget/RemoveTarget/Targets, merge, delete, export, isolation. → verify: `go test -race ./budget/... -run 'TestTarget'`
2. planning: TargetPlan with needed, progress and status. → verify: `go test -race ./planning/... -run 'TestTarget'`
3. web: category page Meta form; Orçamento meta line, status and Usar sugestão; help text. → verify: `go test -race ./web/... -run 'TestTarget'`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Rounding:** needed is rounded up to the centavo, so the user never ends short.
- **Page length:** one extra line per category with a meta only.

## 17. Acceptance criteria

```gherkin
Feature: Savings targets

  Scenario: On track
    Given category "IPVA" has a meta of R$ 2.400,00 due 2027-03
    And Do mês anterior in 2026-10 is R$ 0,00
    When the user opens Orçamento for 2026-10
    Then it shows "Guarde R$ 400,00 este mês" (2.400 ÷ 6 months)
    And "Usar sugestão" sets the month's amount to R$ 400,00

  Scenario: Behind after a skipped month
    Given 2026-10 had R$ 0,00 for IPVA
    When the user opens Orçamento for 2026-11
    Then it shows "Guarde R$ 480,00 este mês" and "Abaixo do ritmo"

  Scenario: Overdue
    Given Disponível is below the meta
    When the user opens Orçamento for 2027-04
    Then the meta shows "Vencida" and no suggestion

  Scenario: Reached
    Given Disponível is at least the meta
    Then the meta shows "Atingida" and 100%
```

## 18. Out of scope

- Repeating metas (rejected by the user, 2026-10-10).
- Monthly "keep R$ X" metas without a date; a separate Metas list; metas on income categories.
- Filling amounts automatically.

## 19. Verification Script (Step-by-Step)

1. Run `make run`. Open Categorias, edit an expense category (e.g. "IPVA") and save a meta: R$ 1.200,00 due 6 months from now.
2. Open Orçamento. Check the meta line: "Guarde R$ 200,00 este mês", 0%, "Abaixo do ritmo", and a **Usar sugestão** button.
3. Click **Usar sugestão**. The month's amount becomes R$ 200,00; the status changes to "No ritmo".
4. Go to next month without changing anything: the amount is copied and the status stays "No ritmo".
5. Move to a month after the due month: the meta shows "Vencida" (if not reached).
6. Edit the category and **Remover meta**: the meta line disappears.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Code and tests carry `// story: e16s01`.
