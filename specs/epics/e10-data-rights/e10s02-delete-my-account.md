# e10s02 — Delete my account

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S21

## 1. Business narrative

Under LGPD (art. 18, VI), a person can have their data erased. In **Meus dados**, the user types their email to confirm and deletes their account. Everything is removed from every module, they're signed out on every device, and signing in again starts from an empty account.

## 2. Actors

- **User**: signed in, on `/conta/dados`.

## 3. Preconditions

- e09s02 is done: `users.DeleteUser(…, purge)` and `DeleteUserData` exist in every module.

## 4. Trigger

The user types their email and clicks **Excluir minha conta**.

## 5. Main flow

1. `/conta/dados` gets an **Excluir minha conta** section. It explains that the deletion is permanent and suggests downloading the data first. There's an email field and the button.
2. `POST /conta/excluir` compares `confirmar` (trimmed, case-insensitive) with the signed-in user's email. It then calls `users.DeleteUser(ctx, u.ID, purgeUserData(d))`, the same purge order as the admin deletion (planning, budget, ledger, then the user and their sessions).
3. The response clears the session cookie and shows **"Conta excluída"** (200), with a link to the home page.
4. The log records `user deleted own account` with `user_id` only.

## 6. Alternative flows

- **6a. The email doesn't match:** 400, "Digite seu e-mail exatamente como aparece para confirmar." Nothing is deleted.
- **6b. The user is an admin (`ADMIN_EMAILS`):** 400, "Administradores não podem excluir a própria conta. Remova seu e-mail de ADMIN_EMAILS antes." The users module protects admins.
- **6c. The purge fails partway:** 500 and the user still exists, so retrying finishes the job (every module delete is idempotent).
- **6d. Signed out:** redirected home.

## 7. Business rules

- **Deletion is immediate in the live database.**
- **Backups:** encrypted copies taken before the deletion are erased when backup retention expires (planned 30 days, e11). The page says so plainly.

## 8. Data

No schema change.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `POST /conta/excluir` (`confirmar`) | Signed in. Answers 200 "Conta excluída" with the cookie cleared, or 400. |

## 10. Non-functional

Same as the admin deletion.

## 11. Security & privacy

- Protected by `CrossOriginProtection` and typed confirmation.
- The user can only delete themselves. The ID comes from the session, never from the form.
- **Known gap.** The wrapped data keys sit in the same database as the data, so they appear in backups too. True crypto-shredding of backups needs keys kept outside the backed-up database. This is recorded in `state.yaml` pending for e11.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Self-service account deletion with typed email confirmation

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `purgeUserData` (web) | Shared with the admin deletion, so both paths remove the same data in the same order. |

## 15. Implementation steps

1. Web: the Excluir section, `POST /conta/excluir`, the cookie cleared, the confirmation page, and refusals for admins and mismatched emails. → verify: `go test -race ./web/... -run TestDeleteMyAccount`
2. Full gate. → verify: `make preflight`

## 16. Risks

- **The backup gap** (§11). It's documented, not hidden.

## 17. Acceptance criteria

```gherkin
Feature: Delete my account

  Scenario: Confirmed
    When João types joao@example.com and clicks Excluir minha conta
    Then his data is gone from every module, he is signed out, and signing in starts empty

  Scenario: Wrong confirmation
    When he types something else
    Then nothing is deleted
```

## 18. Out of scope

- Backup retention configuration (e11).
- A grace period or undo.

## 19. Verification Script (Step-by-Step)

1. Sign in with a non-admin account (start without `ADMIN_EMAILS`, or use a second Google account). Create an account in Contas.
2. In **Meus dados**, type a wrong email and click **Excluir minha conta**. It's refused.
3. Type your email and confirm. You see "Conta excluída" and you're signed out.
4. Sign in again. You're asked to accept the terms, and Contas is empty.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S21.
- Decisions: `specs/planning-context.yaml` → `domain.users` ("self-service export my data and delete my account in v1"), `privacy_security` (crypto-shredding: known gap noted), X1.
- Code and tests carry `// story: e10s02`.
