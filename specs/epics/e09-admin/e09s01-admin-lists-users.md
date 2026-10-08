# e09s01 — Admin lists users

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 3 · **scope:** S20

## 1. Business narrative

The operator, the person who runs upareal, needs to know who signed up and who is active, so they can support people and later disable or delete accounts (e09s02). The admin page shows only identity data: name, email, sign-up date and last activity. It **never** shows accounts, transactions or budgets.

## 2. Actors

- **Admin**: a signed-in user whose Google email is in `ADMIN_EMAILS`.
- **User**: anyone else, who must not even learn that the page exists.

## 3. Preconditions

- e03 (sign-in, sessions) is done.

## 4. Trigger

The admin opens **Admin** (`/admin/usuarios`).

## 5. Main flow

1. `ADMIN_EMAILS` is an **optional** env var: comma-separated emails, compared trimmed and lowercased. When it's empty, nobody is an admin.
2. `users.NewService(conn, kr, users.AdminEmails(cfg.AdminEmails...))` gives `User.Admin`. It's computed from the decrypted email on every load and never stored, so editing the env var and restarting takes effect.
3. `users.ListUsers(ctx)` returns every user, oldest first, as `UserSummary{ID, Name, Email, CreatedAt, LastSeenAt (latest session activity; zero if none), Admin}`.
4. `GET /admin/usuarios` is for admins only. It shows a table with Nome, Email, Cadastro (dd/mm/aaaa), Último acesso (dd/mm/aaaa or "—") and an "admin" tag, plus the total count. The nav shows **Admin** only to admins.

## 6. Alternative flows

- **6a. A non-admin opens `/admin/usuarios`:** 404, the same page as any unknown URL.
- **6b. Signed out:** redirected home, like every app page.

## 7. Business rules

- The admin role comes **only** from `ADMIN_EMAILS` (planning-context `domain.users`).
- Admins never see financial data. The page reads only the users module.
- Logs record only user IDs, never emails.

## 8. Data

No schema change. One new query lists users with their latest `last_seen_at`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `config.Config.AdminEmails []string` | `ADMIN_EMAILS` (optional) |
| `users.AdminEmails(emails ...string) Option`; `NewService(conn, kr, opts ...Option)` | |
| `User.Admin bool` | |
| `(*users.Service).ListUsers(ctx) ([]UserSummary, error)` | |
| `GET /admin/usuarios` | Admin only, otherwise 404 |

## 10. Non-functional

There's a single query, and there are a handful of users (a private app).

## 11. Security & privacy

- Authorization is checked on the server for every admin route. Hiding the nav link is only cosmetic.
- Emails are decrypted only for the admin view, and they're never logged.
- Tests: a non-admin gets 404; the page contains no ledger or budget data.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Admin role from ADMIN_EMAILS
#### ADDED: R2: Admin user list without financial data

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `users.AdminEmails` option | Admin is a users-module concept. e09s02 needs it to protect admins from being disabled. A variadic option keeps every existing constructor call unchanged. |

## 15. Implementation steps

1. Config: `ADMIN_EMAILS`. → verify: `go test -race ./internal/config/... -run TestLoad`
2. Users: `AdminEmails` option, `User.Admin`, `ListUsers` with last activity. → verify: `go test -race ./users/... -run 'TestAdmin|TestListUsers'`
3. Web: `/admin/usuarios` (admin only, 404 otherwise) and the Admin nav link; main wires the config. → verify: `go test -race ./web/... -run TestAdminUsers`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **A misspelled `ADMIN_EMAILS` entry** means nobody is admin. That fails safe.

## 17. Acceptance criteria

```gherkin
Feature: Admin user list

  Scenario: Admin sees users
    Given ADMIN_EMAILS contains maria@example.com
    When Maria opens Admin
    Then she sees every user's name, email, sign-up date and last access

  Scenario: Non-admin
    When João opens /admin/usuarios
    Then he gets 404
```

## 18. Out of scope

- Disable, enable and delete (e09s02).
- Audit log of admin actions.

## 19. Verification Script (Step-by-Step)

1. Start the app with `ADMIN_EMAILS=<your Google email>` (e.g. `ADMIN_EMAILS=you@gmail.com make run`).
2. Sign in. The nav shows **Admin**. Open it and you see yourself tagged "admin", with today's sign-up and last access.
3. Sign in with another Google account in a private window. There's no Admin link, and `/admin/usuarios` shows "Página não encontrada".
4. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S20.
- Decisions: `specs/planning-context.yaml` → `domain.users` (admin role only from ADMIN_EMAILS; admins cannot see financial data), `privacy_security` (logs only user IDs).
- Code and tests carry `// story: e09s01`.
