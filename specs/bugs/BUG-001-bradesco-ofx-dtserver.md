# BUG-001 — Bradesco OFX rejected: zeroed DTSERVER

**severity:** high (a supported bank's statements cannot be imported) · **found:** 2026-10-08 by the user · **story ref:** e07s04

## Symptom

Importing a Bradesco OFX shows "Nada foi importado… Não foi possível ler o OFX…", and the logs say nothing.

## Root cause

1. Bradesco's OFX 1.02 (SGML, CHARSET 1252) sends `<DTSERVER>00000000000000` in the sign-on header. `ofxgo.ParseResponse` refuses the zero date ("Couldn't parse date") and rejects the **whole file**. All 112 transactions in the user's file are valid; DTSERVER is never used by upareal.
2. `web/import.go` replaced the parser error with a generic message, and `importPost` did not log import failures.
3. `cmd/upareal` built the JSON logger but never called `slog.SetDefault`, so handler logs (`slog.InfoContext` …) went to the plain-text default logger on stderr, not to the JSON log.

Diagnosed locally on the user's file (read-only; only header keys, tag counts and the parser error were printed).

## Fix plan (TDD)

| Task | Test first | Fix |
|------|-----------|-----|
| t1 | Fixture `bradesco_dtserver_zero.ofx` (zeroed DTSERVER, CHECKNUM, `[-03:EST]` dates) parses; a zeroed DTPOSTED is still rejected | Before ofxgo, replace a zeroed `<DTSERVER>` with a placeholder (that tag only) |
| t2 | Unreadable OFX shows "Detalhe técnico: …" on the page; the failure is logged as JSON `import failed` with user_id, account_id, format, kind, rejected_lines and no message text; `run()` sets the JSON logger as default | `importer.Parse` also returns the detected format; web shows the detail and logs; `slog.SetDefault(logger)` |
| t3 | — | `make preflight` |

## Verification

The user re-imports `/tmp/Bradesco_08102026_200005.ofx`: 112 lines reach Revisão. Re-importing gives "0 importadas, 112 já existentes".
