# Contributing to upareal

Thank you for helping. This page gets you from a fresh clone to a good first pull request. The rules in [`CONVENTIONS.md`](CONVENTIONS.md) are the source of truth; this page is the short path through them.

## 1. Set up

You need Go 1.26+, golangci-lint, Docker with Compose (optional) and a Google OAuth client for sign-in. templ, sqlc and goose are Go `tool` dependencies, so you don't install them globally.

```bash
git clone https://github.com/upareal/upareal.git
cd upareal
openssl rand -base64 32    # once: your local MASTER_KEY; keep it, or the local database becomes unreadable
export BASE_URL=http://localhost:8080 DB_PATH=./upareal.db \
  GOOGLE_CLIENT_ID=... GOOGLE_CLIENT_SECRET=... MASTER_KEY=...
make run    # http://localhost:8080
```

In the Google Cloud console, add `http://localhost:8080/auth/callback` as an authorized redirect URI of the OAuth client.

See the [README](README.md#configuration) for every environment variable. Never commit secrets, `.env` files, SQLite files or real bank statements.

## 2. Commands

| Command | What it does |
|---------|--------------|
| `make gen` | Generates templ and sqlc code |
| `make run` | Generates code and runs the app |
| `make test` | `go test -race ./...` in a far time zone, to catch São Paulo date bugs |
| `make lint` | `golangci-lint run` |
| `make preflight` | gen, stale generated-code check, lint, test, build. Must be green before you open a PR |

## 3. Workflow

1. Create a branch from `main`: `git switch -c feat/short-name`. Never commit to `main`.
2. Write the test first, watch it fail, then write the least code that makes it pass.
3. Commit with [Conventional Commits](https://www.conventionalcommits.org/) and a scope: `users`, `ledger`, `budget`, `planning`, `web`, `internal`, `deploy`, `deps`. Example: `fix(planning): carry overspending into next month`.
4. Commit generated code (`*_templ.go`, `*/internal/store/`) with the change that caused it. Never edit it by hand.
5. Run `make preflight`. Open the PR only when it is green.

Larger work is planned in [`specs/`](specs/README.md) first: a story spec and a tasks file under `specs/epics/`. Ask in an issue before you start a big change.

## 4. Where things go

| I want to… | Do this |
|------------|---------|
| Add a page | A `.templ` file and a handler in `web/`; register the route in `web/web.go`. UI text in pt-BR. |
| Add a query | Write SQL in `<module>/queries/`, run `make gen`, call it from the module's `Service`. Filter by `user_id`. |
| Change a table | Add a new goose file in `<module>/migrations/`. Never edit a committed migration. |
| Use data from another module | Call that module's exported `Service` methods. Never read its tables. |
| Store personal text | Encrypt it with `internal/vault`. |

[`docs/architecture.md`](docs/architecture.md) explains the modules and why these rules exist.

## 5. Rules you must not break

- Money is `int64` centavos. Never `float`.
- Every ledger transaction's legs sum to zero.
- Every query filters by `user_id`.
- Times are stored in UTC. Days and months are computed in `America/Sao_Paulo`.
- Never log names, emails, descriptions or amounts. Log `user_id` only.
- No CGO. The build stays `CGO_ENABLED=0`.
- UI text is pt-BR. Code, comments, logs, commits and docs are English.

## 6. Tests

- Tests sit next to the code in `_test.go` files and use a real SQLite database in `t.TempDir()`. Don't mock the database.
- Every new function gets a test. Every bug fix gets a regression test.
- Every module keeps a test that proves user A can't read user B's data.
- Tag tests and code with the story they belong to: `// story: e15s01`.
