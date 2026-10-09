# Architecture

upareal is one Go binary with server-rendered HTML. This page explains how the code is split and why. Decisions and their reasons live in [`specs/planning-context.yaml`](../specs/planning-context.yaml); the rules live in [`CONVENTIONS.md`](../CONVENTIONS.md).

## Stack

Go 1.26, [templ](https://templ.guide) + [HTMX](https://htmx.org), SQLite through `modernc.org/sqlite` (pure Go, no CGO), [goose](https://github.com/pressly/goose) migrations and [sqlc](https://sqlc.dev) queries.

## Modules

```
cmd/upareal ──► web ──► planning ──► ledger
                 │          └──────► budget
                 └──► users, ledger, budget
all modules ──► internal (config, db, money, vault)
```

| Package | Owns |
|---------|------|
| `users` | Google sign-in, sessions, terms acceptance, admin, data export and account deletion |
| `ledger` | Money movement: accounts, transactions and their legs, imports (`ledger/importer`), counter-account suggestions |
| `budget` | Intent: groups, categories and the amount for each category per month |
| `planning` | Joins ledger and budget: account→category mapping, envelopes, carry-over, credit card envelopes |
| `web` | Every HTML page and HTMX fragment. The only package that knows about HTTP and HTML |
| `internal/` | Code shared inside this repo: `config` (env vars), `db` (SQLite setup), `money` (centavos ↔ "R$ 1.234,56"), `vault` (encryption) |

`cmd/upareal/main.go` reads the config, opens the database, runs each module's migrations (users, ledger, budget, planning), builds each module's `Service` and hands them to `web`.

### Module boundaries

The modules are public packages so each one could move to its own repository later. To keep that possible:

- Each module owns its schema: `migrations/` (embedded goose SQL, version table `goose_<module>`), `queries/` (sqlc input) and `internal/store/` (sqlc output). Its tables use the `<module>_` prefix.
- No foreign keys between modules. Modules refer to each other by ID, and Go code keeps them consistent. For example, deleting a category drops its mappings in planning.
- A module never reads another module's tables. It calls the other module's exported `Service` methods.
- Modules contain no HTML.

## Domain model

**Ledger.** Double-entry bookkeeping with signed amounts. A transaction has two or more legs, each with an account and a non-zero amount, and the legs sum to zero. Account types are asset, liability, income, expense and equity. A balance is the sum of the legs; it is never stored.

**Budget.** Envelope budgeting. Categories sit in one level of groups. Each category has at most one amount per month, and a new month copies the previous amounts.

**Planning.** Each income or expense account maps to at most one category. Planning computes spent, available, carry-over, "A distribuir" (to be assigned) and the credit card envelopes on every request; none of these are stored. Unmapped accounts show as "Sem categoria", so no money is left out.

The numbered invariants (L1–L7, B1–B4, P1–P4, X1–X2) are in [`specs/planning-context.yaml`](../specs/planning-context.yaml) under `domain.invariants`. Every change must keep them true.

## Cross-cutting rules

- **Money**: `int64` centavos everywhere. `internal/money` parses and formats pt-BR amounts.
- **Time**: stored in UTC. Day and month membership is computed in `America/Sao_Paulo`. Date-only import lines are stored at 00:00 São Paulo time.
- **IDs**: UUIDv7 stored as `TEXT`.
- **Users**: every row belongs to one user and every query filters by `user_id`. The only exception is the public statistics queries, which return counts and sums only.
- **Privacy**: names, descriptions and other personal text are encrypted with AES-256-GCM through `internal/vault`, using a per-user data key wrapped by `MASTER_KEY`. Amounts and dates stay plaintext so SQL can sum them. Lookups on encrypted values use HMAC blind indexes.

## A request, end to end

1. `web` routes the request (`web/web.go`, Go 1.22+ `http.ServeMux` patterns) and resolves the signed-in user from the session cookie through `users`.
2. The handler calls module services with that `user_id`, for example `planning.Month(ctx, userID, month)`.
3. The module runs sqlc queries inside its own tables, decrypts what it returns and applies the domain rules.
4. The handler renders a templ component. HTMX requests get the same page, and the browser swaps the part it needs.
