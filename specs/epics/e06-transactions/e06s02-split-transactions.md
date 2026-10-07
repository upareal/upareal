# e06s02 — Split transactions

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S10

## 1. Business narrative

One supermarket receipt of R$ 300,00 is really R$ 200,00 "Mercado" and R$ 100,00 "Casa". The user records it once, from the account that paid, and spreads the total across several expense or income accounts. Planning can then count each part in the right category.

## 2. Actors

- **User**: on the account register page.

## 3. Preconditions

- e06s01 is done: register page and simple entries exist.

## 4. Trigger

The user opens **Lançamento dividido** on an asset or liability register and submits it.

## 5. Main flow

1. **Lançamento dividido** form fields:
   - **total** (more than zero: the money that left this account), date, time, description;
   - at least two **lines**, each with a counter account (the user's expense and income accounts) and an amount greater than zero.
2. **Adicionar linha** (HTMX `hx-get`) appends one more line.
3. While the user types, **"Falta distribuir"** (HTMX `hx-post`, triggered by input) shows `total − Σ(expense lines) + Σ(income lines)`. Income lines are cashback or discounts and reduce the total.
4. **Save** posts one transaction through `postTransaction`:
   - each **expense** line A: counter `+A`;
   - each **income** line A: counter `−A`;
   - the paying account: `−total`.
5. The register shows **one** line for the transaction, with all counterparts and the total amount.

## 6. Alternative flows

- **6a. Lines don't add up to the total:** 400 with "Falta distribuir R$ X" (or "Distribuído a mais R$ X"). Nothing is saved (**L1**).
- **6b. Fewer than two filled lines:** 400 with "Informe pelo menos duas linhas." A single line is a simple entry.
- **6c. Line or total ≤ 0, or malformed:** 400 with "Valor inválido".
- **6d. Line account not income or expense, archived, or belonging to another user:** rejected (L3, L6).
- **6e. Empty lines** (no account and no amount) are ignored, so the form can show spare rows.

## 7. Business rules

- L1–L3 and L6 hold through `postTransaction`.
- The same counter account may appear on two lines. They become two legs.
- Direction per line comes from the counter's type, exactly as in `RecordSimple`.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.SplitEntry{AccountID string; Total int64; At time.Time; Description string; Parts []Part}` | |
| `ledger.Part{CounterID string; Amount int64}` | |
| `(*Service).RecordSplit(ctx, userID, SplitEntry) (string, error)` | |
| `ledger.ErrSplitMismatch` | Unwrap gives the remaining amount through `SplitMismatch{Remaining int64}` |
| `GET /contas/{id}/dividido`, `POST /contas/{id}/dividido` | Form page and submit |
| `GET /contas/{id}/dividido/linha` | HTMX fragment: one empty line |
| `POST /contas/{id}/dividido/restante` | HTMX fragment: "Falta distribuir R$ X" |

## 10. Non-functional

The remaining calculation reads the user's accounts once to learn each line's direction. It writes nothing.

## 11. Security & privacy

- The remaining endpoint is a POST under `CrossOriginProtection`. It only reveals the user's own account types.
- The description is sealed, as in e06s01.

## 12. Dependencies (slopcheck)

None new. HTMX 2.0.11 is already vendored. Attributes used: `hx-get`, `hx-post`, `hx-target`, `hx-swap="beforeend"`, `hx-trigger="input changed delay:300ms from:closest form"`, `hx-include="closest form"`.

## 13. Requirements

#### ADDED: R1: Split one payment across several expense or income accounts
#### ADDED: R2: Live "Falta distribuir" with HTMX; save only when it is zero

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `RecordSplit` | Owns the direction rule and the total check for many lines. The web layer never builds legs. |
| `ledger.SplitMismatch` | Carries the exact remaining amount, so the page can say how much is missing without recomputing it. |

## 15. Implementation steps

1. `RecordSplit` with its checks (total, lines, directions, mismatch). → verify: `go test -race ./ledger/... -run TestRecordSplit`
2. Web: split form, HTMX add-line and remaining fragments, submit with errors. → verify: `go test -race ./web/... -run 'TestSplitForm|TestSplitBalances'`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **Spare empty rows mistaken for errors.** Mitigated by 6e: completely empty rows are skipped.

## 17. Acceptance criteria

```gherkin
Feature: Split transactions

  Scenario: Supermarket receipt
    When the user splits total "300,00" into Mercado "200,00" and Casa "100,00"
    Then Itaú goes down by R$ 300,00, Mercado up R$ 200,00, Casa up R$ 100,00
    And the register shows one line with both counterparts

  Scenario: Cashback line
    When total "95,00" is split into Mercado "100,00" and Cashback (income) "5,00"
    Then it is saved

  Scenario: Not fully distributed
    When total "300,00" has lines "200,00" and "50,00"
    Then the page says "Falta distribuir R$ 50,00" and nothing is saved
```

## 18. Out of scope

- Transfers between own accounts (e06s03).
- Editing a split (e06s04).
- Splitting imported lines (e07s02).

## 19. Verification Script (Step-by-Step)

1. Create the expense accounts "Mercado" and "Casa". Open Itaú → Lançamentos → **Lançamento dividido**.
2. Enter total `300,00`, then Mercado `200,00`. The page shows "Falta distribuir R$ 100,00" while you type.
3. Add Casa `100,00`. The message shows "Falta distribuir R$ 0,00". Save.
4. The register shows one line, "Mercado, Casa", for -R$ 300,00.
5. Try lines that don't add up and save. You see the message, and nothing is saved.
6. Click **Adicionar linha**. A new empty line appears without a page reload.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S10.
- Decisions: `specs/planning-context.yaml` → `domain.ledger` (full split transactions in the form), `domain.invariants.ledger`.
- Code and tests carry `// story: e06s02`.
