# e01s01 — Walking skeleton: pt-BR home page and health check

**type:** feat · **risk:** P1 · **context:** infra · **bcps:** 5 · **scope:** S01, S22

## 1. Business narrative

As the developer of upareal, I run one command and get a working app: a mobile-friendly pt-BR home page and a health check backed by the real database. Every later story adds to this path instead of building infrastructure in isolation.

## 2. Actors

- **Developer** — runs `make run`, `make preflight`.
- **Visitor** — opens the home page in a desktop or phone browser.

## 3. Preconditions

- Go 1.26.5, make and golangci-lint 2.14.0 are installed (`specs/state.yaml` → `environment`).
- gcc is installed. `go test -race` needs cgo. The shipped binary stays `CGO_ENABLED=0`.

## 4. Trigger

The developer runs `make run` with `DB_PATH` set.

## 5. Main flow

1. `make run` runs `make gen` (templ) and starts `go run ./cmd/upareal`.
2. The app reads config from the environment. `DB_PATH` is required.
3. The app opens SQLite at `DB_PATH` with WAL, `busy_timeout` and `foreign_keys` on.
4. The app listens on `:8080`.
5. `GET /` returns the pt-BR home page with the base layout.
6. `GET /healthz` runs `SELECT 1` and returns `200 ok`.
7. `GET /static/htmx.min.js` returns the vendored HTMX file.
8. Each request writes one JSON log line.

## 6. Alternative flows

- **6a. `DB_PATH` missing** — the app exits with status 1 and an error naming `DB_PATH`.
- **6b. Database unreachable** — `/healthz` returns `503`.
- **6c. SIGTERM / Ctrl+C** — the app stops accepting connections, finishes in-flight requests (10 s limit), closes the database and exits 0.

## 7. Business rules

- UI text is pt-BR (`<html lang="pt-BR">`). Code, logs and comments are English.
- Request logs contain method, path, status and duration only. No query strings, no bodies.

## 8. Data

No tables. The database file is created on first open.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `GET /` | `200`, `text/html`, contains `lang="pt-BR"` and the viewport meta tag |
| `GET /healthz` | `200 ok` when `SELECT 1` succeeds, else `503` |
| `GET /static/*` | Embedded static files (`embed.FS`) |
| Env `DB_PATH` | Required. Path to the SQLite file. |
| `make gen` / `run` / `build` / `preflight` | See `AGENTS.md` → Commands |

## 10. Non-functional

- `http.Server` sets `ReadHeaderTimeout` 5 s, `ReadTimeout` 15 s, `WriteTimeout` 30 s, `IdleTimeout` 60 s.
- The binary builds with `CGO_ENABLED=0`.
- The binary embeds `time/tzdata`.

## 11. Security & privacy

- Static files are served from `embed.FS` only, so a path cannot escape the embedded tree.
- HTMX is vendored, so the page loads no third-party scripts.
- Logs carry no user data.

## 12. Dependencies (slopcheck)

| Package | Version | Tag | Evidence |
|---------|---------|-----|----------|
| `github.com/a-h/templ` (tool + runtime) | v0.3.1070 | [OK] | Active; `templ.Handler(c Component, ...)` in `handler.go:118` |
| `modernc.org/sqlite` | v1.60.1 | [OK] | Pure Go; driver name `"sqlite"` (`sqlite.go:50`); DSN `_pragma=name(value)`, `busy_timeout` applied first (`sqlite.go:443`) |
| htmx.org (vendored JS) | 2.0.11 | [OK] | npm `latest`; copied to `web/static/htmx.min.js` |

`sqlc` is not added here. It is pinned in e03s01 together with the first query. `go tool sqlc` builds with `CGO_ENABLED=0` (checked: `v1.31.1`).

## 13. Requirements

#### ADDED: R1 — One-command local run
`make run` serves the app on `http://localhost:8080`.

#### ADDED: R2 — Health check
`GET /healthz` reports database reachability.

#### ADDED: R3 — Mobile-friendly pt-BR base layout
Every page uses a base layout with `lang="pt-BR"`, a viewport meta tag, responsive CSS and the vendored HTMX script.

#### ADDED: R4 — Fail-fast config
A missing required env var stops startup with a clear message.

#### ADDED: R5 — Operability
JSON request logs and graceful shutdown.

