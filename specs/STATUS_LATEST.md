# upareal — Consolidated status

**Date:** 2026-10-07 · **Release:** v1 (unversioned) · **e11-production: postponed**

## Built

The app runs locally (`make run`) and in Docker (`docker compose up --build`). All 30 stories of e01–e10 have tests. Every story except e10s02 has a user verification record in `specs/verifications/`. e10s02 is waiting for its UAT.

| Epic | What works |
|------|------------|
| e01 foundation | One Go binary (`CGO_ENABLED=0`), SQLite, per-module goose migrations, health check, Docker image, CI, `make preflight` |
| e02 data protection | Per-user data keys wrapped by a master key; AES-256-GCM sealed text; HMAC blind indexes |
| e03 sign-in | Google OIDC; server-side sessions (30 days idle, 90 days absolute); log out of all devices; public pages; terms acceptance |
| e04 ledger accounts | System accounts; accounts with opening balances; organize, archive and delete accounts |
| e05 budget | Default categories; organize groups and categories; monthly amounts copied from the previous month |
| e06 transactions | Simple, split and transfer entries; edit and delete with encrypted history; search |
| e07 imports | Generic CSV and OFX 1.x/2.x (Windows-1252), deduplicated per account; review queue; categorize; transfer matching |
| e08 planning | Account→category mapping; month view (spent, available, A distribuir); carry-over; card payment envelopes; off-budget accounts; archive, merge and restore categories |
| e09 admin | `ADMIN_EMAILS` admins list, disable, enable and delete users, without seeing financial data |
| e10 data rights | Export my data (JSON, all modules); delete my account (typed confirmation) — **e10s02 awaiting UAT** |

**Size:** 286 test functions; `go test -race ./...` across 12 packages; golangci-lint reports 0 issues.

## Model (as built)

- **Ledger:** L1–L7. Legs sum to zero, amounts are `int64` centavos, and balances are always derived.
- **Budget:** B1–B4.
- **Planning:** P1–P4 and X1–X2. Everything is computed and nothing is stored.
- **Identity:** `A distribuir + Σ Disponível + Σ Para pagar = Dinheiro` (the balances of on-budget, non-card accounts). Tests guard it every month, including with cards and off-budget accounts.
- **Decisions changed while building:**
  - Overspending carries over in its category (e08s03).
  - A card's payment envelope equals the card's debt (e08s04).
  - Only money that crosses the budget boundary counts (e08s05).
  - Import format is detected by content; the per-account bank setting is deferred until e07s05.

## Not built

| Item | Status | Needs |
|------|--------|-------|
| e07s05 bank-specific parsers | Blocked | The list of banks and their export formats |
| e11 production (HTTPS/Caddy, SSM secrets, Litestream backups, EC2) | **Postponed** by the user | A domain, an AWS account, and the decision on backup crypto-shredding (below) |

## Open items

1. **Legal text.** Replace the provisional `/privacidade` and `/termos` text, then bump `web.CurrentTermsVersion`.
2. **Refunds.** The ledger can't record a credit on an expense account yet. Planning already nets refunds once they exist.
3. **UX.** After an import or a match, link to the register month where the lines landed.
4. **Backups and deletion (e11).** The wrapped per-user keys live in the backed-up SQLite file, so backups taken before a deletion stay readable until retention expires. Fixing this needs keys stored outside the backup, or short retention.
5. **Rate limiter.** It keys on `RemoteAddr`. Behind Caddy (e11) it must trust only the forwarded client IP.
6. **Row-level AAD binding** for sealed fields (deferred).
7. **Open question.** Should asset and liability accounts be mappable to categories? (v1 maps only income, expense and off-budget accounts.)

## Where things live

- **Decisions:** `specs/planning-context.yaml`.
- **Story status:** `specs/execution-status.yaml`.
- **Session state:** `specs/state.yaml`.
- **Specs and tasks:** `specs/epics/eNN-*/`.
- **Verifications:** `specs/verifications/`.
