# Frontend design system

The tokens live in `frontend/src/styles/globals.css` (Tailwind v4, `@theme`).
This page says what each token is for and which rules the tests enforce.

Aesthetic: refined utilitarian — a clean engineering tool with subtle warmth.
Warm-slate neutrals, a muted teal primary, no decorative colour.

## Semantic colour tokens

| Token                                       | Use                                                                 |
| ------------------------------------------- | ------------------------------------------------------------------- |
| `background` / `foreground`                 | Page surface and body text                                          |
| `card`, `popover` (+ `-foreground`)         | Raised surfaces                                                     |
| `primary` (+ `-foreground`)                 | The one accent: primary actions, active states, focus ring (`ring`) |
| `secondary`, `muted`, `accent`              | Quiet fills; `muted-foreground` is secondary text                   |
| `destructive`, `success`, `warning`, `info` | Semantic states (see below)                                         |
| `border`, `input`                           | Hairlines and field outlines                                        |
| `chart-1` … `chart-5`                       | Noise-level map and chart series                                    |
| `sidebar-*`                                 | The workspace rail, mirrored from the tokens above                  |

The four state colours are used in two ways, and the contract below makes both
safe without per-page colour patches:

- **As text, border or soft fill on a page surface**, through opacity
  modifiers: `text-warning`, `border-warning/30`, `bg-warning/10`. A callout is
  `border border-warning/30 bg-warning/10 text-foreground` with a
  `text-warning` icon or title; the solid colour is never a background here.
- **As a solid fill** for badges and buttons, always paired with its own
  foreground: `bg-success text-success-foreground`. The `-foreground` tokens
  exist for this pairing only — never use them as text on a page surface.

## Contrast contract

Every pair below reaches **≥ 4.5:1** (WCAG AA for body text) in both the
light (`:root`) and the dark (`.dark`) token set:

- `foreground` on `background` and on `card`;
- `muted-foreground` on `background`;
- `primary-foreground` on `primary`;
- each of `destructive`, `success`, `warning`, `info` on `background` and on
  `card`;
- each `<state>-foreground` on its solid `<state>`.

`frontend/src/styles/tokens.test.ts` enforces this. It parses the two token
blocks, converts oklch to WCAG relative luminance itself (oklch → oklab → LMS →
linear sRGB, clamped to gamut) and fails naming the pair and the measured
ratio. It exists because axe, in the E2E suite, only sees colours that are on
screen on the routes it visits; the test sees every token. Change a token,
run `bun run test`, and keep the ratios — do not lower the threshold.

## Type scale

The Tailwind scale is cleared (`--text-*: initial`) and replaced. These are the
only text sizes in the app; `text-2xl` and larger produce no CSS.

| Class       | Size | Line height | Use                                          |
| ----------- | ---- | ----------- | -------------------------------------------- |
| `text-2xs`  | 11px | 16px        | Badges, counters, table meta                 |
| `text-xs`   | 12px | 16px        | Captions, helper text, timestamps            |
| `text-sm`   | 13px | 20px        | Body in dense UI: tables, sidebars, controls |
| `text-base` | 14px | 20px        | Body text, form fields, page content         |
| `text-lg`   | 16px | 24px        | Subheadings, dialog titles                   |
| `text-xl`   | 20px | 28px        | Page headings                                |

`<body>` itself is not sized by a token yet, so it inherits the browser's 16px:
page content sets `text-base` or `text-sm` explicitly rather than relying on
the inherited size.

Weights: 400 body, 500 labels and buttons, 600 headings, 700 sparingly (the
number in a stat, a key figure). Use `font-mono` for identifiers, hashes,
coordinates and dB values that align in columns.

## Radius

`--radius` is 0.5rem. Only three steps exist: `rounded-sm` (4px), `rounded-md`
(6px) and `rounded-lg` (8px). `rounded-lg` is the largest step for cards,
dialogs and panels; `rounded-xl` and larger produce no CSS. `rounded-full` is
for pills and avatars, `rounded-none` where a surface bleeds to an edge.

## Fonts

IBM Plex Sans (400, 500, 600, 700) and IBM Plex Mono (400, 500) are self-hosted
through `@fontsource/ibm-plex-sans` and `@fontsource/ibm-plex-mono`, imported at
the top of `globals.css`. Vite copies the woff2 files into `dist/assets`; the
app fetches nothing at runtime. This is an offline-first tool: do not add a
`<link>` to a font CDN. The stacks stay `--font-sans` and `--font-mono`.

## Reduced motion

`globals.css` honours `prefers-reduced-motion: reduce` globally by collapsing
every animation and transition to a single 0.01ms frame. Durations are
shortened rather than removed so `animationend`/`transitionend` handlers still
fire. Components need no per-case handling; do not add `motion-safe:` variants
to work around it.

## Dark mode

Both themes are the same token names with different values, and the `.dark`
class on `<html>` switches them (the ThemeProvider follows the OS by default).
Write `text-warning`, `bg-card`, `border-border` and let the token change. Never
patch a Tailwind palette colour per theme (`text-amber-900 dark:text-amber-200`):
it bypasses the contrast contract, and the dark run of the accessibility suite
(`chromium-dark` in `playwright.config.ts`) is where such patches fail.
