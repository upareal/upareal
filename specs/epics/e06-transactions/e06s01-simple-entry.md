# e06s01 — Record a simple expense or income

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S10

## 1. Business narrative

A user records daily life: "R$ 150,00 at Posto Shell from my Itaú account on 31/01 at 22:30", or "Salário R$ 8.000,00 into Itaú". Each entry becomes a balanced transaction. The account's register lists entries month by month with a running balance, with days and months counted in São Paulo time.

## 2. Actors

- **User**: on an account's register page.

## 3. Preconditions

- e04 and e05 are done.
- The deferred row-level AAD binding stays deferred (`specs/planning-context.yaml`). Descriptions use the same user and field binding as the other sealed fields.

## 4. Trigger

The user opens `/contas/{id}/lancamentos` and submits the new entry form.

## 5. Main flow

1. The register page for an asset or liability account shows one month at a time: the current month in São Paulo by default, or another month with `?mes=YYYY-MM`, with links to the previous and next months.
2. **New entry form:** kind (**Despesa** or **Receita**), the counter account (the user's active expense or income accounts, depending on the kind), amount (more than zero), date and time (default now in São Paulo) and description.
3. `ledger.RecordSimple` posts two legs through `postTransaction`:
   - **expense** of A: counter `+A`, account `−A`
   - **income** of A: counter `−A`, account `+A`
4. The register lists the month's entries in time order, each showing:
   - date and time in São Paulo,
   - description,
   - counter account names,
   - the amount on this account, displayed the way the account's type reads (for a card, "R$ 150,00" spent shows as an increase in what is owed),
   - the running balance.

   The running balance starts from the balance before the month.

## 6. Alternative flows

- **6a. Amount zero, negative or malformed:** 400 with "Valor inválido". Refunds are recorded as splits (e06s02) or edits (e06s04).
- **6b. Account not asset or liability, or counter not income or expense:** 400.
- **6c. An archived or another user's account:** rejected through L3 and L6. Shows 404 or 400.
- **6d. Description over 200 characters:** 400. An empty description is allowed.
- **6e. Malformed date or time:** 400.

## 7. Business rules

- All ledger invariants apply through `postTransaction` (L1–L3, L6).
- Times are stored in UTC. Day and month are computed in America/Sao_Paulo. An entry at 22:30 on 31/01 in São Paulo belongs to **January**.
- Descriptions are sealed (field `"description"`).

## 8. Data

No schema change. This uses `ledger_transaction` and `ledger_leg`, plus the existing index on `(user_id, occurred_at)`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.SimpleEntry{AccountID, CounterID string; Amount int64; At time.Time; Description string}` | Amount > 0 |
| `(*Service).RecordSimple(ctx, userID, SimpleEntry) (string, error)` | Returns the transaction ID |
| `ledger.RegisterLine{TransactionID string; At time.Time; Description string; Counterparts []string; Amount, Balance int64}` | Amount and Balance are debit-positive on this account |
| `(*Service).Register(ctx, userID, accountID string, from, to time.Time) ([]RegisterLine, error)` | `[from, to)`, ordered by time |
| `GET /contas/{id}/lancamentos[?mes=]`, `POST /contas/{id}/lancamentos` | 303 back to the entry's month |

## 10. Non-functional

A register month is two queries: the balance before `from`, and the legs in range with their counterparts.

## 11. Security & privacy

- Descriptions are sealed. Counterpart names are decrypted with the same ledger key.
- Every query is scoped by `user_id`. Isolation is tested.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new. This reuses `budget.Month` in web for month ranges (web may import every module; ledger stays independent).

## 13. Requirements

#### ADDED: R1: Record a simple expense or income (two balanced legs)
#### ADDED: R2: Account register by month with running balance
#### ADDED: R3: São Paulo day and month boundaries for entries

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `RecordSimple` | Owns the sign rule for simple entries, so web never builds legs for the common case. |
| `Register` | One read returning the running balance, so the page doesn't sum legs. |

## 15. Implementation steps

1. `RecordSimple` with its validation. → verify: `go test -race ./ledger/... -run TestRecordSimple`
2. `Register`: range, order, running balance, counterparts and isolation. → verify: `go test -race ./ledger/... -run TestRegister`
3. Web: the register page, the new entry form and the São Paulo month boundary. → verify: `go test -race ./web/... -run 'TestRecordTransaction|TestAccountRegister|TestMonthInSaoPaulo'`
4. Proof that descriptions aren't stored in plaintext. → verify: `go test -race ./ledger/... -run TestLedgerHasNoPlaintext`
5. Full gate. → verify: `make preflight`

## 16. Risks

- **Displayed signs on liability registers.** Mitigation: the register reuses `displayBalance` per row, and a card case is tested.

## 17. Acceptance criteria

```gherkin
Feature: Simple entries

  Scenario: Expense from checking
    Given "Itaú" has R$ 1.000,00
    When the user records an expense of "150,00" to "Posto Shell" on 2026-01-31 22:30
    Then Itaú's January register shows the entry and a balance of R$ 850,00
    And "Posto Shell" has R$ 150,00 spent

  Scenario: Month boundary in São Paulo
    Given an entry at 2026-01-31 22:30 São Paulo
    Then it appears in January, not February

  Scenario: Income
    When the user records income "8.000,00" from "Salário" into "Itaú"
    Then Itaú's balance grows by R$ 8.000,00
```

## 18. Out of scope

- Splits (e06s02), transfers (e06s03), editing and deleting (e06s04), search (e06s05).
- Budget categories on entries: planning maps accounts to categories (e08).

## 19. Verification Script (Step-by-Step)

1. Run `make run`. In **Contas**, open "Itaú" → **Lançamentos**.
2. Record a **Despesa** of `150,00` to "Posto Shell" dated the last day of the month at 22:30. It appears in this month with the running balance.
3. Record a **Receita** of `8.000,00` from "Salário". The balance grows.
4. Open the next month. The 22:30 entry isn't there.
5. Try the amount `0`. You see "Valor inválido".
6. Run `strings /tmp/upareal.db | grep -i "<your description>"`. It prints nothing.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S10.
- Decisions: `specs/planning-context.yaml` → `domain.ledger`, `domain.invariants.ledger`, `data_conventions` (UTC storage, São Paulo month), sign convention.
- Code and tests carry `// story: e06s01`.
