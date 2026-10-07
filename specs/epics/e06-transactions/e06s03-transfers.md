# e06s03 — Transfers between own accounts

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S11

## 1. Business narrative

Money moves between the user's own accounts: R$ 1.000,00 from Itaú to savings, R$ 800,00 from Itaú to pay the Nubank bill, or R$ 200,00 withdrawn to the wallet. A transfer is neither an expense nor an income. It only changes where the money is.

## 2. Actors

- **User**: on the register page of the source account.

## 3. Preconditions

- e06s02 is done.

## 4. Trigger

The user submits the **Transferência** form on an asset or liability register.

## 5. Main flow

1. The **Transferência** form has: destination (the user's other active asset and liability accounts), amount (more than zero), date, time and description.
2. `ledger.Transfer` posts one transaction through `postTransaction`: destination `+A`, source `−A`.
   - For example, paying the card from Itaú gives `Nubank +A` (the debt goes down) and `Itaú −A`.
3. Both registers show the transfer, each with the other account as counterpart.

## 6. Alternative flows

- **6a. Source and destination are the same:** 400.
- **6b. Either side isn't an asset or liability:** 400. Expenses and income use simple or split entries.
- **6c. Archived account or another user's account:** rejected (L3, L6).
- **6d. Amount zero, negative or malformed:** 400 "Valor inválido".

## 7. Business rules

- L1–L3 and L6 hold through `postTransaction`.
- A transfer between an **on-budget** and an **off-budget** account (for example Itaú → Tesouro Direto) is still a plain two-leg transfer in the ledger. How planning treats it (needing a category) is e08s05.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `ledger.TransferEntry{FromID, ToID string; Amount int64; At time.Time; Description string}` | |
| `(*Service).Transfer(ctx, userID, TransferEntry) (string, error)` | |
| `POST /contas/{id}/transferencia` | From account `{id}`; 303 to the source register month |

## 10. Non-functional

Single transaction.

## 11. Security & privacy

Both accounts are loaded with `user_id` scoping, and the form is protected by `CrossOriginProtection`.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Transfer between own asset and liability accounts, including card bill payment

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `ledger.Transfer` | Owns the transfer rules (both sides hold money, different accounts), so web doesn't build legs. |

## 15. Implementation steps

1. `Transfer` and its rules. → verify: `go test -race ./ledger/... -run TestTransfer`
2. Web: transfer form on the register, shown in both registers. → verify: `go test -race ./web/... -run TestTransferForm`
3. Full gate. → verify: `make preflight`

## 16. Risks

- **Card payment direction confusion.** The test pins it: paying R$ 800,00 makes the card show R$ 0,00 owed.

## 17. Acceptance criteria

```gherkin
Feature: Transfers

  Scenario: Pay the card bill
    Given Itaú has R$ 1.000,00 and Nubank shows R$ 800,00 a pagar
    When the user transfers "800,00" from Itaú to Nubank
    Then Itaú has R$ 200,00 and Nubank shows R$ 0,00
    And both registers list the transfer

  Scenario: Same account
    When the user transfers from Itaú to Itaú
    Then it is rejected
```

## 18. Out of scope

- Matching imported transfers (e07s03).
- Budget treatment of off-budget transfers (e08s05).

## 19. Verification Script (Step-by-Step)

1. Create Itaú (`1.000,00`) and a Nubank card owing `800,00`.
2. In Itaú → Lançamentos → **Transferência**, send `800,00` to Nubank.
3. Itaú's register shows "Nubank" -R$ 800,00 and a balance of R$ 200,00. Nubank's register shows "Itaú" and R$ 0,00.
4. On Contas, Nubank shows R$ 0,00.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S11.
- Decisions: `specs/planning-context.yaml` → `domain.ledger`, sign convention, `domain.invariants.ledger`.
- Code and tests carry `// story: e06s03`.
