# e14s01 — Markdown export for LLM analysis

**type:** feat · **risk:** P1 · **context:** ui · **bcps:** 3 · **scope:** — (new, user request 2026-10-08)

## 1. Business narrative

The user wants an AI's opinion on their finances: "where is my money going?", "which envelopes do I always overspend?", "how much can I save per month?". In **Meus dados** they download one Markdown file for a chosen period and paste or attach it into the LLM of their choice. upareal never sends data anywhere: the user decides what to share, and with whom.

## 2. Actors

- **User**: signed in, on `/conta/dados`.

## 3. Preconditions

- e08 (planning) and e10s01 (export) are done.

## 4. Trigger

The user picks a period and clicks **Baixar para IA (Markdown)**.

## 5. Main flow

1. The form takes `meses`: 1, 3, 6 or 12. The default is 3, counted back from the current São Paulo month and including it. A checkbox, **Incluir lançamentos**, is on by default.
2. `GET /conta/dados/ia?meses=3&lancamentos=on` answers `text/markdown; charset=utf-8`, as an attachment named `upareal-analise-AAAA-MM.md`, with `no-store`.
3. The document is in pt-BR and uses `R$ 1.234,56` amounts. Its sections:
   1. **Header:** period, export date, and an "Como ler estes dados" block. It explains the envelope model (planejado, gasto, disponível, carry-over, A distribuir, Sem categoria, card Para pagar, off-budget accounts) so the LLM interprets the numbers correctly.
   2. **Sugestões de perguntas:** 4–5 example questions the user can ask.
   3. **Contas:** a table with name, type (in pt-BR), whether the account is in the budget, and the balance at the end of the period. Archived accounts with a zero balance and system accounts are left out.
   4. **Orçamento por mês:** for each month, A distribuir, money in, and a table with grupo, categoria, planejado, do mês anterior, gasto and disponível (expense categories), followed by Recebido for income categories and the Sem categoria totals. Cards with Para pagar and Sem cobertura are listed too.
   5. **Lançamentos** (optional): a table with date, description, and how the money moved (`Itaú −R$ 120,00 → Posto Shell`, with every leg shown for splits). It's sorted by date and leaves out deleted transactions. It's capped at 2.000 rows, with a note when the list was cut.
4. **Sources:** `planning.Month` for each month, plus `ledger.ExportUserData` (accounts and transaction legs, already decrypted) filtered to the period. No new queries, and nothing is stored.

## 6. Alternative flows

- **6a. `meses` is missing or invalid:** 3 is used.
- **6b. No data:** the sections render with "Nenhum lançamento no período."

## 7. Business rules

- Only the signed-in user's data goes into the file (X2).
- No IDs, emails or Google identifiers appear in the file: an LLM gets names and amounts only.
- Logs record `user_id`, `meses` and whether transactions were included. Content is never logged.

## 8. Data

None.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `GET /conta/dados/ia` (`meses`, `lancamentos`) | Signed in; answers with a Markdown attachment |
| Meus dados | A new "Análise com IA" section with the form and a privacy note |

## 10. Non-functional

Twelve months is 12 `planning.Month` calls plus one export. That's fine for a personal app.

## 11. Security & privacy

- The page says plainly that the file contains the user's financial data, and that sending it to an AI service shares it with that service under that service's terms.
- Transaction descriptions are user content. They're escaped for Markdown table cells (`|` and line breaks), so a description can't break the table structure.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Download a Markdown summary of a period, made for LLM analysis

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `web/aiexport.go` | Markdown is presentation, so it lives in web and only reads module APIs (X1). |

## 15. Implementation steps

1. Markdown builder and route: sections, period, optional transactions, cap, escaping, isolation from other users. → verify: `go test -race ./web/... -run TestAIExport`
2. Meus dados form, linked. → verify: `go test -race ./web/... -run TestAIExport`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **Large files for 12 months with many transactions.** The cap and the opt-out checkbox handle it.

## 17. Acceptance criteria

```gherkin
Feature: Markdown export for AI

  Scenario: Three months with transactions
    Given Maria planned and spent in the last 3 months
    When she downloads the AI export for 3 months
    Then the file explains the model, lists accounts, each month's envelopes and the transactions
    And contains nothing of João and no IDs or emails
```

## 18. Out of scope

- Calling an LLM from upareal (a later e14 story, if wanted).
- Charts.

## 19. Verification Script (Step-by-Step)

1. In **Meus dados → Análise com IA**, choose 3 meses and download. You get `upareal-analise-<month>.md`.
2. Open it. The explanation, accounts, monthly envelopes and transactions read well.
3. Paste it into an LLM and ask "Onde estou gastando mais do que planejei?". The answer uses your numbers.
4. Run `make preflight`. It ends green.

## 20. Traceability

- Decisions: X1/X2, privacy (no IDs/emails, no logging of content).
- Code and tests carry `// story: e14s01`.
