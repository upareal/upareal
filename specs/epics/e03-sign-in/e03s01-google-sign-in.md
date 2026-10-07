# e03s01 — First Google sign-in creates the account

**type:** feat · **risk:** P0 · **context:** domain · **bcps:** 8 · **scope:** S03, S04 (session start)

## 1. Business narrative

A visitor opens upareal, clicks "Entrar com Google", and comes back signed in. The first visit creates the account automatically; later visits recognize the same Google account. Their email and name are stored encrypted, and the home page greets them by first name.

## 2. Actors

- **Visitor / user** — signs in with Google.
- **Google** — OpenID Connect provider (`https://accounts.google.com`).
- **Fake OIDC provider** — stands in for Google in all automated tests.

## 3. Preconditions

- e02 done: `internal/vault` exists.
- For the manual check only: a Google OAuth client (Web application) with redirect URI `http://localhost:8080/auth/callback`.

## 4. Trigger

The visitor clicks "Entrar com Google" (`GET /auth/login`).

## 5. Main flow

1. `GET /auth/login` creates a random **state**, **nonce** and **PKCE verifier**, stores them in a short-lived HttpOnly cookie, and redirects to Google with scopes `openid email profile`, the state, the nonce and the S256 code challenge.
2. Google redirects back to `GET /auth/callback?code&state`.
3. The app checks that the state matches the cookie, exchanges the code with the PKCE verifier, and verifies the ID token (signature, issuer, audience, expiry, **nonce**).
4. The app requires `email_verified = true`.
5. `users.SignIn` looks the user up by `BlindIndex("google-sub", sub)`:
   - **not found** → creates the user: UUIDv7 ID, new wrapped data key, encrypted email and name.
   - **found** → updates the encrypted email and name if they changed.
6. The app creates a session: 32 random bytes, stores only the SHA-256 hash, and sets the session cookie.
7. Redirect to `/`, which shows "Olá, <first name>".

## 6. Alternative flows

- **6a. State missing or different** → `400`, no session.
- **6b. Code exchange or ID token verification fails** (bad signature, audience, issuer, expiry, nonce) → `401` page "Não foi possível entrar. Tente novamente.", no session.
- **6c. `email_verified` false or missing** → `403` page explaining the Google email must be verified, no user created.
- **6d. User denies consent** (`error=access_denied`) → redirect to `/` with no session.
- **6e. Session cookie unknown or tampered** → treated as signed out.
- **6f. Required config missing** (`MASTER_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`) → startup fails naming the variable.

## 7. Business rules

- A Google account (`sub`) maps to exactly one user.
- Only verified Google emails may sign in.
- Email and name are always encrypted; the `sub` itself is never stored, only its blind index.
- Sessions are server-side; the cookie holds only an opaque random token.

## 8. Data

Module `users`, goose table `goose_users`, prefix `users_`:

| Table | Columns |
|-------|---------|
| `users_user` | `id TEXT PK` (UUIDv7), `sub_idx BLOB UNIQUE NOT NULL`, `data_key BLOB NOT NULL` (wrapped), `email BLOB NOT NULL`, `name BLOB NOT NULL`, `created_at TEXT NOT NULL` (UTC RFC 3339) |
| `users_session` | `token_hash BLOB PK` (SHA-256), `user_id TEXT NOT NULL REFERENCES users_user(id) ON DELETE CASCADE`, `created_at TEXT NOT NULL`, `last_seen_at TEXT NOT NULL` |

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `GET /auth/login` | 302 to provider; sets `upareal_oauth` cookie (HttpOnly, Secure, SameSite=Lax, Path=/auth, 10 min) |
| `GET /auth/callback` | 302 to `/` with `upareal_session` cookie (HttpOnly, Secure, SameSite=Lax, Path=/); errors per §6 |
| `GET /` | Signed out: link "Entrar com Google". Signed in: "Olá, <first name>" |
| Env | Required: `MASTER_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`. Optional: `OIDC_ISSUER_URL` (default `https://accounts.google.com`) |
| `users.Service` | `SignIn(ctx, Identity) (User, error)`, `CreateSession(ctx, userID) (token string, error)`, `UserBySession(ctx, token) (User, error)` |
| `users.Google` | `AuthCodeURL(state, nonce, verifier) string`, `Exchange(ctx, code, verifier, nonce) (Identity, error)` |
| `internal/db.Migrate(ctx, conn, module, fsys)` | Applies a module's embedded goose migrations with table `goose_<module>` |

