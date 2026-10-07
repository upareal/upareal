# e06s04 — Edit and delete with history

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S10

## 1. Business narrative

People make mistakes: a wrong amount, the wrong day, the wrong account. A user fixes a transaction in place or deletes it. Nothing is lost silently: every previous version is kept, encrypted, and shown on the edit page.

## 2. Actors

- **User**: from a register line.

## 3. Preconditions

- e06s01 to e06s03 are done.

## 4. Trigger

The user clicks a register line, opening `/lancamentos/{id}?conta={accountID}`.

## 5. Main flow

1. The edit page loads the transaction and picks a form by its **shape**, seen from the account the user came from:
   - **entry:** the other legs are all income or expense. The page shows the split-style form (total plus lines), and one line is allowed.
   - **transfer:** exactly one other leg, on an asset or liability account. The page shows the destination and amount.
2. **Save:** the ledger stores a **history** row with the encrypted snapshot of the previous version (time, description, legs). It then replaces the time, description and legs through the same checks as `postTransaction` (L1–L3, L6), all in one database transaction.
3. **Delete** (soft): it stores a history row, sets `deleted_at`, and the transaction disappears from balances, registers and every sum.
4. The edit page lists the history, newest first: when it changed, the action, and the previous amount and description.

## 6. Alternative flows

- **6a. A transaction that touches a system account** (Saldo inicial, A categorizar): it can't be edited. The page offers **Delete** only, plus "Para corrigir o saldo, use Ajustar saldo." Imported lines ("A categorizar") are handled in e07s02.
- **6b. An invalid edit** (unbalanced, zero, wrong types, archived account): 400. Nothing changes, and no history row is written.
- **6c. Another user's transaction, or one already deleted:** 404.

## 7. Business rules

- L1–L3 and L6 hold for the new version.
- **L4:** balances exclude deleted transactions.
- An account with any leg, **including legs of deleted transactions**, still counts as used. It can't be hard-deleted and must be archived instead.
- History is append-only and sealed with the ledger key, field `"history"`.

## 8. Data

Migration `ledger/migrations/00003_history.sql`:

| Change | Columns |
|--------|---------|
| `ledger_transaction` + | `deleted_at TEXT` |
| `ledger_transaction_history` | `id TEXT PK`, `transaction_id TEXT NOT NULL REFERENCES ledger_transaction(id)`, `user_id TEXT NOT NULL`, `action TEXT NOT NULL CHECK (action IN ('edit','delete'))`, `changed_at TEXT NOT NULL`, `snapshot BLOB NOT NULL` (sealed JSON) |

Every read that sums or lists legs (`AccountBalance`, `AccountBalances`, `BalanceBefore`, `RegisterEntries`, `Counterparts`) adds `deleted_at IS NULL`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.Transaction{ID; At; Description; Legs []TxLeg{AccountID, AccountType, SystemRole string; Amount int64}}` | |
| `(*Service).Transaction(ctx, userID, id) (Transaction, error)` | Active only. `ErrTransactionNotFound` otherwise. |
| `(*Service).EditEntry(ctx, userID, id string, e SplitEntry) error` | One or more lines |
| `(*Service).EditTransfer(ctx, userID, id string, e TransferEntry) error` | |
| `(*Service).DeleteTransaction(ctx, userID, id) error` | |
| `(*Service).History(ctx, userID, id) ([]HistoryEntry, error)` | `HistoryEntry{At; Action; Previous Transaction}`, newest first |
| `ledger.ErrNotEditable`, `ledger.ErrTransactionNotFound` | |
| `GET /lancamentos/{id}?conta=`, `POST /lancamentos/{id}`, `POST /lancamentos/{id}/excluir` | 303 back to the register of `conta` |

## 10. Non-functional

An edit is one database transaction: history, then delete the old legs, then insert the new ones.

## 11. Security & privacy

- Snapshots are sealed. Amounts inside the snapshot are encrypted as part of the JSON, which is acceptable because the current legs stay plaintext.
- Every lookup is scoped by `user_id`, with isolation tests.

## 12. Dependencies (slopcheck)

None new. `encoding/json` from the standard library.

## 13. Requirements

#### ADDED: R1: Edit a transaction, keeping an encrypted history
#### ADDED: R2: Soft-delete a transaction; it leaves balances and registers
#### MODIFIED: Balances and registers
**Before:** summed every leg.
**After:** ignore legs of deleted transactions.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `replaceTransaction` | One internal path for edits: history snapshot, leg replacement and invariant checks. `EditEntry` and `EditTransfer` only build legs. |
| `entryLegs(minLines)` | Shared by `RecordSplit` (at least 2 lines) and `EditEntry` (at least 1), so the direction rule exists once. |

## 15. Implementation steps

1. Migration, plus `Transaction` and `History` reads. Deleted transactions are excluded from every sum. → verify: `go test -race ./ledger/... -run TestTransactionRead`
2. `EditEntry` and `EditTransfer` with history. System transactions aren't editable, and there's an isolation test. → verify: `go test -race ./ledger/... -run TestEditKeepsHistory`
3. `DeleteTransaction` (soft, with history). Balances and registers exclude it, and the account still counts as used. → verify: `go test -race ./ledger/... -run TestSoftDelete`
4. Web: register lines link to the edit page, which has the entry or transfer form, a delete button and the history. → verify: `go test -race ./web/... -run TestEditTransactionPage`
5. Proof: history snapshots aren't stored in plaintext. → verify: `go test -race ./ledger/... -run TestLedgerHasNoPlaintext`
6. Full gate. → verify: `make preflight`

## 16. Risks

- **A sum that forgets `deleted_at`.** Detect: the soft-delete test checks Balance, Balances, Register and the balance before a month.

## 17. Acceptance criteria

```gherkin
Feature: Edit and delete

  Scenario: Fix an amount
    Given an expense of R$ 150,00 to Posto Shell
    When the user changes it to R$ 120,00
    Then the balance reflects R$ 120,00
    And the history shows the previous R$ 150,00

  Scenario: Delete
    When the user deletes the expense
    Then it no longer appears in the register or the balance
    And the history records the deletion

  Scenario: Opening balance
    When the user opens the opening-balance line
    Then only Delete is offered, with a hint to use Ajustar saldo
```

## 18. Out of scope

- Restoring a deleted transaction.
- Editing imported lines (e07s02).

## 19. Verification Script (Step-by-Step)

1. Record an expense of `150,00` from Itaú to Posto Shell. Click it in the register.
2. Change the amount to `120,00` and save. The balance updates. Open it again: the history shows R$ 150,00.
3. Open a transfer and change its destination or amount. Both registers update.
4. Delete the expense. It disappears from the register, and the balance goes back.
5. Open the opening-balance line. Only **Excluir** is offered, with the hint.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S10.
- Decisions: `specs/planning-context.yaml` → `domain.ledger` (plain editing with encrypted transaction_history; soft delete), `domain.invariants.ledger`.
- Code and tests carry `// story: e06s04`.