#### ADDED: R6 — Preflight
`make preflight` runs gen, the stale generated-code check, lint, tests and build.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `internal/config.Load(getenv func(string) string)` | Tests inject env without touching the process environment. |
| `internal/db.Open(path)` | One place owns the DSN pragmas that every module and Litestream rely on. |
| `web.NewHandler(db *sql.DB) http.Handler` | `httptest` drives the full router without a network listener. |
| `cmd/upareal.run(ctx, getenv, stdout) error` | `main` stays 3 lines; tests cancel `ctx` to prove graceful shutdown. |

No interfaces are introduced. `*sql.DB` is passed directly.

## 15. Implementation steps

1. Add `templ` with `go get -tool github.com/a-h/templ/cmd/templ@v0.3.1070`, plus a `Makefile` with `gen`, `run`, `build` and `preflight`. `preflight` fails when `git status --porcelain -- '*_templ.go'` is non-empty after gen. Add a minimal `.golangci.yml` (`version: "2"`). → verify: `go tool templ version && make -n preflight`
2. Add `internal/config.Load` with required `DB_PATH`. → verify: `go test -race ./internal/config/... -run TestLoad`
3. Add `internal/db.Open` with the WAL, `busy_timeout(5000)` and `foreign_keys(1)` pragmas. Add `/healthz` with `200`/`503`. → verify: `go test -race ./internal/db/... ./web/... -run 'TestOpen|TestHealthz'`
4. Add the base layout and home page as templ components, plus vendored `htmx.min.js` 2.0.11 and CSS served from `embed.FS`. → verify: `go test -race ./web/... -run 'TestHomePage|TestStatic'`
5. Add slog JSON request logging middleware, server timeouts, and `run(ctx, ...)` with graceful shutdown. → verify: `go test -race ./cmd/upareal/... ./web/... -run 'TestGracefulShutdown|TestRequestLog'`
6. Run the full gate. → verify: `make preflight`

## 16. Risks

- **Race tests need cgo and gcc.** Detect: `make preflight` fails on a machine without gcc. CI uses `ubuntu-latest`, which has gcc.
- **Stale generated templ code.** Detect: the Preflight porcelain check.
- **The HTMX file is modified or truncated.** Detect: `TestStatic` asserts its size is above 40 KB and it contains `htmx`.

## 17. Acceptance criteria

```gherkin
Feature: Walking skeleton

  Scenario: Home page renders in pt-BR and fits a phone
    Given the app is running with DB_PATH set
    When a visitor requests GET /
    Then the response is 200 text/html
    And the page has lang="pt-BR" and a viewport meta tag
    And the page loads /static/htmx.min.js

  Scenario: Health check reports the database
    Given the app is running with DB_PATH set
    When GET /healthz is requested
    Then the response is 200 with body "ok"

  Scenario: Health check fails when the database is closed
    Given the database handle is closed
    When GET /healthz is requested
    Then the response is 503

  Scenario: Missing config stops startup
    Given DB_PATH is not set
    When the app starts
    Then it exits non-zero with an error naming DB_PATH

  Scenario: Graceful shutdown
    Given a request is in flight
    When the app receives SIGTERM
    Then the in-flight request completes
    And the app exits 0
```

## 18. Out of scope

- Docker, compose and CI (e01s02).
- Migrations and sqlc (e03s01, with the first table).
- Sign-in, landing page copy and legal pages (e03).
- Rate limiting (e03s02).

## 19. Verification Script (Step-by-Step)

1. Run `DB_PATH=/tmp/upareal.db make run`.
2. Open http://localhost:8080 on desktop. The pt-BR home page appears.
3. Open the same URL with browser dev tools in phone mode (e.g. 375 px wide). The layout fits with no horizontal scroll.
4. Run `curl -i http://localhost:8080/healthz`. Expect `200` and `ok`.
5. Check the terminal. Each request printed one JSON line with `method`, `path`, `status` and `duration`.
6. Press Ctrl+C. The app logs a shutdown line and exits with status 0 (`echo $?`).
7. Run `make run` without `DB_PATH`. The app exits non-zero with an error naming `DB_PATH`.
8. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S01 (local run), S22 (phone-sized screens).
- Decisions: `specs/planning-context.yaml` → `architecture`, `operations`, `tech_stack`.
- Prior art: `specs/product/PRIOR_ART_LATEST.md` (templ, modernc.org/sqlite, HTMX, `time/tzdata`).
- Code and tests carry `// story: e01s01`.