## 10. Non-functional

- Context deadline (10 s) on the token exchange and discovery calls.
- Startup does not call Google: discovery happens on construction of `users.Google` in `run`, which uses `OIDC_ISSUER_URL`; tests point it at the fake provider.

## 11. Security & privacy

- PKCE (S256) + state + nonce; ID token verified by go-oidc (signature via JWKS, issuer, audience, expiry).
- `email_verified` required.
- Session token: 32 bytes from `crypto/rand`, only SHA-256 stored; cookie HttpOnly + Secure + SameSite=Lax.
- `http.CrossOriginProtection` wraps the whole handler (rejects cross-origin unsafe requests).
- Logs never contain email, name, sub, tokens or codes.
- **Note:** `Secure` cookies work on `http://localhost` in Chrome and Firefox (localhost is a secure context). Other hosts over plain HTTP will not keep the session — production is HTTPS (e10).

## 12. Dependencies (slopcheck)

| Package | Version | Tag | Evidence |
|---------|---------|-----|----------|
| `github.com/coreos/go-oidc/v3` | v3.21.0 | [OK] | `IDToken.Nonce`, `IDToken.Subject`, `IDToken.Claims(v)`; `oidc.Config{ClientID, Now}`; `InsecureIssuerURLContext` exists (not used) |
| `golang.org/x/oauth2` | v0.37.0 | [OK] | `GenerateVerifier()`, `S256ChallengeOption(verifier)`, `VerifierOption(verifier)` |
| `github.com/pressly/goose/v3` | v3.28.0 | [OK] | `NewProvider(DialectSQLite3, db, fsys, WithTableName(...))`, `Provider.Up(ctx)` |
| `github.com/sqlc-dev/sqlc` (tool) | v1.31.1 | [OK] | Builds with `CGO_ENABLED=0`; reads goose migrations as schema |
| `github.com/google/uuid` | v1.6.0 | [OK] | `uuid.NewV7()` |
| `github.com/go-jose/go-jose/v4` | v4.1.4 (already transitive via go-oidc) | [OK] | Signs ID tokens in the fake provider (tests only) |
| stdlib `net/http.CrossOriginProtection` | go1.26 | [OK] | `NewCrossOriginProtection()`, `Handler(h)` |

## 13. Requirements

#### ADDED: R1 — Sign in with Google
A visitor signs in with Google (OIDC + PKCE + state + nonce).

#### ADDED: R2 — Automatic registration
First sign-in creates the user; later sign-ins with the same `sub` reuse it.

#### ADDED: R3 — Verified email only
Accounts with unverified email are refused.

#### ADDED: R4 — Encrypted identity
Email and name are encrypted; `sub` stored only as a blind index.

#### ADDED: R5 — Server-side session
A session cookie with an opaque token identifies the signed-in user.

#### ADDED: R6 — Required config
`MASTER_KEY`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL` are required; local dev passes them from the shell.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `internal/db.Migrate` | Every module needs the same goose wiring with its own version table. |
| `users.Service` | Owns user + session persistence and the vault calls; web never touches users tables. |
| `users.Google` | Hides OIDC/PKCE/nonce/email_verified checks behind 2 methods; web only handles HTTP and cookies. |
| `users/internal/store` (sqlc) | Generated typed queries, private to the module. |
| `internal/oidctest` | Fake OIDC provider shared by `users` and `web` tests (two packages need it). |

No interfaces: `users.Google` is concrete; tests point it at the fake provider through the issuer URL.

## 15. Implementation steps

0. Config: require `MASTER_KEY` (parsed with `vault.ParseMasterKey`), `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `BASE_URL`; optional `OIDC_ISSUER_URL`. `compose.yaml` passes them from the shell with `${VAR:?message}`. → verify: `go test -race ./internal/config/... -run TestLoad && MASTER_KEY=x GOOGLE_CLIENT_ID=x GOOGLE_CLIENT_SECRET=x docker compose config -q`
1. `internal/db.Migrate` with per-module goose table. → verify: `go test -race ./internal/db/... -run TestMigrate`
2. Users schema (goose SQL) + sqlc queries + `sqlc.yaml`; pin sqlc tool; `make gen` runs sqlc too. → verify: `make gen && go build ./users/...`
3. `internal/oidctest` fake provider (discovery, JWKS, authorize, token) with configurable user claims. → verify: `go test -race ./internal/oidctest/...`
4. `users.Service.SignIn`: create by `sub` index, reuse on return, update changed email/name, refuse unverified. → verify: `go test -race ./users/... -run TestSignIn`
5. `users.Google`: auth URL carries state, nonce, S256 challenge; `Exchange` verifies token + nonce + email_verified. → verify: `go test -race ./users/... -run TestGoogle`
6. Sessions: `CreateSession` / `UserBySession`; only the hash stored. → verify: `go test -race ./users/... -run TestSession`
7. Web flow: `/auth/login`, `/auth/callback`, greeting on `/`; error pages per §6; `run` wires migrations, vault, users, Google. → verify: `go test -race ./web/... ./cmd/upareal/... -run 'TestGoogleSignIn|TestGracefulShutdown'`
8. `CrossOriginProtection` on the handler. → verify: `go test -race ./web/... -run TestCrossOrigin`
9. Proof: after sign-in, the database copy holds no plaintext email, name or sub. → verify: `go test -race ./users/... -run TestUsersTableHasNoPlaintext`
10. Full gate. → verify: `make preflight`

