# e14s02 — Category suggestions in Revisão

**type:** feat · **risk:** P1 · **context:** domain · **bcps:** 5 · **scope:** — (user request 2026-10-08)

## 1. Business narrative

Most imported lines repeat: "POSTO SHELL", "IFOOD", "NETFLIX". In **Revisão**, upareal now *suggests* the account the user chose before for similar lines, and explains why ("3 lançamentos parecidos"). **The user always decides.** Nothing is categorized until they click.

## 2. Actors

- **User**: on `/revisao`.

## 3. Preconditions

- e07s02 (quick assign) and e12s01 (in-place saves) are done.

## 4. Trigger

The user opens Revisão.

## 5. Main flow

1. `ledger.SuggestCounters(ctx, userID, lines)` returns at most one `Suggestion{AccountID, AccountName, Matches int}` per queued line. It's computed in Go from the user's own decrypted history, and nothing is stored.
   - **History:** the last 2.000 live transactions that have exactly one money leg (asset or liability) and one counter leg (an active, non-system income or expense account). Splits and queued lines don't count.
   - **Normalization:**
     - the text is lowercased, accents are removed, and digits and punctuation become spaces;
     - tokens shorter than 3 letters are dropped;
     - so are generic bank words: `pagto, pagamento, eletron, cobranca, compra, cartao, debito, credito, pix, ted, doc, transferencia, enviado, recebido, rem, des, ltda, com, br, www`.
   - **Similarity:** Jaccard over the token sets. A history line counts as *parecida* when the similarity is ≥ 0.5.
   - **Direction:** money out suggests only expense accounts, and money in only income accounts. The history direction must match too.
   - **Choice:** the account with the most similar lines, with ties broken by the best similarity. If nothing is similar, there's no suggestion.
2. Revisão, per line:
   - with a suggestion, a **"Usar ⟨conta⟩"** button posts that account to the existing quick-assign route, next to a muted reason "N lançamentos parecidos";
   - the manual select now starts at **"— escolha —"** (empty and required), so nothing is pre-chosen by accident;
   - saving works exactly as before, in place, through htmx.

## 6. Alternative flows

- **6a. No history, or nothing similar:** no button. Only the manual select is shown.
- **6b. The suggested account was archived after it was used:** it isn't suggested, because only active accounts count.

## 7. Business rules

- **Suggestions never write.** Only the user's click categorizes, through the existing `Categorize` path with all its checks.
- Each user's suggestions come only from their own history (X2).

## 8. Data

No schema change. There's one read query: recent live transactions with their legs and account kinds.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*ledger.Service).SuggestCounters(ctx, userID string, lines []QueuedLine) (map[string]Suggestion, error)` | Keyed by transaction ID |
| Revisão | A "Usar X" button, plus "— escolha —" as the select default |

## 10. Non-functional

It decrypts at most 2.000 descriptions per Revisão load, which takes milliseconds.

## 11. Security & privacy

- It runs inside the server on data the user already sees. No AI service, no stored model, no logs of descriptions.

## 12. Dependencies (slopcheck)

None new (`golang.org/x/text/unicode/norm` is already in the module, so accent removal uses `transform` and `runes`).

## 13. Requirements

#### ADDED: R1: Suggest the counter account for queued lines from similar past lines, with a reason
#### MODIFIED: The quick-assign select starts empty ("— escolha —") instead of on the first account

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.SuggestCounters` | The ledger owns descriptions and their key. The web layer only renders the suggestions. |

## 15. Implementation steps

1. `SuggestCounters`: normalization, similarity, direction, active accounts, deleted and queued lines ignored, isolation. → verify: `go test -race ./ledger/... -run TestSuggest`
2. Revisão: the Usar button with its reason, and the empty default select. → verify: `go test -race ./web/... -run TestReviewSuggest`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **Bad suggestions on noisy descriptions.** The user decides, the reason is shown, and the threshold is conservative.

## 17. Acceptance criteria

```gherkin
Feature: Category suggestions

  Scenario: Similar past line
    Given 3 past "POSTO SHELL" purchases assigned to Posto Shell
    When "PAGTO ELETRON POSTO SHELL 4567" is in Revisão
    Then it shows "Usar Posto Shell" and "3 lançamentos parecidos"
    And nothing is assigned until the user clicks

  Scenario: Nothing similar
    Then there is no suggestion and the select starts at "— escolha —"
```

## 18. Out of scope

- LLM fallback for never-seen descriptions (needs the in-app LLM decision).
- Using the "conta" text, and suggesting splits.

## 19. Verification Script (Step-by-Step)

1. Import a statement whose lines resemble ones you categorized before (for example, re-import an older Bradesco month into a new account, or use similar descriptions).
2. In Revisão, check that lines like your past ones show **"Usar ⟨conta⟩"** with "N lançamentos parecidos", and that others show none.
3. Click a suggestion. The line leaves the queue, and it's saved to that account.
4. Check that the manual select starts at "— escolha —".
5. Run `make preflight`. It ends green.

## 20. Traceability

- Code and tests carry `// story: e14s02`.
