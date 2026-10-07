# e04s02 — Create accounts with opening balance and manual adjustment

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 8 · **scope:** S07, S09, S22

## 1. Business narrative

A user records where their money is and where it goes: "Conta corrente Itaú" holds R$ 2.500,00, the "Nubank" card has R$ 800,00 to pay, and "Posto Shell" or "Salário" are where money goes to and comes from. Every real starts life in a balanced transaction. An opening balance is balanced against "Saldo inicial", and so is a later correction ("o banco diz R$ 2.480,00").

## 2. Actors

- **User**: creates accounts, sets opening balances and adjusts balances.

## 3. Preconditions

- e04s01 is done: system accounts exist.
- Conventions: amounts are debit-positive `int64` centavos. Liabilities carry `is_card` (`specs/planning-context.yaml`).

## 4. Trigger

The user opens **Contas** (`/contas`), creates an account, or adjusts a balance.

## 5. Main flow

1. `/contas` lists the active accounts grouped by type: **Contas e dinheiro** (asset), **Cartões e dívidas** (liability), **Receitas** (income), **Despesas** (expense). System accounts are hidden.
   - Asset and liability rows show the balance. A liability shows the amount owed as positive ("R$ 800,00 a pagar").
2. **Create account:** name, type, plus:
   - for **asset**: an "No orçamento" checkbox (default on);
   - for **liability**: "É cartão de crédito" (default on) and "No orçamento" (forced on for cards);
   - for asset and liability: an optional opening balance and date (default today, São Paulo time).
3. A non-zero opening balance posts one transaction on that date:
   - asset: `account +X | Saldo inicial −X`;
   - liability (amount owed X): `account −X | Saldo inicial +X`.
4. **Adjust balance** (asset and liability): the user types the real balance, and the difference posts against Saldo inicial with the description "Ajuste de saldo".
5. Every write goes through one function, `postTransaction`, which enforces L1–L3 and L6 in the same database transaction.

## 6. Alternative flows

- **6a. Empty name, unknown type, malformed amount** → 400 with a pt-BR message, and nothing is created.
- **6b. Opening balance of zero or empty** → the account is created with no transaction.
- **6c. Adjusting to the current balance** → no transaction.
- **6d. Card with `on_budget` off** → it's stored on anyway (cards are always on-budget).
- **6e. Income or expense with an opening balance** → rejected. Only assets and liabilities hold balances.

## 7. Business rules (invariants enforced here)

- **L1** A transaction has at least 2 legs, and the legs sum to zero.
- **L2** Every leg is non-zero `int64` centavos.
- **L3** Every leg's account belongs to the transaction's user.
- **L4** A balance is the sum of legs, never stored.
- **L6** Archived accounts take no new legs. The check runs now, and archiving arrives in e04s03.

## 8. Data

Migration `ledger/migrations/00002_transactions.sql`:

| Change | Columns |
|--------|---------|
| `ledger_account` + | `on_budget INTEGER NOT NULL DEFAULT 0 CHECK (on_budget IN (0,1))`, `is_card INTEGER NOT NULL DEFAULT 0 CHECK (is_card IN (0,1))`, `CHECK (is_card = 0 OR (type = 'liability' AND on_budget = 1))` |
| `ledger_transaction` | `id TEXT PK`, `user_id TEXT NOT NULL`, `occurred_at TEXT NOT NULL` (UTC), `description BLOB NOT NULL` (sealed, field `"description"`), `created_at TEXT NOT NULL` |
| `ledger_leg` | `id TEXT PK`, `transaction_id TEXT NOT NULL REFERENCES ledger_transaction(id) ON DELETE CASCADE`, `account_id TEXT NOT NULL REFERENCES ledger_account(id)`, `amount INTEGER NOT NULL CHECK (amount <> 0)` |

Indexes: `ledger_leg(account_id)`, `ledger_transaction(user_id, occurred_at)`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `internal/money.ParseBRL(s) (int64, error)` | Accepts `1.234,56`, `1234,56`, `1234`, `-50,00`, `R$ 10,00`. Rejects more than 2 decimal places, letters and overflow. |
| `internal/money.FormatBRL(c int64) string` | `R$ 1.234,56`, `-R$ 50,00` |
| `ledger.NewAccount{Name, Type, OnBudget, IsCard, Opening int64, OpeningAt time.Time}` | |
| `(*Service).CreateAccount(ctx, userID, NewAccount) (Account, error)` | |
| `(*Service).Balance(ctx, userID, accountID) (int64, error)` | Debit-positive |
| `(*Service).AdjustBalance(ctx, userID, accountID, target int64, at time.Time) error` | `target` is debit-positive |
| `Account` + | `OnBudget, IsCard bool` |
| `GET /contas`, `POST /contas`, `POST /contas/{id}/ajuste` | Plain forms with POST then redirect (303) to `/contas` |