## 16. Risks

- **Fake provider diverges from Google.** Mitigation: go-oidc does the real verification in both cases; the manual check uses real Google.
- **`Secure` cookie on plain HTTP.** Works on `localhost` in Chrome/Firefox; Safari may not. Detect: §19 step 3.
- **Discovery at startup needs network.** Mitigation: if discovery fails, startup fails with a clear error naming `OIDC_ISSUER_URL`.
- **Large story (8 BCP).** Mitigation: commits per task; each task leaves the gate green.

## 17. Acceptance criteria

```gherkin
Feature: Sign in with Google

  Scenario: First sign-in creates the account
    Given no user exists for Google sub "g-123"
    When the visitor completes Google sign-in as sub "g-123", "Maria Silva", verified email
    Then a user is created with encrypted email and name
    And the visitor lands on "/" seeing "Olá, Maria"

  Scenario: Returning user is recognized by sub
    Given a user exists for sub "g-123"
    When the visitor signs in again as sub "g-123" with a new email
    Then no new user is created
    And the stored email is updated

  Scenario: Unverified email is refused
    When the visitor signs in with email_verified false
    Then the response is 403 and no user is created

  Scenario: Forged callback is rejected
    When /auth/callback is called with a state that does not match the cookie
    Then the response is 400 and no session cookie is set

  Scenario: No plaintext identity at rest
    Given a user signed in
    When the database files are read as raw bytes
    Then the email, name and sub do not appear
```

## 18. Out of scope

- Session expiry, logout, rate limiting, server timeouts on OIDC beyond the 10 s deadline (e03s02).
- Landing copy, legal pages, terms acceptance (e03s03).
- Default categories on sign-up (e04s01).
- Admin role (e08).

## 19. Verification Script (Step-by-Step)

1. Create a Google OAuth client (Web application) with redirect URI `http://localhost:8080/auth/callback`.
2. In your shell: `export MASTER_KEY=$(openssl rand -base64 32) GOOGLE_CLIENT_ID=<id> GOOGLE_CLIENT_SECRET=<secret> BASE_URL=http://localhost:8080 DB_PATH=/tmp/upareal.db` (keep the same `MASTER_KEY` between runs, or delete `/tmp/upareal.db*`).
3. `make run`, open http://localhost:8080 in Chrome or Firefox, click "Entrar com Google", choose your account. You land on "/" with "Olá, <your first name>".
4. Reload the page: still signed in.
5. `strings /tmp/upareal.db | grep -i "<your email>"` → no output.
6. Stop the app, start it again, sign in again: still one user (`sqlite3 /tmp/upareal.db "select count(*) from users_user"` → 1, if sqlite3 is installed).
7. `make preflight` → green.

## 20. Traceability

- Scope: S03, S04.
- Decisions: `specs/planning-context.yaml` → `domain.users`, `key_decisions` (sub, shell env, email_verified, go-oidc, CrossOriginProtection).
- Prior art: `specs/product/PRIOR_ART_LATEST.md`.
- Code and tests carry `// story: e03s01`.
