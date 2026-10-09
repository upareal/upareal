# e15s01 — Public pt-BR help pages at /ajuda

**type:** feat · **risk:** P2 · **context:** ui · **bcps:** 2 · **scope:** — (new, user request 2026-10-09)

## 1. Business narrative

A new user doesn't know where to start: which accounts to create, how the envelope budget works, what Revisão is for. The help pages explain the app in plain pt-BR, so the user can learn it alone. This first version is short on purpose; it grows later.

## 2. Actors

- **Anyone**: signed in or not. The pages hold no user data.

## 3. Preconditions

None.

## 4. Trigger

The user clicks **Ajuda** in the footer of any page.

## 5. Main flow

1. `GET /ajuda` shows the index: a short intro and links to each page.
2. Pages, each one screen or two of text:
   - `/ajuda/primeiros-passos` — Primeiros passos: sign in, create accounts, set the month's budget.
   - `/ajuda/contas` — Contas e lançamentos: account types, opening balance, in/off budget, credit cards, new entry, split, transfer, edit.
   - `/ajuda/importar` — Importar extratos e Revisão: OFX/CSV, duplicates, Revisão, suggestions, transfers. Links to the existing `/ajuda/csv`.
   - `/ajuda/orcamento` — Orçamento: envelopes, A distribuir, Disponível, carry-over, overspending, cards, Mapa, Sem categoria.
   - `/ajuda/dados` — Seus dados e privacidade: encryption, export, AI export, account deletion.
3. Every page links back to `/ajuda`.
4. The footer of every page gets an **Ajuda** link.

## 6. Alternative flows

- **6a. Unknown page** (`/ajuda/xyz`): 404, like any unknown path.

## 7. Business rules

- Public, like `/privacidade`. No session needed.
- `/ajuda/csv` stays where it is (the import page links to it).

## 8. Data

None.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| `GET /ajuda`, `GET /ajuda/{page}` | Public; 200 with the page, 404 for an unknown page |
| Footer | New "Ajuda" link |

## 10–12. Non-functional, security, dependencies

Static templ pages. No user data, no new dependency.

## 13. Requirements

#### ADDED: R1: Public pt-BR help pages that explain how to use the app

## 14. Design notes

| Unit | Reason for Depth |
|------|------------------|
| `web/help.templ` | Help is presentation, so it lives in web. One templ component per page, served by `templ.Handler` like `/privacidade`. |

## 15. Implementation steps

1. Help index and pages, public, linked from each other. → verify: `go test -race ./web/... -run TestHelp`
2. Footer "Ajuda" link on every page. → verify: `go test -race ./web/... -run TestHelp`
3. Full gate. → verify: `make preflight`

## 17. Acceptance criteria

```gherkin
Feature: Help pages

  Scenario: Anyone opens help
    Given a visitor who is not signed in
    When they open /ajuda
    Then they see "Ajuda" and links to the five help pages
    And each page opens with status 200 and links back to /ajuda

  Scenario: Help from any page
    When any page renders
    Then its footer links to /ajuda
```

## 18. Out of scope

- Search, screenshots, videos, per-screen contextual help.
- Rendering Markdown files at runtime.

## 19. Verification Script (Step-by-Step)

1. Run `make run`. Without signing in, click **Ajuda** in the footer.
2. Open each of the five pages. Check that the text is clear pt-BR and matches the screens.
3. Sign in and check that the footer still shows **Ajuda**.
4. Run `make preflight`. It ends green.

## 20. Traceability

- Code and tests carry `// story: e15s01`.
