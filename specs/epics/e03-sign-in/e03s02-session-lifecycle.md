# e03s02 — Session lifecycle and login protection

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 5 · **scope:** S04

## 1. Business narrative

A signed-in user stays signed in while they keep using upareal, but an abandoned session dies on its own. They can sign out, or sign out of every device after losing a phone. The login endpoints can't be hammered from one address.

## 2. Actors

- **User**: signs out, or signs out everywhere.
- **Attacker**: replays an old session cookie, or floods the login endpoints.

## 3. Preconditions

- e03s01 is done: sessions exist and store only SHA-256 token hashes.

## 4. Trigger

Any request with a session cookie, a logout button, or any request to `/auth/*`.

## 5. Main flow

1. On each request with a session cookie, the app loads the session.
2. **Idle expiry:** the session is dead if `last_seen_at` is more than **30 days** ago.
3. **Absolute expiry:** the session is dead if `created_at` is more than **90 days** ago, even when it's in active use.
4. Dead sessions are deleted and treated as signed out.
5. Live sessions refresh `last_seen_at`, at most once an hour (no write on every request).
6. **"Sair"** (POST `/auth/logout`) deletes this session, clears the cookie and redirects to `/`.
7. **"Sair de todos os dispositivos"** (POST `/auth/logout-all`) deletes all of the user's sessions, clears the cookie and redirects to `/`.
8. `/auth/login` and `/auth/callback` allow **10 requests per minute per client IP** (burst 10). Excess requests get `429`.

## 6. Alternative flows

- **6a. Logout without a session:** redirect to `/` (idempotent).
- **6b. Cross-site logout POST:** `403` from `CrossOriginProtection` (e03s01).
- **6c. Rate limit hit:** `429` page "Muitas tentativas. Aguarde um minuto e tente novamente."

## 7. Business rules

- Idle limit is 30 days and absolute limit is 90 days (`specs/planning-context.yaml`).
- Logging out of all devices ends every session of that user, including the current one.
- When a user creates a new session, their expired sessions are deleted.

## 8. Data

No schema change. `users_session.created_at` and `last_seen_at` already exist.

New queries: `GetSessionWithUser` (session plus user via `sqlc.embed`), `TouchSession`, `DeleteSession`, `DeleteUserSessions`, `DeleteExpiredUserSessions`.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `users.Service.UserBySession(ctx, token)` | Now also returns `ErrNoSession` for expired sessions and deletes them. Refreshes `last_seen_at` (throttled to 1 h). |
| `users.Service.EndSession(ctx, token) error` | Deletes one session. Unknown token is not an error. |
| `users.Service.EndAllSessions(ctx, userID) error` | Deletes all of a user's sessions. |
| `POST /auth/logout`, `POST /auth/logout-all` | 303 to `/` and the session cookie is cleared |
| Home (signed in) | Two forms with "Sair" and "Sair de todos os dispositivos" |

## 10. Non-functional

- The `last_seen_at` refresh is throttled to at most one write per session per hour.
- The rate limiter lives in memory. That's correct for the single-instance deployment.

## 11. Security & privacy

- Expired sessions can't be revived. They're deleted when found.
- Logout uses POST and is protected by `CrossOriginProtection`.
- The rate limit is keyed by client IP from `r.RemoteAddr`. **Behind Caddy (e10), RemoteAddr is the proxy.** e10 must forward the real client IP and the app must trust only Caddy's header. That's recorded as a risk and an e10 task.
- Server timeouts (e01s01) and the 10 s OIDC deadline (e03s01) already cover the "timeouts" item from the slice, so no new work is needed.

## 12. Dependencies (slopcheck)

| Package | Version | Tag | Evidence |
|---------|---------|-----|----------|
| `golang.org/x/time/rate` | v0.16.0 | [OK] | `NewLimiter(r Limit, b int)`, `(*Limiter).Allow() bool` (go doc) |

## 13. Requirements

#### ADDED: R1: Idle and absolute session expiry
Sessions die after 30 idle days or 90 days total.

#### ADDED: R2: Logout
A user ends the current session.

#### ADDED: R3: Logout everywhere
A user ends all of their sessions.

#### ADDED: R4: Login rate limit
`/auth/*` allows 10 requests per minute per client IP.

#### MODIFIED: Session resolution
**Before:** any stored session resolved to its user forever.
**After:** expired sessions resolve to `ErrNoSession` and are deleted. Live sessions refresh `last_seen_at` hourly.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `users/export_test.go` `SetNow` | Tests move the clock without adding a public clock option to `users.Service`. |
| `web.ipLimiter` | One small type keeps a limiter per IP and prunes idle entries, so the map can't grow forever. |

## 15. Implementation steps

1. Expiry: 30 days idle, 90 days absolute, hourly touch, expired-session cleanup on create. → verify: `go test -race ./users/... -run TestSessionExpiry`
2. `EndSession` and `EndAllSessions`, POST `/auth/logout` and `/auth/logout-all`, and the buttons on home. → verify: `go test -race ./users/... ./web/... -run TestLogout`
3. Per-IP rate limit on `/auth/*` with 429. → verify: `go test -race ./web/... -run TestLoginRateLimit`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Client IP behind a proxy.** Detect: in e10, all users share one limiter. Mitigation is an e10 task (trusted proxy header).
- **The throttled touch delays idle expiry by up to 1 hour.** That's acceptable against a 30-day limit.

## 17. Acceptance criteria

```gherkin
Feature: Session lifecycle

  Scenario: Idle session expires
    Given a session last seen 30 days and 1 minute ago
    When the user opens upareal
    Then they are signed out and the session is deleted

  Scenario: Absolute limit
    Given a session created 90 days and 1 minute ago that was used every day
    When the user opens upareal
    Then they are signed out

  Scenario: Active session stays alive
    Given a session last seen 29 days ago and created 60 days ago
    When the user opens upareal
    Then they are still signed in and last_seen_at is refreshed

  Scenario: Log out of all devices
    Given a user signed in on two browsers
    When they choose "Sair de todos os dispositivos" in one
    Then both browsers are signed out

  Scenario: Login flood
    Given 10 requests to /auth/login from one IP within a minute
    When an 11th arrives from the same IP
    Then it gets 429
    And a request from another IP still succeeds
```

## 18. Out of scope

- Trusted-proxy client IP (e10).
- Listing active sessions or devices.
- Rate limiting non-auth routes (import and export are rate limited in e06 and e09).

## 19. Verification Script (Step-by-Step)

1. Export the same variables as in e03s01 and run `make run`. Sign in with Google.
2. The home page shows "Sair" and "Sair de todos os dispositivos".
3. Click **Sair**. You're back on the signed-out home page with "Entrar com Google".
4. Sign in in Chrome and in a Firefox (or private) window. In one of them, click **Sair de todos os dispositivos**. Reload the other one, and it's signed out too.
5. Run `for i in $(seq 1 12); do curl -s -o /dev/null -w "%{http_code} " http://localhost:8080/auth/login; done; echo`. The last requests show `429`.
6. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S04.
- Decisions: `specs/planning-context.yaml` → `domain.users` (30 / 90 day sessions, log out of all devices), `defensive_code` (rate limit).
- Code and tests carry `// story: e03s02`.
