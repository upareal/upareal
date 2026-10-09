# BUG-002 — Search test fails in CI between 21:00 and 24:00 São Paulo

**severity:** medium (red CI on main; app behaviour correct) · **found:** 2026-10-08 by CI on main@595e28d · **story ref:** e06s05

## Symptom

`TestSearchPage_DefaultsValidationAndNav` failed in CI at 00:15 UTC on 2026-10-09: "want the range to start 12 months ago (2025-10-09)".

## Root cause

- **The app is correct.** `searchGet` defaults the range to 12 months back from today in São Paulo (2025-10-08 at that moment).
- **The test computed "12 months ago" in the machine's local zone:** `time.Now()` without `.In(saoPaulo)`. On CI that's UTC, which is already the next day from 21:00 São Paulo time.
- **The gate couldn't catch it.** Locally `TZ` is São Paulo, so the mistake never shows. On CI (UTC) it only shows for 3 hours a day.

## Fix

| Task | Change |
|------|--------|
| t1 | `make test` runs with `TZ=Pacific/Kiritimati` (UTC+14). Its local date differs from São Paulo's for 17 hours a day, so a test that confuses "local today" with "São Paulo today" fails almost always, both locally and on CI. Running the whole suite in that zone found only this test. |
| t2 | The test computes the expected date in São Paulo. |
| t3 | `make preflight` passes. |
