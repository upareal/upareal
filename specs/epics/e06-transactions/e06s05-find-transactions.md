# e06s05 — Find transactions

**type:** feat · **risk:** P1 · **context:** domain · **bcps:** 5 · **scope:** S12

## 1. Business narrative

"How much did I spend at Posto Shell this year?" or "Where is that R$ 89,90 charge?" A user searches their transactions by date range, account, amount and text, and jumps to any result to fix it.

## 2. Actors

- **User**: on `/buscar`.

## 3. Preconditions

- e06s04 is done: the edit page exists.
- Descriptions and names are encrypted, so text matching happens in Go after the SQL filters (`specs/planning-context.yaml`).

## 4. Trigger

The user opens **Buscar** and submits filters.

## 5. Main flow

1. Filters:
   - **de / até**: dates in São Paulo time. They default to the last 12 months and are **always** applied.
   - **conta**: any of the user's accounts, including expense and income accounts. Optional.
   - **valor mínimo / máximo**: matched against the transaction's **amount**, the sum of its positive legs. Optional.
   - **texto**: optional. Matches the description or any account name in the transaction, **ignoring case and accents**.
2. SQL applies the plaintext filters: `user_id`, date range, not deleted, account and amount.
3. Go decrypts the remaining rows and keeps those whose description or account names contain the text.
4. Results are shown newest first, at most **500**. Each shows date and time, description, accounts and amount, and links to the edit page from the first asset or liability account in the transaction.
5. A summary line shows how many results there are and their total.

## 6. Alternative flows

- **6a. Range over 5 years, or `de` after `até`:** 400.
- **6b. More than 500 matches:** the first 500 are shown, with "Mostrando 500 resultados; refine a busca."
- **6c. Malformed amounts or dates:** 400.
- **6d. Another user's account ID in `conta`:** no results (X2).

## 7. Business rules

- Deleted transactions never appear.
- Every search is date-bounded (privacy decision: decrypting has a cost, so it's bounded).

## 8. Data

No schema change. The new query `SearchTransactions` filters by user, range, optional account (`EXISTS` on legs) and amount `HAVING`, with `LIMIT 2000` as an SQL-side safety cap before the text filter.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.SearchQuery{From, To time.Time; AccountID string; Min, Max int64; Text string}` | Min and Max are 0 when unset |
| `ledger.SearchResult{TransactionID; At; Description; Accounts []string; Amount int64; LinkAccountID string}` | |
| `(*Service).Search(ctx, userID, SearchQuery) ([]SearchResult, bool, error)` | The bool reports truncation |
| `GET /buscar?de=&ate=&conta=&min=&max=&texto=` | |

## 10. Non-functional

- At most 2000 rows are decrypted per search. Personal ledgers have a few thousand transactions a year, so this takes milliseconds.

## 11. Security & privacy

- Text is matched in memory, never sent to SQL, and never logged (query strings are already excluded from logs since e01).
- Isolation is tested.

## 12. Dependencies (slopcheck)

| Package | Version | Tag | Evidence |
|---------|---------|-----|----------|
| `golang.org/x/text` (`unicode/norm`, `runes`, `transform`) | v0.42.0 (already indirect, becomes direct) | [OK] | Accent folding: NFD, then remove `unicode.Mn` |

## 13. Requirements

#### ADDED: R1: Search by date range, account and amount, always date-bounded
#### ADDED: R2: Accent- and case-insensitive text search over decrypted descriptions and account names
#### MODIFIED: Search by category
**Before:** the slice planned a category filter.
**After:** filter by ledger account. A budget-category filter follows planning's mapping (e08).

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.fold` | One normalization for search text and haystack, so "agua" finds "Água" both ways. |

## 15. Implementation steps

1. `Search` SQL filters: range, account, min and max, excluding deleted, with isolation. → verify: `go test -race ./ledger/... -run TestSearchFilters`
2. Text match on description and account names, accent- and case-insensitive, with the 500 cap. → verify: `go test -race ./ledger/... -run TestSearchText`
3. The `/buscar` page with defaults, validation, results and summary. → verify: `go test -race ./web/... -run TestSearchPage`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Large ranges cost decrypt time.** Mitigation: the 5-year cap and the 2000-row SQL cap.

## 17. Acceptance criteria

```gherkin
Feature: Find transactions

  Scenario: By text ignoring accents
    Given a transaction described "Conta de Água"
    When the user searches "agua"
    Then it is found

  Scenario: By account and amount
    Given Posto Shell transactions of R$ 150,00 and R$ 50,00
    When the user filters Posto Shell with minimum "100,00"
    Then only the R$ 150,00 one is found

  Scenario: Deleted never appear
    Given a deleted transaction
    When the user searches its description
    Then nothing is found
```

## 18. Out of scope

- Filtering by budget category (after e08).
- Saved searches and export.

## 19. Verification Script (Step-by-Step)

1. Record a few entries, including one described "Conta de Água".
2. Open **Buscar** and type `agua`. It's found.
3. Filter by account "Posto Shell" with a minimum of `100,00`. Only the bigger entries show.
4. Click a result. The edit page opens.
5. Delete a transaction and search for it. It's gone.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S12.
- Decisions: `specs/planning-context.yaml` → `privacy_security` (text search in Go, always date-bounded).
- Code and tests carry `// story: e06s05`.
