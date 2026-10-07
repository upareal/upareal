# e07s04 — Generic OFX import

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S14

## 1. Business narrative

Most Brazilian banks export **OFX**, either as old SGML (1.x, often in Windows-1252) or as XML (2.x), for both checking accounts and credit cards. The user downloads the OFX and uploads it as is: no spreadsheet work and no reformatting. The lines land in the same review queue, and duplicates are skipped by the bank's own `FITID`.

## 2. Actors

- **User**: on the import page.

## 3. Preconditions

- e07s01 to e07s03 are done.
- Prior art: `aclindsa/ofxgo` v0.1.3 reads both OFX versions. It **ignores CHARSET**, so the text is converted before parsing (`specs/product/PRIOR_ART_LATEST.md`).

## 4. Trigger

The user uploads a `.ofx` file on **Importar**.

## 5. Main flow

1. **Format detection:** if the file (after BOM and spaces) starts with `OFXHEADER`, or its first 4 KB contain `<OFX>`, it's read as OFX. Otherwise it's read as the generic CSV.
2. **Charset:** if the bytes are not valid UTF-8, they're converted from **Windows-1252** to UTF-8. Windows-1252 also covers ISO-8859-1 for every printable character.
3. `ofxgo.ParseResponse` reads the bank statement (`STMTRS`) and the credit card statement (`CCSTMTRS`). Every `STMTTRN` becomes one line:
   - `ImportID` = `FITID`
   - `Amount` = `TRNAMT` in centavos. It must be a whole number of centavos.
   - `Description` = `NAME` plus `MEMO` (when they differ), trimmed to 200 characters.
   - `At` = `DTPOSTED`. A date **without a zone** is read as **São Paulo** local time, because ofxgo would assume GMT. A date with a zone (`[-3:BRT]`) keeps it.
4. The lines go through the same `ledger.Import`: queued against "A categorizar", duplicates skipped per account, all-or-nothing.

## 6. Alternative flows

- **6a. Unreadable OFX, or no transactions:** 400 "Não foi possível ler o OFX" or "O arquivo não tem lançamentos". Nothing is imported.
- **6b. A sub-centavo amount, or a transaction without FITID:** 400, naming the FITID or the transaction index.
- **6c. Over 5 MB or over 10.000 transactions:** 400 "grande demais".
- **6d. Investment statements** (`INVSTMTRS`): not supported. "O arquivo não tem lançamentos" is shown.

## 7. Business rules

- Same rules as e07s01: dedupe per account, all-or-nothing, nothing in plaintext.
- In a card statement, a purchase has a negative `TRNAMT` (money owed grows) and a payment a positive one. That's consistent with "negative = money left the account" from the card's point of view.

## 8. Data

No schema change. **The bank on each account is deferred to e07s05**, when there's a bank-specific parser to choose. Until then, detection by content is enough.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `importer.ParseOFX(r io.Reader) ([]Line, error)` | |
| `importer.Parse(r io.Reader) ([]Line, error)` | Detects OFX or CSV by content |
| Import page | Accepts `.ofx` and `.csv`. The help page mentions OFX. |

## 10. Non-functional

The whole file (≤ 5 MB) is read in memory, as in e07s01.

## 11. Security & privacy

- The decoder only sees ≤ 5 MB. `encoding/xml` has no external entities, so there's no XXE.

## 12. Dependencies (slopcheck)

| Package | Version | Tag | Evidence |
|---------|---------|-----|----------|
| `github.com/aclindsa/ofxgo` | v0.1.3 (pinned) | [OK] (last release 2021; OFX is frozen; covered by fixtures) | `ParseResponse(io.Reader) (*Response, error)`; `Response.Bank`/`CreditCard []Message`; `StatementResponse.BankTranList *TransactionList`; `Transaction{FiTID, DtPosted Date, TrnAmt Amount(big.Rat), Name, Memo}`; a date without a zone defaults to GMT (`types.go:153`) |
| `golang.org/x/text/encoding/charmap` | v0.42.0 (already direct) | [OK] | `charmap.Windows1252.NewDecoder()` |

## 13. Requirements

#### ADDED: R1: Import OFX 1.x (SGML) and 2.x (XML) bank and credit card statements
#### ADDED: R2: Windows-1252 files are converted; dates without a zone are São Paulo time
#### MODIFIED: Import format selection
**Before (slice):** "the account's bank selects its parser".
**After:** the format is detected by content. The per-account bank arrives with the first bank-specific parser (e07s05).

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `importer.Parse` | One entry point for the web layer. Adding a format means adding a detector and a parser, and the web layer doesn't change. |

There's still no `Parser` interface: two functions and a detector cover two formats.

## 15. Implementation steps

1. `ParseOFX` with fixtures: SGML 1.x bank in Windows-1252, XML 2.x card, date without zone, date with offset, cents, errors. → verify: `go test -race ./ledger/importer/... -run TestOFX`
2. `Parse` (detection), the web upload using it, and the help page mentioning OFX. → verify: `go test -race ./ledger/importer/... ./web/... -run 'TestDetect|TestImportFlow_OFX'`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **A bank sends a date with an explicit `[0:GMT]` meaning real GMT.** It would be read as São Paulo time (a 3-hour shift). That's rare, and it's documented by a `ponytail:` comment.
- **Bank quirks** (missing closing tags, unusual header lines). Mitigation: fixtures, and e07s05 bank-specific parsers.

## 17. Acceptance criteria

```gherkin
Feature: OFX import

  Scenario: SGML bank statement in Windows-1252
    Given an OFX 1.x file with "PADARIA SÃO JORGE" encoded in Windows-1252
    When the user imports it
    Then the line reads "PADARIA SÃO JORGE" in the review queue

  Scenario: Date without zone
    Given DTPOSTED "20260131"
    Then the line is dated 31/01/2026 00:00 São Paulo

  Scenario: Re-import by FITID
    When the same OFX is imported again
    Then nothing is added
```

## 18. Out of scope

- Bank-specific formats and the per-account bank (e07s05).
- Investment statements.

## 19. Verification Script (Step-by-Step)

1. Download an OFX from your bank (checking or card). If you don't have one, use the fixture `ledger/importer/testdata/bank_sgml_1252.ofx`.
2. Go to **Contas → (account) → Importar**, upload the `.ofx`, and check that the lines appear in **Revisão** with correct accents and dates.
3. Upload it again. You see "0 importadas, N já existentes".
4. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S14.
- Decisions: `specs/planning-context.yaml` → `domain.import` (OFX 1.x/2.x, transcode before ofxgo), `key_decisions` (transcode OFX charset).
- Code and tests carry `// story: e07s04`.
