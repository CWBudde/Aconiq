import { describe, expect, it } from "vitest";
import { featureFilter } from "@maplibre/maplibre-gl-style-spec";
import type { FilterSpecification } from "maplibre-gl";
import { LAYER_IDS, REVIEW_LAYERS } from "./layers";
import {
  markAcousticsReviewed,
  needsAcousticsReview,
} from "@/model/source-acoustics";
import type { ModelFeature } from "@/model/types";

/**
 * The review flags, as MapLibre itself decides them.
 *
 * These layers are the only thing that takes a signed-off road's violet casing
 * back off the map, and their filter is a data expression rather than code — so
 * nothing in a component test reaches it. It is run here through
 * `@maplibre/maplibre-gl-style-spec`, the evaluator MapLibre uses at draw time,
 * which is why that package is an explicit devDependency: a hand-check of an
 * expression rots, and the claim the filter makes is one the editor and the
 * validator depend on.
 *
 * Each case is asserted against `needsAcousticsReview` as well as against a
 * literal, because "the map and the validator cannot disagree" is the actual
 * invariant; the literals alone would only say the filter did not change.
 */

/**
 * The line layer's filter, resolved once.
 *
 * `LayerSpecification` is a union and a background layer carries no `filter`,
 * hence the narrowing; a review layer that lost its filter would draw the flag
 * on every source, so this asserts rather than defaulting to "matches all".
 */
function reviewLineFilter(): FilterSpecification {
  const layer = REVIEW_LAYERS.find(
    (spec) => spec.id === LAYER_IDS.sourcesReviewLine,
  );
  const filter =
    layer !== undefined && "filter" in layer ? layer.filter : undefined;
  expect(filter).toBeDefined();
  return filter as FilterSpecification;
}

/** The filter as MapLibre evaluates it, for a line geometry. */
function drawsFlag(properties: Record<string, unknown>): boolean {
  return featureFilter(reviewLineFilter()).filter(
    { zoom: 14 } as never,
    // `type: 2` is MapLibre's vector-tile code for a LineString.
    { type: 2, properties } as never,
    {} as never,
  );
}

/** The same properties as a model feature, for the validator's own predicate. */
function asFeature(properties: Record<string, unknown>): ModelFeature {
  return {
    id: "src-1",
    kind: "source",
    sourceType: "line",
    geometry: {
      type: "LineString",
      coordinates: [
        [10, 51],
        [10.01, 51.01],
      ],
    },
    properties,
  };
}

describe("the review-flag filter", () => {
  const imported = { source_acoustics_review_required: true };

  it("flags a source the import had to guess at", () => {
    expect(drawsFlag(imported)).toBe(true);
    expect(needsAcousticsReview(asFeature(imported))).toBe(true);
  });

  it("clears the flag once the reader has signed the source off", () => {
    // The regression this file exists for: without the second half of the
    // filter, a road the panel and the editor both consider done stays violet.
    const signed = markAcousticsReviewed(asFeature(imported), true).properties;

    expect(drawsFlag(signed ?? {})).toBe(false);
    expect(needsAcousticsReview(asFeature(signed ?? {}))).toBe(false);
  });

  it("keeps flagging a source whose sign-off was withdrawn", () => {
    const withdrawn = markAcousticsReviewed(
      markAcousticsReviewed(asFeature(imported), true),
      false,
    ).properties;

    expect(drawsFlag(withdrawn ?? {})).toBe(true);
    expect(needsAcousticsReview(asFeature(withdrawn ?? {}))).toBe(true);
  });

  it("treats an explicit false sign-off as no sign-off", () => {
    // Nothing in the app writes this — withdrawing removes the key — but a
    // hand-edited or imported model can carry it.
    const explicit = { ...imported, source_acoustics_reviewed: false };

    expect(drawsFlag(explicit)).toBe(true);
    expect(needsAcousticsReview(asFeature(explicit))).toBe(true);
  });

  it("leaves a source the import never flagged alone", () => {
    expect(drawsFlag({})).toBe(false);
    expect(needsAcousticsReview(asFeature({}))).toBe(false);
  });

  it("reads booleans only, exactly as the validator does", () => {
    // The reason map and validator agree by construction: `getFeatureBoolean`
    // accepts a real boolean and nothing else, and MapLibre's `==` against a
    // boolean literal is just as strict. A model carrying the string "true"
    // raises no finding, and must therefore draw no flag either.
    const stringly = { source_acoustics_review_required: "true" };
    expect(drawsFlag(stringly)).toBe(false);
    expect(needsAcousticsReview(asFeature(stringly))).toBe(false);

    // And a string sign-off must not clear a real flag.
    const stringlySigned = { ...imported, source_acoustics_reviewed: "true" };
    expect(drawsFlag(stringlySigned)).toBe(true);
    expect(needsAcousticsReview(asFeature(stringlySigned))).toBe(true);
  });
});

/**
 * Sources traced from imagery (P18 E2) are drawn dashed while proposed, by
 * layers of their own: a proposal is not an acoustics finding, so it stays
 * out of the filter above and the invariant that filter shares with the
 * validator.
 */
describe("the proposed-source flag", () => {
  function drawsProposed(
    properties: Record<string, unknown>,
    type: 1 | 2 = 2,
  ): boolean {
    const id =
      type === 1
        ? LAYER_IDS.sourcesProposedPoint
        : LAYER_IDS.sourcesProposedLine;
    const layer = REVIEW_LAYERS.find((spec) => spec.id === id);
    const filter =
      layer !== undefined && "filter" in layer ? layer.filter : undefined;
    expect(filter).toBeDefined();
    return featureFilter(filter as FilterSpecification).filter(
      { zoom: 14 } as never,
      { type, properties } as never,
      {} as never,
    );
  }

  it("flags a proposed source, point or line", () => {
    expect(drawsProposed({ review_status: "proposed" })).toBe(true);
    expect(drawsProposed({ review_status: "proposed" }, 1)).toBe(true);
  });

  it("leaves a confirmed, rejected or unreviewed source alone", () => {
    expect(drawsProposed({ review_status: "confirmed" })).toBe(false);
    expect(drawsProposed({ review_status: "rejected" })).toBe(false);
    expect(drawsProposed({})).toBe(false);
  });

  it("does not draw the acoustics flag for a proposal", () => {
    expect(drawsFlag({ review_status: "proposed" })).toBe(false);
  });
});

describe("a rejected source", () => {
  it("is drawn faded, and only it", async () => {
    const { createExpression } =
      await import("@maplibre/maplibre-gl-style-spec");
    const { SOURCE_LAYERS } = await import("./layers");
    const line = SOURCE_LAYERS.find(
      (spec) => spec.id === LAYER_IDS.sourcesLine,
    );
    const opacity =
      line !== undefined && "paint" in line
        ? (line.paint as Record<string, unknown>)["line-opacity"]
        : undefined;
    const parsed = createExpression(opacity);
    if (parsed.result !== "success") throw new Error("unparsable opacity");
    const at = (properties: Record<string, unknown>): unknown =>
      parsed.value.evaluate(
        { zoom: 14 } as never,
        {
          type: 2,
          properties,
        } as never,
      );

    expect(at({ review_status: "rejected" })).toBe(0.3);
    expect(at({ review_status: "proposed" })).toBe(1);
    expect(at({})).toBe(1);
  });
});
