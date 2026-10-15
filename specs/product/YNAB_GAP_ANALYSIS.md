# YNAB API gap analysis

- **Date:** 2026-10-15
- **Source:** YNAB API reference v1.87.0 (`ynab_reference_api.yaml`, not committed): 44 endpoints, all schemas, operation descriptions, query parameters and request bodies.
- **Compared with:** upareal `main@8c3d687` and `specs/product/SCOPE_LATEST.yaml`.
- **Purpose:** input for choosing the next epics. Nothing here is planned yet.

## 1. New gaps (not in scope, not deferred)

Ranked by value. "Code" points to the upareal behavior that was checked.

| # | Gap | What YNAB does | What upareal does today | Why it matters |
|---|-----|----------------|-------------------------|----------------|
| G1 | **Split with a transfer** | A transfer to a tracking account can be split. | Split lines must be income or expense accounts. Code: `ledger/split.go:112-117`. | Blocks loan payments (principal + interest), ATM withdrawal + fee, investment contribution + costs. Foundation for G6. |
| G2 | **Fallback import ID** | Lines without a bank ID get `YNAB:<amount>:<date>:<occurrence>`, with the occurrence counting same-date, same-amount lines per account. | OFX lines without FITID are rejected ("lançamento sem FITID", `ledger/importer/ofx.go:87`); the CSV `id` column is required. | Some bank exports have no ID, and some banks issue a new FITID on every download, which duplicates lines on re-import. Matters for e07s05. |
| G3 | **Match an import to a typed entry** | Same account, same amount, date within ±10 days. Typed entries are match candidates; the match is recorded (`matched_transaction_id`). | Duplicates are detected only by import ID. Matching (`ledger/match.go:17`) pairs only the two sides of a transfer (±3 days). | A purchase typed by hand and later imported becomes a duplicate. Prevents wrong data. |
| G4 | **Pay down card debt over months** | Each card has a "Credit Card Payment" category. The user assigns money to it to pay off existing debt; its default goal is a monthly amount. | Existing card debt is reserved all at once; opening debt lowers "A distribuir" right away (`TestCards_OpeningDebtLowersToBeAssignedAndIsCovered`). | Carrying card debt (rotativo, parcelamento da fatura) is common in Brazil. No way to plan "R$ 500 a month toward this". |
| G5 | **Move money between categories, with history** | A "move money" action between two categories, or a category and Ready to Assign. Each move records month, time, from, to, amount and note; moves done together form a group. | The user edits two amounts by hand. Nothing records why the plan changed. | Core envelope gesture. The history explains the plan later. |
| G6 | **Loans and financing** | Mortgage, auto, student, personal loan and medical debt account types. Interest rate, minimum payment and escrow, each as values over time. Debt transaction types: payment, interest, fee, refund, charge, escrow, credit, balance adjustment. Debt payoff goal. | Only "Cartão ou dívida" (liability), with no interest or minimum payment. | Financiamento and consignado are common. Needs G1 to record payments. |
| G7 | **More target (meta) types** | Balance target with no date (TB), monthly funding (MF), spending targets with "refill up to" or "set aside another" and a repeat cadence (NEED), debt payoff (DEBT). Snooze a goal for a month. Funding still needed this month (`goal_under_funded`), which allows "fund everything that is behind". | e16 has one type: amount by a due month, no repeat (repeat rejected by the user on 2026-10-10). Suggestion per category only. | "Fill all metas" and "no date" (emergency fund) build directly on e16. |
| G8 | **Notes** | Notes on accounts, categories and months. | None. ("Memos per transaction leg" is deferred; this is different.) | "IPVA in 3 installments", "March: unusual expenses". |
| G9 | **Approval of imported lines** | Imported transactions start unapproved, even when categorized or matched automatically. Lists can filter "uncategorized" and "unapproved". | Revisão holds only uncategorized lines. | Today this is enough. Once G3 or auto-rules exist, auto-filled lines would skip the user's review. Plan with them. |
| G10 | **Future-dated entries** | Regular transactions cannot have a future date. Future ones are scheduled transactions (up to 5 years ahead). | No date check in `RecordSimple` / `RecordSplit`. A future entry counts in today's balance and in that month's plan. | Decide if this is intended. Otherwise reject future dates until recurring transactions exist. |
| G11 | **Bulk edit of transactions** | Update many transactions in one call, by ID or import ID. | One line at a time. | Categorize many lines in Revisão at once. |
| G12 | **Flags** | Six colors on transactions, each with an optional custom name. | None. | "Reimbursable", "check with spouse". |
| G13 | **Age of Money** | Days between earning a real and spending it, per month. | None. | Can be computed from existing data, nothing stored. |
| G14 | **Several budgets per user** | One user has several plans. | One budget per user. | Personal vs MEI or small business. Different from sharing (deferred). |
| G15 | **Public API** | OAuth for apps, personal tokens, delta sync (`last_knowledge_of_server`), full plan export, bulk create reporting duplicate import IDs. | Only the JSON data export and the AI Markdown export. | Would let an AI assistant read and write the budget (fits e14). |
| G16 | **Payee name cleanup** | Keeps the original statement text, the imported text and the payee after rename rules. | Keeps the raw description and the bank's account text; manual edit only. | Goes with the deferred auto-categorization rules. |
| G17 | **Account subtypes** | Checking, savings, cash, credit card, line of credit, other asset/liability, loan types. | "Conta ou dinheiro", "Cartão ou dívida", plus the card flag. | Group lists ("Investimentos", "Financiamentos"). Minor until G6. |

