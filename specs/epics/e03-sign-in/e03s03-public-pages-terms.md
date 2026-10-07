# e03s03 — Public pages and terms acceptance

**type:** feat · **risk:** P1 · **context:** domain · **bcps:** 3 · **scope:** S03, S05

## 1. Business narrative

Anyone can read the privacy policy and the terms of service before signing in. Google also requires public URLs for both before it will publish the OAuth app. A signed-in user must accept the current version of the terms before using upareal. When the terms change, they're asked again. The acceptance is recorded as LGPD consent evidence.

## 2. Actors

- **Visitor**: reads `/privacidade` and `/termos`.
- **User**: accepts the current terms.

## 3. Preconditions

- e03s01 and e03s02 are done.
- The legal text is an open item. This story ships clearly marked **provisional** text ("Texto provisório") that you replace later.

## 4. Trigger

A visitor opens a public page, or a signed-in user opens any app page.

## 5. Main flow

1. `/privacidade` and `/termos` render for everyone, signed in or not, in pt-BR.
2. Every page footer links to both.
3. After sign-in, if the user's accepted `terms_version` is not the current one, any app page redirects to `/termos/aceitar`.
4. `/termos/aceitar` shows links to both documents and a required checkbox "Li e aceito os Termos de Uso e a Política de Privacidade", plus a button "Aceitar e continuar".
5. `POST /termos/aceitar` with the box checked stores `terms_version` and `terms_accepted_at` (UTC) and redirects to `/`.

## 6. Alternative flows

- **6a. Box not checked:** the page shows again with status 400 and the message "Marque a caixa para continuar."
- **6b. Terms version bumped:** a user who accepted an older version is asked again.
- **6c. Exempt routes:** `/healthz`, `/static/*`, `/auth/*` (including logout), `/privacidade`, `/termos` and `/termos/aceitar` never redirect.
- **6d. Signed out:** `/termos/aceitar` redirects to `/`.

## 7. Business rules

- `CurrentTermsVersion` is a constant in `web` (`"2026-10-07"`). Bump it whenever the text of either document changes.
- Acceptance stores the version and the time. The latest acceptance replaces the previous one. A full history is out of scope.

## 8. Data

Migration `users/migrations/00002_terms.sql` adds two nullable columns to `users_user`:
- `terms_version TEXT`
- `terms_accepted_at TEXT`

These aren't personal text, so they stay plaintext.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `GET /privacidade`, `GET /termos` | 200 for everyone |
| `GET /termos/aceitar` | Signed in: the form. Signed out: 303 to `/` |
| `POST /termos/aceitar` | `aceito=on` → stores the acceptance, 303 to `/`. Missing → 400 with the form again |
| `users.User.TermsVersion` | The version the user accepted, or `""` |
| `users.Service.AcceptTerms(ctx, userID, version)` | Records the acceptance |

## 10. Non-functional

The terms check reuses the session lookup the page already performs, so it adds no extra query.

## 11. Security & privacy

- The acceptance POST is protected by `CrossOriginProtection` (e03s01).
- `terms_accepted_at` is consent evidence (LGPD).

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Public legal pages
`/privacidade` and `/termos` are public and linked from every page.

#### ADDED: R2: Terms acceptance
A signed-in user must accept the current terms version before using the app.

#### MODIFIED: Home access
**Before:** a signed-in user saw the home page right after sign-in.
**After:** a signed-in user who hasn't accepted the current terms is redirected to `/termos/aceitar` first.

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `web.requireTerms` | One gate in front of every app page, so new pages can't forget the check. |

## 15. Implementation steps

1. Public pages and footer links. → verify: `go test -race ./web/... -run TestPublicPages`
2. Migration and `AcceptTerms`; `User.TermsVersion` is loaded. → verify: `go test -race ./users/... -run TestAcceptTerms`
3. The gate redirects until the current version is accepted, with the accept form and POST, and a version bump asks again. → verify: `go test -race ./web/... -run TestTermsAcceptance`
4. Full gate. → verify: `make preflight`

## 16. Risks

- **Provisional legal text goes live by mistake.** Mitigation: the text says "Texto provisório" in a banner, and the open item stays in `specs/state.yaml` until replaced. e10's deploy runbook checks for it.

## 17. Acceptance criteria

```gherkin
Feature: Terms acceptance

  Scenario: Public pages
    Given a visitor who is not signed in
    When they open /privacidade or /termos
    Then the page renders with status 200

  Scenario: First use asks for acceptance
    Given a user who just signed in for the first time
    When they open /
    Then they are redirected to /termos/aceitar

  Scenario: Accepting unlocks the app
    Given the acceptance form
    When the user checks the box and submits
    Then they land on / and see the greeting

  Scenario: New terms version asks again
    Given a user who accepted version "2026-01-01"
    When the current version is "2026-10-07"
    Then they are redirected to /termos/aceitar
```

## 18. Out of scope

- Final legal text (an open item for the user).
- An acceptance history table.
- Withdrawing consent (the same as deleting the account, e09s02).

## 19. Verification Script (Step-by-Step)

1. Delete `/tmp/upareal.db*`, export the variables and run `make run`.
2. Signed out, open http://localhost:8080/privacidade and /termos. Both render, and the footer links work.
3. Sign in. You're sent to "Aceite os termos".
4. Click "Aceitar e continuar" without checking the box. You see "Marque a caixa para continuar."
5. Check the box and submit. You land on "Olá, …".
6. Reload. There's no further prompt.
7. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S03, S05.
- Decisions: `specs/planning-context.yaml` → `domain.users` (terms_version + accepted_at), `privacy_security` (static pages for Google OAuth publishing).
- Code and tests carry `// story: e03s03`.
