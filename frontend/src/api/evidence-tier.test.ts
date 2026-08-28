import { describe, expect, it } from "vitest";
import {
  EVIDENCE_TIERS,
  isScaffoldTier,
  parseEvidenceTier,
} from "./evidence-tier";

describe("parseEvidenceTier", () => {
  it("accepts every tier the backend declares", () => {
    for (const tier of EVIDENCE_TIERS) {
      expect(parseEvidenceTier(tier)).toBe(tier);
    }
  });

  it("treats an absent or blank field as no claim at all", () => {
    expect(parseEvidenceTier(undefined)).toBeNull();
    expect(parseEvidenceTier("")).toBeNull();
    expect(parseEvidenceTier("  ")).toBeNull();
  });

  it("does not guess at a tier it was never taught", () => {
    expect(parseEvidenceTier("provisional")).toBe("unknown");
    expect(parseEvidenceTier("NORMATIVE")).toBe("unknown");
  });

  it("tolerates surrounding whitespace on a known tier", () => {
    expect(parseEvidenceTier(" scaffold ")).toBe("scaffold");
  });
});

describe("isScaffoldTier", () => {
  it("is true only for scaffold", () => {
    expect(isScaffoldTier("scaffold")).toBe(true);
    expect(isScaffoldTier("normative")).toBe(false);
    expect(isScaffoldTier("preview")).toBe(false);
    expect(isScaffoldTier("test-fixture")).toBe(false);
  });

  it("never claims scaffold on missing or unknown evidence", () => {
    expect(isScaffoldTier(undefined)).toBe(false);
    expect(isScaffoldTier("scaffolding")).toBe(false);
  });
});
