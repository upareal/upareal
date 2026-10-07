# e04s03 — Organize, archive and delete accounts

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S07, S08

## 1. Business narrative

Accounts change over time. A user renames "Conta" to "Itaú", moves their main card to the top, closes an old savings account without losing its history, and deletes an account they created by mistake. The ledger keeps every real accounted for while they do it.

## 2. Actors

- **User**: manages their accounts on `/contas`.

## 3. Preconditions

- e04s02 is done: accounts, legs and balances exist.

## 4. Trigger

The user edits, moves, archives, restores or deletes an account.

## 5. Main flow

1. **Edit** (`/contas/{id}`): change the name; for asset and liability accounts, the on-budget and card flags; and the **type**, but only if the account has no legs.
2. **Move** up or down within its type: this swaps the position with the neighbour.
3. **Archive** an account whose balance is zero. It leaves the active list and appears under "Arquivadas", where **Restore** brings it back.
4. **Delete** an account that has never had a leg.

## 6. Alternative flows

- **6a. Type change on an account with legs** → rejected: "Contas com lançamentos não podem mudar de tipo." (**L5**)
- **6b. Archive with a non-zero balance** → rejected: "Zere o saldo antes de arquivar."
- **6c. Delete with legs** → rejected: "Contas com lançamentos não podem ser excluídas. Arquive-a."
- **6d. Any change to a system account** (rename, type, move, archive, delete) → rejected (**L7**). They aren't listed anyway.
- **6e. Moving the first account up or the last down** → no change.
- **6f. Another user's account ID** → 404 (**X2**).

## 7. Business rules

- **L5** An account's type can't change once it has legs.
- **L6** Archived accounts take no new legs. `postTransaction` already enforces this (e04s02).
- **L7** System accounts can't be deleted or archived. They also can't be edited or moved.
- Card rules from e04s02 still apply after an edit: a card must be an on-budget liability.

## 8. Data

No schema change. The existing `archived_at` and `position` columns are used. New queries: `UpdateAccount`, `SetAccountArchived`, `DeleteAccount`, `CountAccountLegs`, `ListArchivedAccounts`, `NeighbourAccount`, `SetAccountPosition`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.AccountUpdate{Name, Type string; OnBudget, IsCard bool}` | |
| `(*Service).UpdateAccount(ctx, userID, id, AccountUpdate) (Account, error)` | Same validation as `CreateAccount`, plus L5 and L7 |
| `(*Service).MoveAccount(ctx, userID, id string, up bool) error` | |
| `(*Service).ArchiveAccount` / `RestoreAccount(ctx, userID, id) error` | |
| `(*Service).DeleteAccount(ctx, userID, id) error` | |
| `(*Service).ArchivedAccounts(ctx, userID) ([]Account, error)` | |
| Errors | `ErrTypeLocked` (L5), `ErrSystemAccount` (L7), `ErrNonZeroBalance`, `ErrHasLegs`, `ErrAccountNotFound` |
| `GET /contas/{id}`, `POST /contas/{id}` | Edit form |
| `POST /contas/{id}/mover?dir=up\|down`, `/arquivar`, `/restaurar`, `/excluir` | 303 to `/contas` |

## 10. Non-functional

A move is two position updates in one transaction.

## 11. Security & privacy

- Every action loads the account with `id AND user_id`. Isolation is tested.
- The new name is sealed. All forms are POST and protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Edit accounts (L5, L7)
#### ADDED: R2: Reorder accounts within a type
#### ADDED: R3: Archive and restore at zero balance (L6, L7)
#### ADDED: R4: Delete accounts that were never used (L7)

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.loadUserAccount` | One guard returns `ErrAccountNotFound` for other users' IDs and `ErrSystemAccount` for system accounts, for every mutation. |

## 15. Implementation steps

1. `UpdateAccount` with L5, L7 and the card rules. → verify: `go test -race ./ledger/... -run TestUpdateAccount`
2. `MoveAccount` within a type. → verify: `go test -race ./ledger/... -run TestMoveAccount`
3. Archive and restore at zero balance, plus `ArchivedAccounts`. → verify: `go test -race ./ledger/... -run TestArchiveAccount`
4. `DeleteAccount` only without legs. → verify: `go test -race ./ledger/... -run TestDeleteAccount`
5. Web: edit page, move, archive and restore with an "Arquivadas" section, and delete. → verify: `go test -race ./web/... -run TestOrganizeAccounts`
6. Full gate. → verify: `make preflight`

## 16. Risks

- **Position gaps after delete or archive.** That's harmless: ordering only compares positions, and moves swap with the actual neighbour.

## 17. Acceptance criteria

```gherkin
Feature: Organize accounts

  Scenario: Type locked after use
    Given "Conta corrente" has an opening balance
    When the user changes its type to liability
    Then it is rejected and the type is unchanged

  Scenario: Archive only at zero
    Given "Poupança" has R$ 10,00
    When the user archives it
    Then it is rejected
    When the user adjusts it to 0,00 and archives it
    Then it moves to "Arquivadas" and takes no new legs

  Scenario: Delete only unused
    Given "Teste" has no legs
    When the user deletes it
    Then it is gone

  Scenario: System accounts are protected
    When the user tries to archive or delete "Saldo inicial"
    Then it is rejected
```

## 18. Out of scope

- Merging accounts.
- Archiving or merging budget categories (e08s06).
- Bulk actions.

## 19. Verification Script (Step-by-Step)

1. Run `make run` and open **Contas**. Click an account's name. The edit page opens.
2. Rename "Conta corrente" to "Itaú" and save. The new name shows.
3. On "Itaú" (which has a balance), try changing the type. You see "Contas com lançamentos não podem mudar de tipo."
4. Create two expenses and use ↑ and ↓. The order changes.
5. Try to archive an account with a balance. You see "Zere o saldo antes de arquivar." Adjust it to `0,00` and archive it. It moves to "Arquivadas". Restore it.
6. Create the expense "Teste" and delete it. It's gone. Try deleting "Itaú". It's rejected.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S07, S08.
- Decisions: `specs/planning-context.yaml` → `domain.invariants.ledger` (L5–L7), `domain.ledger` (archive at zero balance, delete only if unused).
- Code and tests carry `// story: e04s03`.
