# e12s03 — Logo and favicon

**type:** feat · **risk:** P3 · **context:** ui · **bcps:** 1 · **scope:** S22

## 1. Business narrative

The user supplied `logo.png` (1254×1254, 1.1 MB). The app shows the logo on the home page, and the browser tab, bookmarks and the iPhone home screen show it as the icon.

## 5. Main flow

- **Source:** `assets/logo.png` is the original. It's kept in the repo but not embedded in the binary.
- **Derived files** live in `web/static/` and are generated from the source:
  - transparent margins are cropped to the tile;
  - the image is shrunk with an area-averaging filter in premultiplied alpha;
  - the outputs are `logo.png` (192 px), `apple-touch-icon.png` (180 px) and `favicon.ico` (PNG entries at 16, 32 and 48 px), about 55 KB in total.
  - **The page logo is the symbol only.** The cream tile (#F7F3EB) is nearly the page's areia (#F4F0E6), so on the page it looked like a washed-out box. For `logo.png`, pixels within colour distance 24 of the tile become transparent and pixels beyond 60 stay opaque, with the foreground colour recovered in between. Pixels that aren't opaque in the source (the tile rim) are dropped. The result is cropped to the symbol. The icons keep the tile, which reads well on any browser or OS background.
- **Layout:** every page links `rel="icon"` → `/favicon.ico` and `rel="apple-touch-icon"` → `/static/apple-touch-icon.png`.
- **Root favicon:** `GET /favicon.ico` serves the embedded icon, because browsers request that path on their own (it used to return 404).
- **Home page:** the 56 px logo sits next to the "upareal" title. App pages are unchanged, so there's no extra header.

## 15. Implementation steps

1. Add the static files, the `/favicon.ico` route, the head links and the home logo. A test checks the icon (ICO magic bytes), the link tags and that the logo is served. → verify: `go test -race ./web/... -run TestBrand`
2. Run `make preflight`.

## 18. Out of scope

- A web app manifest (PWA), dark-mode icons, and committing the throwaway generator (regenerate the files the same way if the logo changes).

## 19. Verification Script (Step-by-Step)

1. Run `make run` and open http://localhost:8080. The logo is next to "upareal", and the browser tab shows the icon.
2. Open http://localhost:8080/favicon.ico. The icon appears.
3. Run `make preflight`. It ends green.
