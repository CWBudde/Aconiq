import { describe, expect, it } from "vitest";
import { getStandardLabel, KNOWN_STANDARD_IDS } from "./standards-meta";
import { EVIDENCE_TIERS } from "@/api/evidence-tier";
import { m } from "@/i18n/messages";

/**
 * The ids `backend/internal/standards/registry.go` registers today.
 *
 * Copied, not derived — and that is a limit worth stating plainly rather than
 * dressing up. The registry composes its ids inside each module's
 * `Descriptor()`, so there is no file a test can read to learn them, and a
 * module added there while this list stays untouched leaves every assertion
 * below green. What catches that case at runtime is `getStandardLabel`'s
 * fallback: an unnamed id prints as itself, which is ugly and honest, not
 * wrong. A generated standards manifest shared by both targets is the real
 * fix; `PLAN.md` carries it as open work.
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

describe("getStandardLabel", () => {
  it("names every standard the backend registers", () => {
    for (const id of REGISTERED_IDS) {
      expect(getStandardLabel(id), id).not.toBe(id);
    }
  });

  it("names nothing the backend does not register", () => {
    expect([...KNOWN_STANDARD_IDS].sort()).toEqual([...REGISTERED_IDS].sort());
  });

  it("falls back to the id a newer backend registered", () => {
    expect(getStandardLabel("something-later")).toBe("something-later");
  });
});

describe("the evidence qualifier", () => {
  it("is taken from the tier it is given, not from the id", () => {
    // The whole point: the same module reads differently when the backend
    // reclassifies it, and no table here has to be edited for that to happen.
    expect(getStandardLabel("bub-road", "scaffold")).toContain(
      m.evidence_tier_scaffold(),
    );
    expect(getStandardLabel("bub-road", "normative")).not.toContain(
      m.evidence_tier_scaffold(),
    );
  });

  it("qualifies every tier that is not normative", () => {
    for (const tier of EVIDENCE_TIERS) {
      const label = getStandardLabel("rls19-road", tier);
      if (tier === "normative") {
        expect(label, tier).toBe("RLS-19 Straße");
      } else {
        expect(label, tier).not.toBe("RLS-19 Straße");
      }
    }
  });

  it("says nothing when no tier was published", () => {
    // An older backend omits the field, and an absent claim is not a tier —
    // the same rule `EvidenceTierBadge` follows. Guessing one from the id is
    // what this module stopped doing.
    expect(getStandardLabel("bub-road")).toBe("BUB Straße");
    expect(getStandardLabel("bub-road", "")).toBe("BUB Straße");
  });

  it("invents nothing for a tier this build does not know", () => {
    expect(getStandardLabel("bub-road", "provisional")).toBe("BUB Straße");
  });

  it("spells the qualifier with the badge's own words", () => {
    // One source for the wording, so the name beside a badge and the badge
    // cannot disagree.
    expect(getStandardLabel("dummy-freefield", "test-fixture")).toBe(
      `Freifeld (${m.evidence_tier_test_fixture()})`,
    );
  });
});
