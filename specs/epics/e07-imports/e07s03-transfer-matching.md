# e07s03 — Match transfers across two imports

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S16

## 1. Business narrative

When money moves between two of the user's own accounts and both statements are imported, it shows up twice: "TED para poupança −1.000,00" at Itaú and "TED recebida +1.000,00" at the savings account. The user marks one as a transfer, the app finds its twin, and after confirming there is **one** transfer and no duplicate. Re-importing either file adds nothing.

## 2. Actors

- **User**: on the Categorizar page of a queued line.

## 3. Preconditions

- e07s02 is done: queued lines have a Categorizar page.

## 4. Trigger

On a queued line, the user chooses **É uma transferência** and picks the other account.

## 5. Main flow

1. The user picks the other asset or liability account.
2. `ledger.TransferCandidates` lists **queued** lines on that account with the **opposite amount**, dated within **±3 days**, closest date first.
3. **No candidate:** the line becomes a transfer right away (`MarkTransfer` with no match). Its legs are this account `a` and the other account `−a`. The previous version goes to history.
4. **One or more candidates:** a confirmation page lists them (date, description, amount), plus **"Não juntar"**. On confirm, `MarkTransfer`:
   - turns this line into the transfer;
   - soft-deletes the chosen twin, recording "delete" in its history;
   - keeps **both** import records, so re-importing either statement still skips them.
5. Both lines leave the queue. One transfer appears in both registers.

## 6. Alternative flows

- **6a. The other account is the same one, isn't an asset or liability, or belongs to another user:** 400 or 404.
- **6b. The chosen match is no longer a valid candidate** (already used, wrong amount, outside the date window): 400 "Correspondente inválido". Nothing changes.
- **6c. The line is no longer queued:** 400 (`ErrNotQueued`).

## 7. Business rules

- L1–L3 and L6 hold through the edit path.
- Matching never happens automatically. The user always confirms (auto-matching is deferred to auto-categorization).
- The import duplicate key stays attached to both import records, even though the twin's transaction is deleted.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.TransferCandidates(ctx, userID, txID, otherAccountID) ([]QueuedLine, error)` | Queued lines on the other account with the opposite amount, within ±3 days, closest first |
| `ledger.MarkTransfer(ctx, userID, txID, otherAccountID, matchTxID string) error` | `matchTxID` empty means no twin |
| `ledger.ErrInvalidMatch` | |
| `POST /lancamentos/{id}/transferencia?conta=` (`outra`) | No candidate: converts and goes to `/revisao`. Otherwise: shows the confirmation page. |
| `POST /lancamentos/{id}/transferencia/confirmar?conta=` (`outra`, `par`) | `par` empty means no twin |

## 10. Non-functional

The candidate search is one query over one account's transactions in a 6-day window.

## 11. Security & privacy

- Everything is loaded with `user_id`. A foreign match ID gives `ErrInvalidMatch` or not found.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Turn a queued line into a transfer to another own account
#### ADDED: R2: Find and merge its twin from the other statement after the user confirms

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `MarkTransfer` | One transaction: convert, delete the twin, write history for both. A crash can't leave half a merge. |

## 15. Implementation steps

1. `TransferCandidates`: queued only, opposite amount, ±3 days, other account, isolation. → verify: `go test -race ./ledger/... -run TestTransferCandidates`
2. `MarkTransfer`, with and without a twin; the twin is soft-deleted; dedupe kept; invalid matches rejected. → verify: `go test -race ./ledger/... -run TestTransferMerge`
3. Web: transfer section on Categorizar, confirmation page, and both paths. → verify: `go test -race ./web/... -run TestReviewTransfer`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Two unrelated R$ 100,00 lines inside the window get proposed.** Mitigation: the user always confirms, and "Não juntar" is always offered.

## 17. Acceptance criteria

```gherkin
Feature: Transfer matching

  Scenario: Twin found and merged
    Given Itaú has queued "TED para poupança" -1.000,00 on 10/02
    And Poupança has queued "TED recebida" +1.000,00 on 11/02
    When the user marks the Itaú line as a transfer to Poupança and confirms the match
    Then there is one transfer: Itaú -1.000,00, Poupança +1.000,00
    And both lines leave the queue
    And re-importing either statement adds nothing

  Scenario: No twin
    When the user marks a line as a transfer to an account with no matching line
    Then the line becomes a transfer immediately
```

## 18. Out of scope

- Automatic matching without confirmation.
- Matching posted (non-queued) transactions.

## 19. Verification Script (Step-by-Step)

1. Create "Poupança" (asset). Import one CSV into Itaú with `t1;10/02/2026;;TED para poupança;-1.000,00`, and another into Poupança with `p1;11/02/2026;;TED recebida;1.000,00`.
2. In **Revisão**, open the Itaú line. Under **É uma transferência**, choose Poupança.
3. The confirmation page lists "TED recebida". Confirm it.
4. Both lines leave Revisão. Both registers show one transfer.
5. Re-import both files. You see "0 importadas, 1 já existentes" each time.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S16.
- Decisions: `specs/planning-context.yaml` → `domain.import` (transfer matching in the review queue, ±3 days, user confirms, both import ids recorded).
- Code and tests carry `// story: e07s03`.
