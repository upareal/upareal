# Conventions

Rules for every AI agent and human working on upareal. Decisions and rationale live in `specs/planning-context.yaml`.

## Conventional Commits & Semantic Versioning

All commits MUST follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
Versioning MUST follow [Semantic Versioning 2.0.0](https://semver.org/).

### Commit Message Format

`<type>(<scope>): <description>` (space after colon is MANDATORY)

```
feat(ledger): reject transactions whose legs don't sum to zero
fix(planning): carry overspending into next month
chore(deps): bump templ to v0.3
```

**Scopes:** `users`, `ledger`, `planning`, `web`, `internal`, `deploy`, `deps`.

### Types & Version Bumps

- `feat`: Minor (x.Y.z) — new feature
- `fix`: Patch (x.y.Z) — bug fix
- `perf`: Patch (x.y.Z) — performance improvement
- `docs`, `chore`, `style`, `refactor`, `test`, `ci`, `build`: no bump (unless breaking)
- `BREAKING CHANGE:` footer (or `!` after type): Major (X.y.z)

## GitHub & Git Operations

- DO NOT work directly on `main`. Start every task on a feature branch via `kickoff-branch`.
- Integrate mode comes from `specs/state.yaml` → `workflow_mode` (`solo-git` by default).
- NEVER run `git push`, force push, `reset --hard`, `clean` or `branch -D` without explicit permission.
- NEVER add `Co-authored-by` or any footer that attributes code to an AI agent.
- Use `gh` for GitHub operations. NEVER call the GitHub REST API directly.
- NEVER create GitHub issues from automated workflows. Write local files in `specs/` instead.

## Agent Workflow Mandates

- **No Direct Coding:** Route feature work through bigpowers skills. DO NOT write feature code without a plan in `specs/`.
- Start with `survey-context` when context is missing.
- Use `plan-work` to write tasks with `verify:` commands before feature code.
- Use `develop-tdd` or `execute-plan` to implement the plan.
- Use `investigate-bug` before writing a bug fix.
- **Verification Mandate:** End every story with a manual verification script. Wait for user confirmation before declaring done.
- **Traceability Mandate:** Every story MUST have at least one `// story: eNNsNN` tag in its code or tests.

## Always Green / Shift Left

**Always Green** means Preflight and CI are green before any forward work.

**Shift Left (1-10-100):** A defect costs 1× in development, 10× in integration, 100× in production.

**Preflight** is `make preflight`. It MUST pass before kickoff, develop or verify phases advance.

**CI green:** when a PR exists, `gh pr checks` MUST pass before merge.

## Discovered Defects

A reproducible gate failure found during unrelated work is a discovered defect.

**fix-or-log ladder (mandatory):**

1. **quick-fix** — trivial, data-only or single-file fixes within guardrails.
2. **fix-bug** — when quick-fix aborts, or the failure needs investigation (`specs/bugs/BUG-*.md` + TDD).
3. **Log** — only when reproduction is blocked. Write a BUG spec and stop forward work until triaged.

Ship discovered fixes in the same PR, in separate commits. NEVER narrate a failure and continue.

### Banned dismissive phrases

| Banned phrase | Required behavior instead |
|---------------|---------------------------|
| Pre-existing / pre-existing issues | Run fix-or-log. If truly unrelated, prove it with a passing repro after revert. |
| unrelated to this session | Same. Session boundaries do not waive green gates. |
| not introduced by my changes | Bisect or fix anyway. Solo default owns the whole tree. |
| out of scope (ignoring a red gate) | Invoke quick-fix or fix-bug. Scope never overrides Always Green. |

## specs/ — All Planning Output Goes Here

| Question | File |
|----------|------|
| What did we decide, and why? | `specs/planning-context.yaml` |
| What is in or out of scope? | `specs/product/SCOPE_LATEST.yaml` |
| North star | `specs/product/VISION_LATEST.yaml` |
| Glossary | `specs/product/GLOSSARY_LATEST.yaml` |
| What ships in this release, in what order? | `specs/release-plan.yaml` |
| How to implement an epic or story? | `specs/epics/eNN-*/` |
| Story status (sole source of truth) | `specs/execution-status.yaml` |
| Where are we in the session? | `specs/state.yaml` |
| Stack and architecture | `specs/tech-architecture/tech-stack.md` |
| Architectural decisions | `specs/adr/ADR-*.md` |
| Bugs | `specs/bugs/BUG-*.md` + `specs/bugs/registry.yaml` |
| Verification evidence | `specs/verifications/` |

DO NOT put story status in `release-plan.yaml`. Git tags are the version authority. DO NOT hand-track versions in specs.

## Architecture Rules

- `users`, `ledger` and `planning` are public packages at the repo root. They MUST stay splittable into separate repos.
- Modules MUST NOT contain HTML. `web/` renders all pages and HTMX fragments with templ.
- A module MUST NOT import another module's unexported internals or query another module's tables.
- Each module owns `migrations/` (goose SQL, embedded), `queries/` (sqlc) and its generated code.
- Each module uses its own goose version table `goose_<module>` and table prefix `<module>_`.
- Foreign keys exist ONLY inside a module. Cross-module integrity is enforced in Go.
- `cmd/upareal` runs migrations at startup in order: users, ledger, planning.
- `internal/` holds code shared within this repo only: config, database helpers, crypto.

## Data Rules

- Money MUST be `int64` centavos. NEVER use `float` for money.
- Every ledger transaction's legs MUST sum to zero. Reject unbalanced writes.
- Store all times in UTC. Compute day and month membership in `America/Sao_Paulo`.
- Store date-only import lines at 00:00 `America/Sao_Paulo`.
- IDs MUST be UUIDv7 (`uuid.NewV7()`) stored as `TEXT`.
- Every query MUST filter by `user_id`.
- SQLite connections MUST set `PRAGMA foreign_keys = ON` in the DSN.
- NEVER edit a committed migration. Write a new migration instead.

## Privacy & Security Rules

- Encrypt sensitive text with AES-256-GCM through the `internal/` crypto helper ONLY.
- Sensitive text covers emails, names, account and category names, descriptions, history and import payloads.
- Amounts and dates stay plaintext.
- Use HMAC blind indexes for lookups on encrypted values.
- NEVER log amounts, descriptions or emails. Log `user_id` only.
- Read config from environment variables. Read secrets from SSM when `*_SSM_PARAM` is set, else from env.
- NEVER commit secrets, `.env` files, SQLite database files or real bank statements.
- Admins MUST NOT access other users' financial data.

## Code Style

- Format with `gofmt`. Follow the `golang-*` skills in `.agents/skills/`.
- Prefer the standard library. Add a dependency only when it clearly earns its place.
- Keep functions short and focused. Split a function that does more than one thing.
- Keep files under 300 lines. Split by responsibility.
- Names MUST be specific. Avoid `utils`, `helpers`, `common`, `Manager`.
- Names MUST describe side effects, e.g. `deleteUserSessions`, not `processUser`.
- Return errors. Wrap them with `%w` and context. NEVER panic for expected failures.
- Error messages MUST include the offending value and the expected shape.
- Use early returns over nested ifs.
- Accept interfaces, return structs. Define an interface only when two implementations exist.
- Pass dependencies through constructors. DO NOT use package-level mutable state.
- Delete dead code. DO NOT comment it out.
- UI text MUST be pt-BR. Code, comments, logs, commits and docs MUST be English.

## Comments

- Write WHY, not WHAT.
- Keep existing comments on refactor.
- Exported identifiers MUST have doc comments.
- Link an ADR or commit when a line exists because of a specific decision or bug.
- NEVER commit commented-out code.

## Generated Code

- `*_templ.go` and sqlc output MUST be committed.
- NEVER edit generated code by hand. Change the source and run `make gen`.
- Preflight fails when generated code is stale.

## Tests (F.I.R.S.T)

- Run all tests with `go test -race ./...`.
- Place tests next to the code in `_test.go` files.
- Every new function gets a test. Every bug fix gets a regression test.
- Use real SQLite in `t.TempDir()`. DO NOT mock the database.
- Fakes for external I/O MUST be named types, e.g. a fake OIDC `httptest.Server`.
- Tests MUST NOT use real secrets. Generate a random master key per run.
- Every module MUST have a test proving user A cannot read user B's data.
- Tests are Fast, Independent, Repeatable, Self-validating and Timely.
- NEVER skip a test without a note explaining what is unresolved.
- Cover boundary values: empty, zero, maximum, minimum and off-by-one.
- Test through exported APIs only.

## Logging

- Use `log/slog` with the JSON handler on stdout.
- Request logs contain method, path, status, duration and `user_id` only.

## Defensive Code

Implement defensive code ONLY for the categories listed here.

| Category | Status | Where |
|----------|--------|-------|
| Timeout | Apply | `http.Server` timeouts; `context` deadlines on outbound calls and long DB work |
| Rate limit | Apply | Per user and IP on login callback, import and export (`golang.org/x/time/rate`) |
| Retry | Apply (limited) | Reading SSM secrets at startup only, 3 attempts with backoff. NEVER retry user writes. |
| Circuit breaker | Deferred | Not in v1. Revisit later. |
| Graceful degradation | Deferred | Not in v1. Revisit later. |
