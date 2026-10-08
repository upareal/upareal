# e12s01 — Save without losing your place

**type:** feat · **risk:** P2 · **context:** ui · **bcps:** 2 · **scope:** S22

## 1. Business narrative

On **Orçamento**, **Revisão** and **Mapa**, the user repeats one edit down a long list. Today every save reloads the page and jumps back to the top. After this story the page updates in place: the user stays where they were, and every total still refreshes.

## 2. Actors

- **User**: on `/orcamento`, `/revisao` and `/planejamento/mapa`.

## 3. Preconditions

- htmx 2.0.11 is vendored and loaded (e06s02).

## 4. Trigger

The user saves an amount, assigns a queued line, or maps an account.

## 5. Main flow

1. Those three row forms gain `hx-post` (the same URL as `action`), `hx-target="main"`, `hx-select="main"` and `hx-swap="outerHTML"`.
2. The server is unchanged. It still answers with a 303 redirect followed by the full page. htmx follows the redirect and swaps in the new `<main>`. A plain `hx-post` doesn't scroll, so the user keeps their place.
3. A `<meta name="htmx-config">` in the layout makes htmx also swap 4xx and 5xx responses, so validation messages ("Valor inválido") and error pages still appear.

## 6. Alternative flows

- **6a. No JavaScript:** the plain `method`/`action` still works exactly as it does today.

## 7. Business rules

- No new endpoints and no fragment templates. The full page stays the only rendering.

## 8. Data

None.

## 9. Interfaces

| Interface | Contract |
|-----------|----------|
| Layout | `<meta name="htmx-config">` with `responseHandling` swapping `[23]..` and `[45]..` |
| Row forms on the three pages | `hx-post` + `hx-target/hx-select="main"` + `hx-swap="outerHTML"` |

## 10. Non-functional

Same server cost. The browser skips re-parsing CSS and JS.

## 11. Security & privacy

- htmx sends same-origin XHRs, and `CrossOriginProtection` accepts them, exactly like the existing split form.

## 12. Dependencies (slopcheck)

None new.

## 13. Requirements

#### ADDED: R1: Row saves on Orçamento, Revisão and Mapa update in place without scrolling
#### ADDED: R2: Error responses are still shown

## 14. Design notes (Reason for Depth)

| Unit | Reason for Depth |
|------|------------------|
| `hx-select="main"` on the full page | Reuses the existing handlers and pages, so there's no fragment rendering to keep in sync. |

## 15. Implementation steps

1. Add the attributes on the three pages and the htmx config. A test checks the attributes and the config, and checks that an htmx-style POST (with the `HX-Request` header) still ends in a full page with `<main>`, including the 400 case. → verify: `go test -race ./web/... -run TestKeepPlace`
2. Full gate. → verify: `make preflight`

## 16. Risks

- **The browser behaviour can't be run in Go tests** (no JavaScript). This is covered by manual UAT.

## 17. Acceptance criteria

```gherkin
Feature: Keep my place

  Scenario: Saving an amount far down the page
    Given the user scrolled to "Investimentos" on Orçamento
    When they save its amount
    Then the page stays scrolled there and A distribuir is updated
```

## 18. Out of scope

- `hx-boost` everywhere, live search and inline editing (there's no current need for them).

## 19. Verification Script (Step-by-Step)

1. **Orçamento:** scroll down to a category near the bottom, change its amount and save. The page stays there. Scroll up and A distribuir has changed.
2. **Orçamento:** type an invalid amount (`abc`) and save. "Valor inválido" appears.
3. **Revisão:** import a CSV with several lines and quick-assign one in the middle. The page stays in place, and the line leaves the queue.
4. **Mapa:** map an account at the bottom. The page stays there, and the "sem categoria" counter drops.
5. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S22 (every screen works on a phone-sized browser; less scrolling on small screens).
- Code and tests carry `// story: e12s01`.
