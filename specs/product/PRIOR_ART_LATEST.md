# Prior Art — upareal v1

Written by `research-first` (2026-10-07). Checks every dependency proposed in
[`specs/planning-context.yaml`](../planning-context.yaml) against the repo, the Go stdlib (go1.26.5) and the Go module proxy.
Versions are the `@latest` reported by `proxy.golang.org` on that date.

## Prior Art

| Candidate | Source | Verdict | Evidence / notes |
|-----------|--------|---------|------------------|
| Repo code | this repo | — | Empty: only `go.mod`. Nothing to reuse. |
| **templ** | `github.com/a-h/templ` v0.3.1070 (2026-10-04) | adopt | Active. Pin with a `tool` directive in `go.mod`. |
| **HTMX** | htmx.org (JS) | adopt | Vendor the minified file under `web/static/`. No CDN, so CSP stays `self`. |
| **sqlc** | `github.com/sqlc-dev/sqlc` v1.31.1 (2026-04-22) | adopt | Has a `sqlite` engine. `internal/migrations/migrations.go` strips `-- +goose Down`, so goose files work directly as the schema. |
| **goose** | `github.com/pressly/goose/v3` v3.28.0 (2026-09-02) | adopt | `goose.NewProvider(goose.DialectSQLite3, db, fsys, goose.WithTableName("goose_ledger"))` gives one version table per module. |
| **SQLite driver** | `modernc.org/sqlite` v1.60.1 (2026-09-29) | adopt | Pure Go, works with `CGO_ENABLED=0`. Set `journal_mode=WAL`, `busy_timeout` and `foreign_keys=ON` in the DSN. Litestream requires WAL. |
| **UUIDv7** | `github.com/google/uuid` v1.6.0 | adopt | `uuid.NewV7()` exists (`version7.go:23`). The stdlib has no `uuid` package in go1.26. |
| **OFX parsing** | `github.com/aclindsa/ofxgo` v0.1.3 (2021-10-18) | **extend** | `ofxgo.ParseResponse(r)` handles SGML (1.x) and XML (2.x). **Gap:** `response.go:305` sets `CharsetReader` to pass input through unchanged, so `CHARSET:1252` text is not converted. Transcode to UTF-8 with `golang.org/x/text/encoding/charmap` before parsing. Last release was in 2021, but OFX is a frozen spec. Pin the version and cover it with fixture tests. |
| **Charset conversion** | `golang.org/x/text` v0.42.0 | adopt | `charmap.Windows1252` and `charmap.ISO8859_1`. |
| **CSV** | stdlib `encoding/csv` | adopt | Set `Comma = ';'`. No dependency needed. |
| **Google sign-in (OIDC)** | `github.com/coreos/go-oidc/v3` v3.21.0 (2026-09-01) + `golang.org/x/oauth2` v0.37.0 | adopt | `oidc.NewProvider(ctx, "https://accounts.google.com")` plus `provider.Verifier(...)` validates ID tokens. **New dependency, not in the spec.** Writing JWT/JWKS verification by hand is a security risk. |
| **Sessions** | stdlib `crypto/rand`, `crypto/sha256`, `net/http` cookies | build | Small: random ID, hashed row, cookie. Session libraries add stores and interfaces we don't need. |
| **CSRF** | stdlib `net/http.CrossOriginProtection` (Go 1.25+) | adopt | Checks `Sec-Fetch-Site`/`Origin` on unsafe methods. Replaces tokens in forms and HTMX headers, so no dependency is needed. **Simplifies the spec.** |
| **Field encryption** | stdlib `crypto/aes` + `crypto/cipher` (GCM), `crypto/hmac`, `crypto/hkdf` | build | Envelope scheme on the stdlib. Derive separate encryption and HMAC keys from the master key with `crypto/hkdf` (stdlib since Go 1.24). Google Tink also does envelope encryption but adds heavy dependencies for about 100 lines of code. |
| **Rate limiting** | `golang.org/x/time/rate` v0.16.0 | adopt | `rate.NewLimiter` per user/IP key in a map. |
| **Logging** | stdlib `log/slog` | adopt | JSON handler. |
| **Time zone data** | stdlib `time/tzdata` | adopt | Blank import so `America/Sao_Paulo` loads in distroless. |
| **Secrets (e10)** | `github.com/aws/aws-sdk-go-v2/service/ssm` v1.79.1 | adopt (e10 only) | `GetParameter` with `WithDecryption: true`. Load only when `*_SSM_PARAM` is set. |
| **Backups (e10)** | `github.com/benbjohnson/litestream` v0.5.17 (2026-08-31) | adopt (e10 only) | `replicate -exec` (`replicate.go:68`) and `restore -if-replica-exists` both exist in v0.5. Ships as a binary in the image, not as a Go import. |
| **TLS (e10)** | Caddy (container image) | adopt (e10 only) | Automatic Let's Encrypt certificates. |
| **Lint** | golangci-lint | adopt | **Not installed locally.** `setup-environment` must install it, or Preflight fails. |

## Changes to the spec

1. **Add** `coreos/go-oidc/v3` + `x/oauth2` for Google sign-in. Verifying OIDC by hand is a security risk.
2. **Use** `net/http.CrossOriginProtection` for CSRF instead of tokens.
3. **Extend** ofxgo: transcode the charset before `ParseResponse`.
4. **Derive** encryption and HMAC keys from the master key with `crypto/hkdf`. One secret still covers both.
5. **Install** golangci-lint before e01 Preflight.

## Verify

→ verify: `grep -q 'Prior Art' specs/product/PRIOR_ART_LATEST.md && grep -q PRIOR_ART specs/product/SCOPE_LATEST.yaml`
