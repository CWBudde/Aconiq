import { describe, expect, it } from "vitest";
import type { ModelFeature } from "./types";
import {
  featureBBox,
  proposedSourceCount,
  sourceEvidence,
  sourceOptInMissing,
  sourceReviewStatus,
  tracedFromImagery,
  withSourceReviewStatus,
} from "./source-review";

function source(
  id: string,
  properties?: Record<string, unknown>,
): ModelFeature {
  return {
    id,
    kind: "source",
    sourceType: "point",
    geometry: { type: "Point", coordinates: [1, 2] },
    ...(properties !== undefined ? { properties } : {}),
  };
}

const EVIDENCE = { kind: "orthophoto", source_id: "https://wms" };

describe("source review", () => {
  it("reads only a known status, and only on a source", () => {
    expect(sourceReviewStatus(source("a"))).toBeUndefined();
    expect(sourceReviewStatus(source("a", { review_status: "proposed" }))).toBe(
      "proposed",
    );
    expect(
      sourceReviewStatus(source("a", { review_status: "maybe" })),
    ).toBeUndefined();
    expect(
      sourceReviewStatus({
        ...source("a", { review_status: "proposed" }),
        kind: "building",
      }),
    ).toBeUndefined();
  });

  it("counts the proposed sources a normative run would refuse", () => {
    const features = [
      source("a", { review_status: "proposed" }),
      source("b", { review_status: "confirmed" }),
      source("c", { review_status: "rejected" }),
      source("d"),
      source("e", { review_status: "proposed" }),
    ];
    expect(proposedSourceCount(features)).toBe(2);
    expect(sourceOptInMissing(2, true, false)).toBe(true);
    expect(sourceOptInMissing(2, true, true)).toBe(false);
    expect(sourceOptInMissing(2, false, false)).toBe(false);
    expect(sourceOptInMissing(0, true, false)).toBe(false);
  });

  it("marks a traced source proposed with its evidence, keeping its other properties", () => {
    expect(tracedFromImagery({ traffic_day_pkw: 300 }, EVIDENCE)).toEqual({
      traffic_day_pkw: 300,
      review_status: "proposed",
      evidence: [EVIDENCE],
    });
  });

  it("changes the status and keeps the evidence it was read from", () => {
    const traced = source("a", tracedFromImagery({}, EVIDENCE));
    const confirmed = withSourceReviewStatus(traced, "confirmed");

    expect(sourceReviewStatus(confirmed)).toBe("confirmed");
    expect(sourceEvidence(confirmed)).toEqual([EVIDENCE]);
    expect(sourceEvidence(source("b"))).toEqual([]);
  });

  it("bounds a feature's geometry in its own coordinates", () => {
    expect(
      featureBBox({
        type: "Polygon",
        coordinates: [
          [
            [3, 4],
            [10, 1],
            [7, 9],
            [3, 4],
          ],
        ],
      }),
    ).toEqual([3, 1, 10, 9]);
    expect(featureBBox({ type: "Point", coordinates: [5, 6] })).toEqual([
      5, 6, 5, 6,
    ]);
  });
});
