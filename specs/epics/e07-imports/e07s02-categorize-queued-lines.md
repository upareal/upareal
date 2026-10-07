# e07s02 — Assign accounts to queued lines (and split them)

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S15

## 1. Business narrative

After importing, the user goes through **Revisão** and says what each bank line was: "SUPERMERCADO X" was Mercado, "PIX MARIA" was a gift, and one receipt was half Mercado and half Casa. Each assigned line leaves the queue. The amount the bank reported never changes.

## 2. Actors

- **User**: on `/revisao` or on the edit page of a queued line.

## 3. Preconditions

- e07s01 is done: queued lines exist.
- UAT feedback on e07s01: clicking a queued line showed the "system account" message.

## 4. Trigger

Either the user picks an account on `/revisao` and saves, or the user opens a queued line and saves the split form.

## 5. Main flow

1. **Quick assign** on `/revisao`: each line has one picker (the user's expense and income accounts) and **Salvar**. The whole bank amount goes to that account.
2. **Detailed** on the edit page: a queued line opens a **"Categorizar"** form showing:
   - **Valor do extrato** (read-only),
   - one or more lines (account and amount). A mismatch is reported on save as **"Falta distribuir R$ X"**. The live HTMX version is left out here, because an imported income has a negative total from the account's point of view (refined during implementation).
   - date, time and description (read-only here: they can be edited afterwards with the normal edit form; refined during implementation).
3. `ledger.Categorize` replaces the "A categorizar" leg with the chosen accounts, under these rules:
   - the money account's leg keeps the **imported amount** (lines must add up to it, or `SplitMismatch`);
   - the line direction comes from the account type, as in splits;
   - the previous version goes to history;
   - L1–L3 and L6 hold.
4. The line leaves the queue, because the queue is derived. The transaction is now a normal entry, editable with the e06s04 form.

## 6. Alternative flows

- **6a. Lines don't add up to the bank amount:** 400 "Falta distribuir R$ X". Nothing changes.
- **6b. Choosing "A categorizar" or an asset/liability account:** they aren't offered. Transfers between own accounts are matched in e07s03.
- **6c. Categorize on a transaction that isn't queued** (manual, or already assigned): `ErrNotQueued`, 400.
- **6d. Opening balances and adjustments** still can't be edited (e06s04). Their hint stays, and the queued case gets its own form.
- **6e. Another user's transaction:** 404.

## 7. Business rules

- The money-account leg (asset or liability) of an imported line is fixed during review.
- History records the "A categorizar" version.
- The import record stays (dedupe), and only the legs change.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*ledger.Service).Categorize(ctx, userID, txID string, parts []Part) error` | Lines are at least 1, each > 0. Direction comes from each account's type, and the lines must add up to the imported amount. |
| `ledger.ErrNotQueued` | |
| `Transaction` legs | Already carry `SystemRole`. A queued shape has exactly one asset/liability leg and the rest on `uncategorized_*`. |
| `POST /revisao/{id}` (`contra`) | Quick assign. 303 back to `/revisao`. |
| Edit page | Shape `queued`: Categorizar form, `POST /lancamentos/{id}?conta=` with `fixo=1`. |

## 10. Non-functional

One database transaction per assignment.

## 11. Security & privacy

- The transaction is loaded first, so foreign IDs reveal nothing.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Quick assign of a queued line to one expense/income account
#### ADDED: R2: Split a queued line across several accounts, keeping the bank amount
#### MODIFIED: Edit page for imported lines
**Before:** queued lines showed "system account; delete or adjust" (e07s01 UAT feedback).
**After:** queued lines show the Categorizar form.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.Categorize` | Owns "the bank amount is fixed". The web can't change the money leg by accident, which a generic `EditEntry` would allow. |

## 15. Implementation steps

1. `Categorize`: queued only, amount fixed, one or more lines, history, isolation. → verify: `go test -race ./ledger/... -run TestCategorize`
2. Web: the `queued` edit-page shape with the Categorizar form, and the fix for the misleading hint. → verify: `go test -race ./web/... -run TestReviewCategorize`
3. Web: quick assign on `/revisao`. → verify: `go test -race ./web/... -run TestReviewQuickAssign`
4. Full gate. → verify: `make preflight`

## 16. Risks

- None new.

## 17. Acceptance criteria

```gherkin
Feature: Review queue

  Scenario: Quick assign
    Given "SUPERMERCADO X" -R$ 150,00 is in the queue
    When the user picks "Mercado" and saves on /revisao
    Then it leaves the queue and Mercado has R$ 150,00

  Scenario: Split a queued line
    When the user splits it into Mercado 100,00 and Casa 50,00
    Then it leaves the queue
    And Itaú still shows -R$ 150,00 for it

  Scenario: Bank amount is fixed
    When the lines add up to 140,00
    Then "Falta distribuir R$ 10,00" and nothing changes
```

## 18. Out of scope

- Matching transfers between two imports (e07s03).
- Auto-categorization from the `conta` text (later).

## 19. Verification Script (Step-by-Step)

1. Import the example CSV, then open **Revisão**.
2. For "SUPERMERCADO X", pick **Mercado** and click Salvar. It leaves the queue.
3. Click "SALARIO ACME". The **Categorizar** form shows the bank amount R$ 8.000,00. Split it into Salário `7.000,00` and Renda extra `1.000,00`, then save. It leaves the queue.
4. Try a split that doesn't add up. You see "Falta distribuir", and the line stays queued.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S15.
- Decisions: `specs/planning-context.yaml` → `domain.import` (review queue, second leg on A categorizar), `domain.invariants.ledger`.
- Code and tests carry `// story: e07s02`.
