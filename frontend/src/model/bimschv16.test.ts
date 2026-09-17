import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  BIMSCHV16_AREA_CATEGORIES,
  BIMSCHV16_AREA_CATEGORY_LABELS_DE,
  bimschv16AreaCategoryLabel,
  PROP_BIMSCHV16_AREA_CATEGORY,
} from "./bimschv16";

/**
 * The map writes the Gebietskategorie; `assessment/bimschv16` reads it. The
 * two are written in different languages, and nothing between them is typed:
 * a category the editor spells differently is written, saved, and then
 * reported as "missing 16. BImSchV area category property" — the receiver is
 * dropped into `ExportEnvelope.Skipped` and never assessed at all.
 *
 * So these tests read assessment.go as the source of truth, the way
 * `wasm/parking-vocabulary.test.ts` reads the RLS-19 extractor. The thresholds
 * differ by 12 dB across the four rows, so a value that lands on the wrong one
 * is not cosmetic either.
 */

// The path is passed in rather than written as a literal at the `new URL`
// call: Vite rewrites `new URL("<literal>", import.meta.url)` into an asset
// URL, which under jsdom comes back as `http://localhost/…` and never opens.
// `schall03.test.ts` and `wasm/parking-vocabulary.test.ts` take the same
// indirection for the same reason.
const repoFile = (relative: string): string =>
  readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");

const assessmentGo = (): string =>
  repoFile("../../../backend/internal/assessment/bimschv16/assessment.go");

/**
 * Constant name and value of one Go string enum, in declaration order.
 *
 * `AreaCategory` is a plain `const` block rather than a table literal, so the
 * `goVocabulary()` scraper in `schall03.test.ts` does not match it.
 */
function goStringEnum(
  source: string,
  typeName: string,
): Array<{ name: string; value: string }> {
  const pattern = new RegExp(
    String.raw`^\s*(\w+)\s+${typeName}\s*=\s*"([^"]*)"`,
    "gm",
  );
  return [...source.matchAll(pattern)].map((match) => ({
    name: match[1] ?? "",
    value: match[2] ?? "",
  }));
}

/** `case <Const>: return "<label>"` pairs inside one Go function. */
function goLabelSwitch(
  source: string,
  functionName: string,
): Array<{ name: string; label: string }> {
  const body = new RegExp(String.raw`func ${functionName}\([\s\S]*?\n\}`).exec(
    source,
  );
  if (body === null) return [];
  return [...body[0].matchAll(/case\s+(\w+):\s*\n\s*return\s+"([^"]*)"/g)].map(
    (match) => ({ name: match[1] ?? "", label: match[2] ?? "" }),
  );
}

describe("16. BImSchV Gebietskategorien", () => {
  it("finds the Go constants", () => {
    // Guards the regexes: a rename in assessment.go must fail here rather than
    // turn the comparisons below into vacuous truths.
    const constants = goStringEnum(assessmentGo(), "AreaCategory");

    expect(constants).toHaveLength(4);
    expect(constants.map((entry) => entry.value)).toContain("residential");
    expect(constants.map((entry) => entry.name)).toContain("AreaResidential");
  });

  it("offers exactly the categories ThresholdsForCategory knows", () => {
    expect([...BIMSCHV16_AREA_CATEGORIES]).toEqual(
      goStringEnum(assessmentGo(), "AreaCategory").map((entry) => entry.value),
    );
  });

  it("finds the Go labels", () => {
    const labels = goLabelSwitch(assessmentGo(), "AreaCategoryLabelDE");

    expect(labels).toHaveLength(4);
    expect(labels.map((entry) => entry.label)).toContain("Gewerbegebiet");
  });

  it("labels every category the way AreaCategoryLabelDE does", () => {
    const source = assessmentGo();
    const valueByName = new Map(
      goStringEnum(source, "AreaCategory").map((entry) => [
        entry.name,
        entry.value,
      ]),
    );
    const goLabels = Object.fromEntries(
      goLabelSwitch(source, "AreaCategoryLabelDE").map((entry) => [
        valueByName.get(entry.name) ?? entry.name,
        entry.label,
      ]),
    );

    expect({ ...BIMSCHV16_AREA_CATEGORY_LABELS_DE }).toEqual(goLabels);
  });

  it("falls back to the raw value for a category it does not know", () => {
    // `AreaCategoryLabelDE` returns `string(category)` in its default branch.
    // A model imported under a spelling this build predates is shown as it is
    // stored rather than as an empty row.
    expect(bimschv16AreaCategoryLabel("sondergebiet")).toBe("sondergebiet");
  });
});

describe("16. BImSchV property key", () => {
  it("is the first spelling categoryFromFeature tries", () => {
    // Order is load-bearing, not incidental: the editor writes this key alone,
    // so a receiver that still carries an older spelling must resolve to the
    // value the map last wrote rather than to the stale one.
    const block =
      /func categoryFromFeature\([\s\S]*?\[\]string\{([\s\S]*?)\}/.exec(
        assessmentGo(),
      );
    expect(block, "categoryFromFeature key list not found").not.toBeNull();

    const keys = [...(block?.[1] ?? "").matchAll(/"([^"]*)"/g)].map(
      (match) => match[1] ?? "",
    );

    expect(keys.length).toBeGreaterThan(0);
    expect(keys[0]).toBe(PROP_BIMSCHV16_AREA_CATEGORY);
  });
});
