# e12s02 — A little color

**type:** feat · **risk:** P3 · **context:** ui · **bcps:** 1 · **scope:** S22

## 1. Business narrative

The interface is plain grey on white. The user asked for some color from a soft palette, without bloating the UI. CSS only.

## 2–4. Actors, preconditions, trigger

The user, on any page.

## 5. Main flow

- **Palette:** six CSS variables in `app.css` `:root`: verde folha #6C9A74, verde claro #DDEADE, azul oceano claro #8FB7D8, amarelo manga suave #E9C77B, areia #F4F0E6, terracota suave #D08C73.
- **Page:** areia background.
- **Headings and buttons:** h1 underlined in verde folha. Buttons are verde claro with a verde folha border, and turn verde folha on hover.
- **Group cards:** category and budget groups are light cards with a verde claro header. Table headers are verde claro.
- **Highlights:**
  - totals (A distribuir) have a manga band;
  - hints have an azul oceano band;
  - errors have a terracota band;
  - focus rings are azul oceano.
- **Text:** pastels are never used as text color, because they're below 4.5:1 on areia. Links use a darker verde (#3F6B48), and errors and negative amounts use a darker terracota (#9A4A32). Every text pair is at least 4.5:1 (WCAG AA).

## 6–14. Alternative flows, rules, data, interfaces, design

There's no template or Go change. Every color comes from one `:root` block, so the palette is changed in one place.

## 15. Implementation steps

1. Update `app.css`. → verify: `make preflight`

## 16. Risks

- **Taste.** Checked by the user in the browser.

## 17. Acceptance criteria

The pages show the palette, and all text stays readable (each pair ≥ 4.5:1).

## 18. Out of scope

- Dark mode, icons and new components.

## 19. Verification Script (Step-by-Step)

1. Run `make run` and check Início, Orçamento, Contas, Lançamentos, Revisão, Mapa and Categorias on a desktop and a phone-sized window.
2. Trigger an error (for example, amount `abc` in Orçamento). It shows in a terracota band.
3. Run `make preflight`. It ends green.

## 20. Traceability

- Scope: S22.
- Code carries `/* story: e12s02 */`.