## 10. Non-functional

- A balance is one `SUM` query per account. The accounts page runs a single grouped `SUM` for all accounts.

## 11. Security & privacy

- Account names and transaction descriptions are sealed. Amounts and dates stay plaintext, as decided.
- L3 means a user can't post a leg to another user's account, even with a forged ID. Tested.
- Forms are protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Zero-sum transaction core (L1–L3, L6)
All ledger writes go through `postTransaction`, which rejects invalid transactions as a whole.

#### ADDED: R2: Create accounts of every type
With `on_budget` and `is_card` rules.

#### ADDED: R3: Opening balance and adjustment
Both are balanced against Saldo inicial.

#### ADDED: R4: Accounts page
Grouped by type, with balances in pt-BR format.

#### ADDED: R5: BRL parsing and formatting
`internal/money`.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.postTransaction` | The **only** ledger write path for legs, so L1–L3 and L6 can't be bypassed. Later stories (transactions, imports) reuse it. |
| `internal/money` | Every module and the web layer parse and format BRL the same way. |

## 15. Implementation steps

1. `internal/money` parse and format. → verify: `go test -race ./internal/money/... -run TestBRL`
2. Migration 00002 plus queries. → verify: `make gen && go build ./... && go test -race ./ledger/... -run TestEnsureDefaults`
3. `postTransaction` enforcing L1–L3 and L6. → verify: `go test -race ./ledger/... -run TestTransactionInvariants`
4. `CreateAccount` (types, on-budget and card rules, opening balance). → verify: `go test -race ./ledger/... -run TestCreateAccount`
5. `Balance` and `AdjustBalance` (L4). → verify: `go test -race ./ledger/... -run TestBalance`
6. `/contas` page with the create and adjust forms. → verify: `go test -race ./web/... -run TestAccountsPage`
7. Proof: no plaintext account names or descriptions at rest, and isolation for balances. → verify: `go test -race ./ledger/... -run 'TestLedgerHasNoPlaintext|TestIsolation'`
8. Full gate. → verify: `make preflight`

## 16. Risks

- **Sign confusion in the UI.** Mitigation: one helper converts the stored balance to the displayed amount for each type, and it's tested with an asset, a card and income.

## 17. Acceptance criteria

```gherkin
Feature: Accounts and opening balances

  Scenario: Checking account with opening balance
    When the user creates asset "Conta corrente" with opening balance "2.500,00"
    Then its balance is R$ 2.500,00
    And Saldo inicial changed by -R$ 2.500,00

  Scenario: Card with debt
    When the user creates card "Nubank" owing "800,00"
    Then the card shows "R$ 800,00 a pagar"
    And the card is on-budget

  Scenario: Balance adjustment
    Given "Conta corrente" at R$ 2.500,00
    When the user adjusts it to "2.480,00"
    Then its balance is R$ 2.480,00 via one transaction of -R$ 20,00 against Saldo inicial

  Scenario: Unbalanced transaction is never saved
    When legs +100 and -99 are posted
    Then nothing is saved

  Scenario: Foreign account
    When user B posts a leg to user A's account
    Then nothing is saved
```

## 18. Out of scope

- Renaming, reordering, archiving, deleting and changing type (e04s03).
- General transaction entry, splits and editing (e06).
- Bank selection for imports (e07s04).

## 19. Verification Script (Step-by-Step)

1. Export the variables and run `make run`. Sign in. The home page links to **Contas**.
2. Create asset "Conta corrente" with opening balance `2.500,00`. It shows R$ 2.500,00 under "Contas e dinheiro".
3. Create card "Nubank" owing `800,00`. It shows "R$ 800,00 a pagar" under "Cartões e dívidas".
4. Create expense "Posto Shell" and income "Salário". They appear under Despesas and Receitas, with no balance shown.
5. Adjust "Conta corrente" to `2.480,00`. It shows R$ 2.480,00.
6. Try an opening balance of `12,345`. You get a pt-BR error and nothing is created.
7. Run `strings /tmp/upareal.db | grep -i -E "nubank|conta corrente|posto shell"`. There's no output.
8. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S07, S09, S22.
- Decisions: `specs/planning-context.yaml` → `domain.invariants.ledger`, `key_decisions` (sign convention, is_card).
- Code and tests carry `// story: e04s02`.
