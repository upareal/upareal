# upareal

![License](https://img.shields.io/badge/License-MIT-yellow.svg)
![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg)
![Status](https://img.shields.io/badge/status-planning-lightgrey.svg)

> Private personal budgeting for Brazilians: double-entry bookkeeping plus envelope budgeting, in a responsive web app.

upareal tracks every real you own or owe with double-entry bookkeeping. Every transaction has two or more legs that sum to zero, written as signed amounts instead of debits and credits. Monthly budgets use the envelope method: every real of income is assigned to a category, leftovers carry over, and overspending reduces next month.

It is built for individuals in Brazil: BRL only, pt-BR interface, Google sign-in, and bank statement imports. Financial data is private by design. Names and descriptions are encrypted per user, so a leaked database or backup reveals no readable personal data.

> **Status: planning.** No application code exists yet. The commands below describe the target workflow. Decisions live in [`specs/planning-context.yaml`](specs/planning-context.yaml).

## Features

- **Ledger** — typed accounts (asset, liability, income, expense, equity), split transactions and transfers. Unbalanced transactions are never saved.
- **Envelope budgeting** — assign income to categories, carry leftovers, YNAB-style credit card payment envelopes.
- **Bank imports** — per-bank parsers plus a generic CSV format. A review queue matches transfers and skips duplicates.
- **Privacy** — field-level AES-256-GCM encryption with per-user keys, plus self-service data export and account deletion (LGPD).
- **Mobile-friendly** — server-rendered HTML with templ and HTMX. No native app needed.
- **Admin** — admins from `ADMIN_EMAILS` can list, disable and delete users. They cannot see financial data.

## Architecture

| Path | Role |
|------|------|
| `cmd/upareal/` | `main`: wires config, database, modules and web |
| `users/` | Google sign-in, sessions, admin, export and deletion |
| `ledger/` | Accounts, transactions, imports |
| `planning/` | Monthly envelope budgets |
| `web/` | templ + HTMX pages and fragments; the only layer that renders HTML |
| `internal/` | Shared code for this repo: config, database helpers, crypto |

Each module owns its schema (goose migrations + sqlc queries) and has no foreign keys to other modules. Any module can later move to its own service repo.

## Prerequisites

- **Go** 1.26+
- **Docker** with Compose
- **golangci-lint** (for `make preflight`)
- A **Google OAuth client** (web application) for sign-in

templ, sqlc and goose are pinned as Go `tool` dependencies. No global install is needed.

## Development

```bash
git clone https://github.com/upareal/upareal.git
cd upareal
make run                     # generate code, run on http://localhost:8080
docker compose up --build    # or run in a container
```

## Configuration

All configuration comes from environment variables. Secrets are read from AWS SSM Parameter Store when the `*_SSM_PARAM` variable is set, otherwise from the plain variable. Never put secrets in a committed file.

| Variable | Secret | Description |
|----------|--------|-------------|
| `BASE_URL` | No | Public URL, e.g. `http://localhost:8080` |
| `DB_PATH` | No | SQLite file path |
| `ADMIN_EMAILS` | No | Comma-separated admin emails |
| `GOOGLE_CLIENT_ID` | No | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` / `GOOGLE_CLIENT_SECRET_SSM_PARAM` | Yes | Google OAuth client secret |
| `MASTER_KEY` / `MASTER_KEY_SSM_PARAM` | Yes | 32-byte key that wraps per-user data keys |

> **Back up the master key offline.** Losing it makes all user data, including backups, unreadable.

## Tests

```bash
go test -race ./...    # tests, using real SQLite in temp dirs and no real secrets
golangci-lint run      # lint
make preflight         # gen + stale-code check + lint + test + build; must pass before merge
```

## Deployment

One Docker Compose stack on a single EC2 instance:

- **Caddy** terminates TLS with automatic Let's Encrypt certificates.
- **The app** runs under `litestream replicate -exec`, streaming SQLite changes to S3. It restores automatically when the volume is empty.
- **The EC2 instance role** grants SSM read and S3 write access. No access keys sit on disk.

SQLite means exactly one instance. Do not scale horizontally.

## Contributing

1. Read [`AGENTS.md`](AGENTS.md) and [`CONVENTIONS.md`](CONVENTIONS.md).
2. Create a feature branch. Never commit directly to `main`.
3. Use [Conventional Commits](https://www.conventionalcommits.org/) with a module scope, e.g. `feat(ledger): add split transactions`.
4. Run `make preflight` and keep it green.

## Links

- Repository: https://github.com/upareal/upareal
- Issues: https://github.com/upareal/upareal/issues
- Planning decisions: [`specs/planning-context.yaml`](specs/planning-context.yaml)

## License

MIT. See [LICENSE](LICENSE).
