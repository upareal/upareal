# e09s02 — Disable, enable and delete users

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S20

## 1. Business narrative

The operator can lock someone out right away (**Desativar**), let them back in (**Reativar**), or remove them completely (**Excluir**). Deleting erases the person's accounts, transactions, budget, mappings and encryption keys in every module. The people in `ADMIN_EMAILS` are protected, so the operator can't lock themselves out by mistake.

## 2. Actors

- **Admin**: on `/admin/usuarios`.
- **Disabled user**: tries to sign in or keeps using an open tab.

## 3. Preconditions

- e09s01 is done.

## 4. Trigger

The admin clicks Desativar, Reativar, or Excluir (with a confirmation checkbox).

## 5. Main flow

1. **Disable** `users.DisableUser(ctx, id)`:
   - sets `disabled_at` and deletes **all** the user's sessions in the same transaction;
   - after that, `SignIn` returns `ErrDisabled`, and `UserBySession` treats a disabled user as having no session.
2. **Enable** `users.EnableUser(ctx, id)`: clears `disabled_at`.
3. **Delete** `users.DeleteUser(ctx, id, purge)`:
   - refuses protected users (admins);
   - runs `purge(ctx, id)`, which calls `planning.DeleteUserData`, `budget.DeleteUserData` and `ledger.DeleteUserData` in that order. Each module deletes its own rows in one transaction, in foreign-key order, including its data-key table;
   - then deletes the user row (sessions cascade).
4. Admin page:
   - each non-admin row has Desativar or Reativar, and Excluir with a "confirmo" checkbox;
   - disabled rows show "desativado";
   - every action redirects back to the list.
5. Sign-in for a disabled user: 403, "Sua conta está desativada. Fale com o administrador."
6. Every admin action is logged as `admin_id`, `user_id` and `action`. Only IDs are logged.

## 6. Alternative flows

- **6a. Acting on an admin** (including yourself): `ErrProtectedUser` → 400 "Administradores não podem ser desativados nem excluídos." Nothing changes.
- **6b. Excluir without the checkbox:** 400 "Marque a caixa para confirmar a exclusão." Nothing is deleted.
- **6c. An unknown user ID:** 404.
- **6d. Purge fails partway:** the user row still exists, so the admin retries. Every module delete is idempotent.
- **6e. A non-admin posts to an admin route:** 404, as in e09s01.

## 7. Business rules

- Disabling or deleting removes sessions immediately (planning-context `domain.users`).
- Users in `ADMIN_EMAILS` can't be disabled or deleted through the UI.
- X1 and X2: each module deletes only its own tables, filtered by `user_id`.

## 8. Data

| Change | Detail |
|--------|--------|
| `users/migrations/00003_disabled.sql` | `ALTER TABLE users_user ADD COLUMN disabled_at TEXT` |
| `ledger`, `budget`, `planning` | New `DeleteUserData` queries, with no schema change |

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `(*users.Service).DisableUser / EnableUser(ctx, id string) error` | `ErrProtectedUser`, `ErrNotFound` |
| `(*users.Service).DeleteUser(ctx, id string, purge func(context.Context, string) error) error` | Purges, then deletes the user row |
| `users.ErrDisabled` | Returned by `SignIn` |
| `UserSummary.Disabled bool` | |
| `(*ledger/budget/planning.Service).DeleteUserData(ctx, userID string) error` | Idempotent |
| `POST /admin/usuarios/{id}/desativar`, `/reativar`, `/excluir` (`confirmo`) | Admin only |

## 10. Non-functional

One transaction per module. A user's data is small.

## 11. Security & privacy

- Authorization is checked on the server for every POST.
- Deleting the data-key rows crypto-shreds anything sealed. Backups are handled in e10 and e11.
- Tests:
  - user B's data survives user A's deletion in every module;
  - a disabled session is refused at once;
  - a non-admin POST gets 404.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Disable/enable users; disabled users cannot sign in and lose their sessions
#### ADDED: R2: Delete a user and all their data in every module
#### ADDED: R3: Admins are protected from disable/delete

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `users.DeleteUser(…, purge)` | One entry point enforces the order: protection check, then other modules, then the user row. e10's self-service deletion reuses it. |
| `DeleteUserData` per module | Each module knows its own tables and foreign keys. No module touches another's tables (X1). |

## 15. Implementation steps

1. Users: migration, disable/enable (sessions removed, admins protected), sign-in and session refusal, `DeleteUser`, `Disabled` in the list. → verify: `go test -race ./users/... -run 'TestDisable|TestDeleteUser'`
2. `DeleteUserData` in ledger, budget and planning, with other users untouched. → verify: `go test -race ./ledger/... ./budget/... ./planning/... -run TestDeleteUserData`
3. Web: admin actions, the disabled sign-in message, and logging. → verify: `go test -race ./web/... -run 'TestAdminUsers|TestDisabled'`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **A new table added later is forgotten in `DeleteUserData`.** Mitigation: each module's test counts the rows of **every** table in its schema (read from `sqlite_master`) for the deleted user.

## 17. Acceptance criteria

```gherkin
Feature: Disable and delete users

  Scenario: Disable
    Given João is signed in
    When the admin disables João
    Then João's next request is signed out and he cannot sign in again

  Scenario: Delete
    When the admin deletes João (confirmed)
    Then no row of João remains in any module and Maria's data is intact

  Scenario: Protected admin
    When an admin tries to disable another admin
    Then it is refused
```

## 18. Out of scope

- Self-service export and deletion (e10).
- Purging backups (e11 Litestream retention).

## 19. Verification Script (Step-by-Step)

1. Run with `ADMIN_EMAILS=<you>` and sign in. In a private window, sign in with a second Google account and create an account (Contas) there.
2. As admin, click **Desativar** on the second user. Reload the private window: it's signed out, and signing in shows "Sua conta está desativada".
3. Click **Reativar**. The second user can sign in again, and their data is still there.
4. Click **Excluir** without the checkbox. It's refused. Check the box and delete. The user leaves the list. Signing in again creates a fresh, empty account.
5. Your own row has no buttons.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S20.
- Decisions: `specs/planning-context.yaml` → `domain.users` (disable/delete removes sessions immediately; disabled users cannot sign in; ADMIN_EMAILS users cannot be disabled/deleted), `module_rules` (cross-module integrity in Go via `DeleteUserData`).
- Code and tests carry `// story: e09s02`.
