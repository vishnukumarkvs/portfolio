# Design

One column, white paper, hairline rules. No cards, no shadows, no gradients.

## Type

Two typefaces, both self-hosted as woff2 subsets in `assets/fonts/`:

- **Instrument Serif** for names and titles — the editorial voice
- **Instrument Sans** for everything else

Self-hosting avoids a Google Fonts request and a third-party origin, which
means the CSP does not need loosening and there is no external dependency at
runtime. The woff2 files are loaded with `font-display: swap` and preloaded for
the two faces used above the fold.

## Colour

Light and dark are the same layout with a different set of custom properties on
`[data-theme]`. Every colour is a custom property at the top of
`assets/style.css`; nothing below that section hard-codes a colour value.

The theme is applied by an inline script in `<head>` before first paint, so
the page never flashes the wrong theme. That inline script is the only reason
the CSP allows `'unsafe-inline'` for scripts.

## Width

Two variables, not one, because they solve different problems:

| Variable   | Value  | What it controls                        |
| ---------- | ------ | --------------------------------------- |
| `--shell`  | `72rem`| Page width                              |
| `--read`   | `38rem`| Maximum line length for prose           |

An earlier version had a single width variable, which made the page either too
wide to read comfortably or too narrow to use the space on a large display.
Splitting them lets the page be wide while the prose stays at a readable
measure.

Above `60rem` of viewport width, non-hero sections become a two-column grid: a
sticky section label in the left gutter, content on the right.

`--shell` at the top of `assets/style.css` is the single knob for the overall
page width.

## Weight

The page is about 6 KB of HTML. With the stylesheet, the vendored htmx and the
two fonts, roughly 25 KB over the wire gzipped. There is no image on the page
at all.

## Printing

`@media print` drops the topbar, the back link, the htmx container, and the
Close control, then sets the page in black on white at 11pt. Sections avoid
breaking across pages, and links are printed without decoration. A printed
fragment page is readable on its own.

## Progressive enhancement

The site is fully usable with JavaScript disabled — see
[architecture.md](architecture.md#htmx-and-why-it-is-optional). There is no
hamburger menu because there are only three nav links. No contact form and no
phone number; the Articles, GitHub and LinkedIn links in the header are the way
in.

## Accessibility notes

- The theme toggle is a real `<button>` with an `aria-label`.
- `prefers-color-scheme` is respected when no explicit choice is stored.
- Semantic landmarks throughout: one `<main>`, `<nav>`, `<footer>`.
- Anchor links work without JavaScript and the print stylesheet keeps them
  legible.
