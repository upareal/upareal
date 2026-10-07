# upareal — AI Agents

> **Multi-agent context** — This file is the canonical project context for pi, Codex CLI, OpenCode and other AGENTS.md-native tools. Claude Code reads it via the `CLAUDE.md` symlink.

Read CONVENTIONS.md before any GitHub or git operation.

<!-- BEGIN bigpowers:context-routing -->
## Context Routing

| Glob | Load |
|------|------|
| `**/*.go` | `.agents/skills/golang-*` matching the task |
| `specs/**` | `specs/planning-context.yaml` first |
<!-- END bigpowers:context-routing -->

<!-- BEGIN bigpowers:learned-preferences -->
## Learned User Preferences

- NEVER commit to `main`. ALWAYS create a feature branch first, then commit there.

## Workspace Facts

- A bigpowers pi hook blocks `git commit` on `main`. Run `git switch -c <branch>` and `git commit` as separate commands.
- The user merges branches into `main` locally.
<!-- END bigpowers:learned-preferences -->

<!-- BEGIN bigpowers:project -->
## Project

upareal is a private personal budget web app for Brazilians, with double-entry bookkeeping and envelope budgeting.
Stack: Go 1.26, templ + HTMX, SQLite (`modernc.org/sqlite`), goose, sqlc, Docker.

## Commands

| Action | Command |
|--------|---------|
| Generate | `make gen` |
| Run | `make run` (http://localhost:8080) |
| Run (Docker) | `docker compose up --build` |
| Test | `go test -race ./...` |
| Lint | `golangci-lint run` |
| Build | `make build` |
| Preflight | `make preflight` (gen, generated-code diff check, lint, test, build) |
| CI | `gh pr checks` (when a PR is open) |

## Test

`go test -race ./...`

## Lint

`golangci-lint run`

## Build

`make build`

## Architecture

One Go binary. `cmd/upareal` wires config, database, modules and `web/`.
Public modules `users`, `ledger`, `budget` and `planning` hold domain logic and own their schemas.
`ledger` holds accounts and transactions. `budget` holds categories and monthly amounts.
`planning` joins ledger and budget.
Domain invariants (L1-L7, B1-B4, P1-P4, X1-X2) live in `specs/planning-context.yaml` → `domain.invariants`.
`web/` (templ + HTMX) imports the modules and renders all HTML. `internal/` holds shared code.
Full decisions live in `specs/planning-context.yaml`.

## Conventions

- Commits MUST follow Conventional Commits with a module scope, e.g. `feat(ledger): ...`.
- Modules MUST NOT contain HTML or import another module's internals.
- Each module MUST own its `migrations/`, `queries/`, `goose_<module>` table and `<module>_` table prefix.
- Money MUST be `int64` centavos.
- Store all times in UTC. Compute day and month in `America/Sao_Paulo`.
- IDs MUST be UUIDv7 stored as `TEXT`.
- Every query MUST filter by `user_id`.
- Every ledger write MUST verify that the legs sum to zero.
- Write sensitive text ONLY through the encryption helper in `internal/`.
- The UI MUST use pt-BR. Code, comments, logs, commits and README MUST use English.
- Commit generated code (`*_templ.go`, sqlc output).
- Follow the `golang-*` skills in `.agents/skills/`. Prefer the standard library.

## Never

- Never dismiss reproducible gate failures as pre-existing or out of scope.
- Never proceed on red Preflight or red CI. Invoke quick-fix or fix-bug first.
- Never commit secrets, `.env` files, SQLite database files or real bank statements.
- Never log or store plaintext names, emails, descriptions or amounts outside their designated columns.
- Never use `float` for money.
- Never add a foreign key between modules. Never query another module's tables.
- Never edit generated code or a committed migration. Write a new migration instead.
- Never weaken the zero-sum check or `user_id` scoping, including in tests.
- Never add CGO dependencies. The build MUST stay `CGO_ENABLED=0`.
- Never run `git push`, force push, `reset --hard` or `clean` without explicit permission.

## Agent Rules

- **Workflow Mandate:** Use bigpowers skills (e.g. `plan-work`, `develop-tdd`) for structured work.
- **Always Green:** Preflight and CI MUST be green before forward work.
- Read specs/ and CONVENTIONS.md before writing code.
- Write the minimum code that solves the stated problem.
- Run tests after every change. Show evidence before declaring done.
- All planning output goes in specs/.

## Token Economy — Minimal Footprint

1. **Check existing dependencies first.** DO inspect current dependencies before adding a package or writing your own code.
2. **Prefer mature, maintained libraries.** DO NOT rewrite a capability a maintained library provides without a documented reason.
3. **Copy validated patterns.** DO study how established products solve the same problem before inventing a new approach.
4. **Keep the simplest working implementation.** DO write the least code that satisfies the requirement. NEVER add preventive abstraction.
<!-- END bigpowers:project -->