## 2. Already deferred in SCOPE_LATEST.yaml

YNAB has these too:

- Automatic bank sync (Open Finance in Brazil). YNAB's transaction lists also exclude pending transactions.
- Auto-categorization rules.
- Reconciliation and cleared/uncleared balances.
- Recurring and scheduled transactions (YNAB supports 13 frequencies).
- Reports.
- Sharing a budget (YNAB records who made each change).
- Multiple currencies, other date and number formats.
- Memos on each part of a split.
- Refunds (also in `specs/state.yaml` → `pending`).

## 3. Covered, or better in upareal

- **Month boundaries:** YNAB computes "current month" in UTC; upareal uses America/Sao_Paulo.
- **Planning ahead:** amounts can be set in future months on Orçamento.
- **Duplicate imports:** a repeated import ID is skipped.
- **Deletes:** the edit history plays the role of YNAB's `deleted` markers.
- **Archive:** covers YNAB's hidden categories and closed accounts.

## 4. Not worth copying

- **Payee GPS locations:** only useful in a native mobile app.
- **The internal "Inflow: Ready to Assign" category:** upareal's income categories with expected amounts are a deliberate difference.

## 5. Design notes from the same discussion

- **One journal stays.** Separate tables per transaction type were considered and rejected: real events cross types (splits, bill payments, refunds), the type changes after import ("É uma transferência"), and balances would become unions over many tables. A transaction's kind is derived from its legs' account types. Type-specific data (loan terms, installments, card closing day, import matches) goes in side tables keyed by transaction or account.
- **G1 before G6.** Loans need a split that mixes a transfer leg with an expense leg.
- **G3 and G9 together.** Automatic matching needs an approval step so the user still reviews what was filled in.

## 6. Suggested order

1. **G1** — Split with a transfer. Unlocks loans, fees and contributions.
2. **G2** — Fallback import ID. Prevents duplicates; helps e07s05.
3. **G3** — Match an import to a typed entry (plan with G9).
4. **G4** — Pay down card debt over months.
5. **G5** and **G7** — envelope workflow: move money, more meta types.

## 7. Open questions

- Do your bank exports (OFX and CSV) always have IDs, and are FITIDs stable across downloads? Sample files would settle G2.
- Should future-dated entries be allowed (G10)?
- For G4: should existing card debt still be reserved at once by default, with an opt-in paydown plan, or the reverse?
- Which loan types matter first for G6: financiamento imobiliário, veículo, consignado, empréstimo pessoal?
