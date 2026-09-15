import { describe, expect, it } from "vitest";
import { getStandardLabel, KNOWN_STANDARD_IDS } from "./standards-meta";

/**
 * The ids `backend/internal/standards/registry.go` registers.
 *
 * Written out rather than imported: the registry is Go, and this list is the
 * assertion. If a module is added there without a name here, the new id shows
 * up in the API as a raw string and this test is where that is caught — so the
 * list has to be maintained, which is the point.
 */
const REGISTERED_IDS = [
  "dummy-freefield",
  "beb-exposure",
  "bub-industry",
  "bub-rail",
  "bub-road",
  "buf-aircraft",
  "cnossos-aircraft",
  "cnossos-industry",
  "cnossos-rail",
  "cnossos-road",
  "iso9613",
  "rls19-road",
  "schall03",
];

/** Everything the registry declares `scaffold`. */
const SCAFFOLD_IDS = [
  "bub-industry",
  "bub-rail",
  "bub-road",
  "buf-aircraft",
  "cnossos-aircraft",
  "cnossos-industry",
  "cnossos-rail",
  "cnossos-road",
];

describe("getStandardLabel", () => {
  it("names every standard the backend registers", () => {
    for (const id of REGISTERED_IDS) {
      expect(getStandardLabel(id), id).not.toBe(id);
    }
  });

  it("names nothing the backend does not register", () => {
    expect([...KNOWN_STANDARD_IDS].sort()).toEqual([...REGISTERED_IDS].sort());
  });

  it("marks every scaffold as one, wherever its name is shown", () => {
    // The tier badge sits beside the name in two of the five places it is
    // rendered; the filter bar, the detail header and the list row have none,
    // and `RunSummary` carries no tier to give them one. So the limit is in the
    // name. See docs/conformance/cnossos-umfangserklaerung.md.
    for (const id of SCAFFOLD_IDS) {
      expect(getStandardLabel(id), id).toContain("Gerüst");
    }
  });

  it("does not mark a normative module as a scaffold", () => {
    for (const id of ["rls19-road", "schall03", "iso9613"]) {
      expect(getStandardLabel(id), id).not.toContain("Gerüst");
    }
  });

  it("falls back to the id a newer backend registered", () => {
    expect(getStandardLabel("something-later")).toBe("something-later");
  });
});
