import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Contrast contract for the colour tokens in globals.css.
 *
 * axe only measures colours that happen to be on screen in the E2E run; this
 * test checks the tokens themselves, in both themes, so a palette change that
 * breaks a pairing no page currently renders still fails. It parses the
 * `:root` and `.dark` blocks, converts each oklch value to WCAG relative
 * luminance and asserts the pairs listed in docs/frontend-design-system.md.
 */

type Oklch = readonly [L: number, C: number, h: number];
type Tokens = ReadonlyMap<string, Oklch>;

const MIN_RATIO = 4.5;
const STATES = ["destructive", "success", "warning", "info"] as const;

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom into a served `/@fs/...` URL, which fileURLToPath then
// refuses (see kernel-parity.test.ts).
const css = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), "globals.css"),
  "utf8",
);

/** Extracts every `--name: oklch(L C h [/ alpha])` declaration of one block. */
function parseBlock(selector: string): Tokens {
  const escaped = selector.replace(".", "\\.");
  const block = new RegExp(`(?:^|\\n)${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  if (block?.[1] === undefined) {
    throw new Error(`globals.css has no top-level ${selector} block`);
  }
  const tokens = new Map<string, Oklch>();
  const decl =
    /--([a-z0-9-]+)\s*:\s*oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*(?:\/[^)]*)?\)/g;
  for (const match of block[1].matchAll(decl)) {
    const [, name, L, C, h] = match;
    if (
      name === undefined ||
      L === undefined ||
      C === undefined ||
      h === undefined
    ) {
      continue;
    }
    tokens.set(name, [Number(L), Number(C), Number(h)]);
  }
  return tokens;
}

/**
 * oklch -> oklab -> LMS -> linear sRGB, per Björn Ottosson's reference
 * matrices. Channels are clamped to the sRGB gamut the way a browser would
 * before painting, so an out-of-gamut token is measured as it is shown.
 */
function oklchToLinearSrgb([L, C, hDeg]: Oklch): [number, number, number] {
  const h = (hDeg * Math.PI) / 180;
  const a = C * Math.cos(h);
  const b = C * Math.sin(h);

  const cube = (x: number): number => x * x * x;
  const l = cube(L + 0.3963377774 * a + 0.2158037573 * b);
  const m = cube(L - 0.1055613458 * a - 0.0638541728 * b);
  const s = cube(L - 0.0894841775 * a - 1.291485548 * b);

  const clamp = (v: number): number => Math.min(1, Math.max(0, v));
  return [
    clamp(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s),
    clamp(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s),
    clamp(-0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s),
  ];
}

/** WCAG 2.x relative luminance from linear sRGB. */
function relativeLuminance(colour: Oklch): number {
  const [r, g, b] = oklchToLinearSrgb(colour);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** WCAG 2.x contrast ratio, always >= 1. */
function contrastRatio(a: Oklch, b: Oklch): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const [lighter, darker] = la > lb ? [la, lb] : [lb, la];
  return (lighter + 0.05) / (darker + 0.05);
}

/** The token pairs that must reach MIN_RATIO: [text or foreground, surface]. */
function contractPairs(): readonly (readonly [string, string])[] {
  const pairs: (readonly [string, string])[] = [
    ["foreground", "background"],
    ["foreground", "card"],
    ["muted-foreground", "background"],
    ["primary-foreground", "primary"],
  ];
  for (const state of STATES) {
    // Used as text, border and soft fill on the page surfaces ...
    pairs.push([state, "background"], [state, "card"]);
    // ... and as a solid fill under its own foreground.
    pairs.push([`${state}-foreground`, state]);
  }
  return pairs;
}

function token(tokens: Tokens, theme: string, name: string): Oklch {
  const value = tokens.get(name);
  if (value === undefined) {
    throw new Error(`${theme} theme defines no --${name} token`);
  }
  return value;
}

describe("colour token contrast contract", () => {
  it("converts oklch to WCAG luminance", () => {
    // Sanity anchors for the conversion itself: black, white and mid grey.
    expect(relativeLuminance([0, 0, 0])).toBeCloseTo(0, 6);
    expect(relativeLuminance([1, 0, 0])).toBeCloseTo(1, 4);
    // oklch(0.5 0 0) is sRGB #636363 ≈ luminance 0.122.
    expect(relativeLuminance([0.5, 0, 0])).toBeCloseTo(0.122, 2);
    expect(contrastRatio([0, 0, 0], [1, 0, 0])).toBeCloseTo(21, 1);
  });

  for (const [theme, selector] of [
    ["light", ":root"],
    ["dark", ".dark"],
  ] as const) {
    describe(`${theme} theme (${selector})`, () => {
      const tokens = parseBlock(selector);

      it("defines every semantic state and its foreground", () => {
        for (const state of STATES) {
          expect(tokens.has(state), `--${state} is missing`).toBe(true);
          expect(
            tokens.has(`${state}-foreground`),
            `--${state}-foreground is missing`,
          ).toBe(true);
        }
      });

      for (const [fg, bg] of contractPairs()) {
        it(`--${fg} on --${bg} reaches ${String(MIN_RATIO)}:1`, () => {
          const ratio = contrastRatio(
            token(tokens, theme, fg),
            token(tokens, theme, bg),
          );
          expect(
            ratio,
            `${theme}: --${fg} on --${bg} measures ${ratio.toFixed(2)}:1, below ${String(MIN_RATIO)}:1`,
          ).toBeGreaterThanOrEqual(MIN_RATIO);
        });
      }
    });
  }
});
