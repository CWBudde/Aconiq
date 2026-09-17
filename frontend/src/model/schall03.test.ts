import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  arrayPropertyLength,
  PROP_SCHALL03_BASE_HEIGHT_M,
  PROP_SCHALL03_BRIDGE_MITIGATION,
  PROP_SCHALL03_BRIDGE_TYPE,
  PROP_SCHALL03_CURVE_RADIUS_M,
  PROP_SCHALL03_FAHRBAHN,
  PROP_SCHALL03_IS_STATION,
  PROP_SCHALL03_OPERATIONS,
  PROP_SCHALL03_PARALLEL_EDGES,
  PROP_SCHALL03_PERMANENTLY_SLOW,
  PROP_SCHALL03_REFLECTING_WALL,
  PROP_SCHALL03_REFLECTIVE,
  PROP_SCHALL03_S_FAHRBAHN,
  PROP_SCHALL03_STRECKE_MAX_KPH,
  PROP_SCHALL03_SURFACE,
  PROP_SCHALL03_THICKNESS_M,
  PROP_SCHALL03_TRACK_FEATURES,
  PROP_SCHALL03_WALL_SURFACE,
  PROP_SCHALL03_WATER_BODY_FRACTION,
  SCHALL03_FAHRBAHN_TYPES,
  SCHALL03_S_FAHRBAHN_TYPES,
  SCHALL03_SURFACE_TYPES,
  SCHALL03_WALL_SURFACES,
} from "./schall03";

/**
 * The map editor writes the properties the Go extractor reads, so the two lists
 * have to be the same list. They are written in different languages, and a name
 * this side spells differently is not a type error anywhere: the property is
 * simply written, saved, and never read — a Fahrbahnart correction silently
 * absent from every level the run reports.
 *
 * The Go source is the truth here, the way `wasm/parking-vocabulary.test.ts`
 * treats `parking.go`. These tests read it rather than a copy of it.
 */

const repoFile = (relative: string): string =>
  readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");

const extractorGo = (): string =>
  repoFile(
    "../../../backend/internal/app/cli/run_extract_schall03_normative.go",
  );

const vocabularyGo = (): string =>
  repoFile("../../../backend/internal/standards/schall03/vocabulary.go");

/** Every `propSchall03… = "…"` constant in the extractor, in any order. */
function goPropertyNames(source: string): Set<string> {
  const names = new Set<string>();
  for (const match of source.matchAll(
    /^\s*prop\w+\s*=\s*"(schall03_[a-z0-9_]+)"/gm,
  )) {
    names.add(match[1] ?? "");
  }
  return names;
}

/** The `Name:` column of one `…Names = []struct{…}{…}` table, in order. */
function goVocabulary(source: string, table: string): string[] {
  const block = new RegExp(
    String.raw`var ${table} = \[\]struct \{[\s\S]*?\n\}\{([\s\S]*?)\n\}`,
  ).exec(source);
  if (!block?.[1]) throw new Error(`no ${table} table in vocabulary.go`);

  return [...block[1].matchAll(/\{"([^"]+)",/g)].map((match) => match[1] ?? "");
}

describe("Schall 03 property names", () => {
  it("finds the Go constants", () => {
    // Guards the regex: a rename in the extractor must fail here rather than
    // turn the comparison below into a vacuous truth.
    const names = goPropertyNames(extractorGo());
    expect(names.size).toBeGreaterThan(10);
    expect(names).toContain(PROP_SCHALL03_OPERATIONS);
  });

  it("spells every editable property the way the extractor reads it", () => {
    const names = goPropertyNames(extractorGo());

    for (const property of [
      PROP_SCHALL03_OPERATIONS,
      PROP_SCHALL03_STRECKE_MAX_KPH,
      PROP_SCHALL03_FAHRBAHN,
      PROP_SCHALL03_S_FAHRBAHN,
      PROP_SCHALL03_SURFACE,
      PROP_SCHALL03_BRIDGE_TYPE,
      PROP_SCHALL03_BRIDGE_MITIGATION,
      PROP_SCHALL03_CURVE_RADIUS_M,
      PROP_SCHALL03_IS_STATION,
      PROP_SCHALL03_PERMANENTLY_SLOW,
      PROP_SCHALL03_TRACK_FEATURES,
      PROP_SCHALL03_WATER_BODY_FRACTION,
      PROP_SCHALL03_REFLECTIVE,
      PROP_SCHALL03_BASE_HEIGHT_M,
      PROP_SCHALL03_THICKNESS_M,
      PROP_SCHALL03_PARALLEL_EDGES,
      PROP_SCHALL03_WALL_SURFACE,
      PROP_SCHALL03_REFLECTING_WALL,
    ]) {
      expect(names).toContain(property);
    }
  });
});

describe("Schall 03 vocabularies", () => {
  it("matches Tabelle 7 (Fahrbahnart Eisenbahn) exactly", () => {
    expect([...SCHALL03_FAHRBAHN_TYPES]).toEqual(
      goVocabulary(vocabularyGo(), "fahrbahnartNames"),
    );
  });

  it("matches Tabelle 15 (Fahrbahnart Straßenbahn) exactly", () => {
    expect([...SCHALL03_S_FAHRBAHN_TYPES]).toEqual(
      goVocabulary(vocabularyGo(), "sFahrbahnartNames"),
    );
  });

  it("matches Tabelle 8 (Maßnahme am Fahrweg) exactly", () => {
    expect([...SCHALL03_SURFACE_TYPES]).toEqual(
      goVocabulary(vocabularyGo(), "surfaceCondNames"),
    );
  });

  it("matches Tabelle 18 (Wandoberfläche) exactly", () => {
    expect([...SCHALL03_WALL_SURFACES]).toEqual(
      goVocabulary(vocabularyGo(), "wallSurfaceNames"),
    );
  });

  it("keeps the reference row first, which is what an absent property means", () => {
    // `ParseFahrbahnart("")` resolves to Schwellengleis and `ParseSurfaceCond("")`
    // to none: the editor's "not set" is that row, not a missing value.
    expect(SCHALL03_FAHRBAHN_TYPES[0]).toBe("schwellengleis");
    expect(SCHALL03_S_FAHRBAHN_TYPES[0]).toBe("schwellengleis");
    expect(SCHALL03_SURFACE_TYPES[0]).toBe("none");
    expect(SCHALL03_WALL_SURFACES[0]).toBe("hard");
  });
});

describe("arrayPropertyLength", () => {
  it("counts the entries of an array property", () => {
    expect(
      arrayPropertyLength(
        { schall03_operations: [1, 2, 3] },
        "schall03_operations",
      ),
    ).toBe(3);
  });

  it("answers null for an absent property and for a non-array one", () => {
    // Null and 0 are different answers: a feature that declares no operations
    // at all is not the same as one declaring an empty list, and only the
    // second is a model defect the extractor will name.
    expect(arrayPropertyLength(undefined, "schall03_operations")).toBeNull();
    expect(arrayPropertyLength({}, "schall03_operations")).toBeNull();
    expect(
      arrayPropertyLength({ schall03_operations: 2 }, "schall03_operations"),
    ).toBeNull();
  });
});
