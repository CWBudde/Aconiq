import { describe, expect, it } from "vitest";
import {
  getAcousticsReviewed,
  getFeatureString,
  getReceiverString,
  markAcousticsReviewed,
  needsAcousticsReview,
  setFeatureProperty,
  setReceiverProperty,
} from "./source-acoustics";
import type { ModelFeature, ModelReceiver } from "./types";

/**
 * The receiver helpers are the feature helpers' counterparts, and they share
 * their implementation precisely so the two cannot drift: a receiver whose
 * `bimschv16_area_category` were written under different rules than a source's
 * `surface_type` would be a second property model to keep in mind.
 */

const feature: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10, 51] },
};

describe("getReceiverString", () => {
  it("reads a property a receiver carries", () => {
    expect(
      getReceiverString(
        { ...receiver, properties: { bimschv16_area_category: "mixed" } },
        "bimschv16_area_category",
      ),
    ).toBe("mixed");
  });

  it("returns undefined for a receiver with no properties at all", () => {
    // Receivers drawn on the map have none; the bag is optional, not empty.
    expect(getReceiverString(receiver, "bimschv16_area_category")).toBe(
      undefined,
    );
  });

  it("skips a blank or non-string value and takes the next key", () => {
    const stored: ModelReceiver = {
      ...receiver,
      properties: {
        bimschv16_area_category: "   ",
        gebietskategorie: 7,
        area_category: " commercial ",
      },
    };

    expect(
      getReceiverString(
        stored,
        "bimschv16_area_category",
        "gebietskategorie",
        "area_category",
      ),
    ).toBe("commercial");
  });

  it("reads a receiver the way getFeatureString reads a feature", () => {
    const properties = { bimschv16_area_category: " residential " };

    expect(
      getReceiverString({ ...receiver, properties }, "bimschv16_area_category"),
    ).toBe(
      getFeatureString({ ...feature, properties }, "bimschv16_area_category"),
    );
  });
});

describe("setReceiverProperty", () => {
  it("writes a property onto a receiver that had none", () => {
    const next = setReceiverProperty(
      receiver,
      "bimschv16_area_category",
      "residential",
    );

    expect(next.properties).toEqual({
      bimschv16_area_category: "residential",
    });
    // The input is left alone: `updateReceiver` replaces the whole object and
    // the command stack keeps the previous one for undo.
    expect(receiver.properties).toBe(undefined);
  });

  it("keeps the receiver's identity, height and geometry", () => {
    const next = setReceiverProperty(
      receiver,
      "bimschv16_area_category",
      "mixed",
    );

    expect(next.id).toBe("rcv-1");
    expect(next.heightM).toBe(4);
    expect(next.geometry).toEqual(receiver.geometry);
  });

  it("leaves the receiver's other properties in place", () => {
    const next = setReceiverProperty(
      { ...receiver, properties: { name: "IO 1" } },
      "bimschv16_area_category",
      "commercial",
    );

    expect(next.properties).toEqual({
      name: "IO 1",
      bimschv16_area_category: "commercial",
    });
  });

  it("removes the key on undefined and drops the bag once it is empty", () => {
    const next = setReceiverProperty(
      { ...receiver, properties: { bimschv16_area_category: "mixed" } },
      "bimschv16_area_category",
      undefined,
    );

    // Absent rather than `{}`: `ModelReceiver.properties` is "absent or a
    // value", and an empty object would survive into every saved GeoJSON.
    expect("properties" in next).toBe(false);
  });

  it("treats the empty string as a removal", () => {
    const next = setReceiverProperty(
      {
        ...receiver,
        properties: { name: "IO 1", bimschv16_area_category: "mixed" },
      },
      "bimschv16_area_category",
      "",
    );

    expect(next.properties).toEqual({ name: "IO 1" });
  });

  it("clears the inferred marker and the aliases it replaces", () => {
    const next = setReceiverProperty(
      {
        ...receiver,
        properties: {
          bimschv16_area_category_inferred: true,
          gebietskategorie: "wohngebiet",
          bimschv16_area_category: "residential",
        },
      },
      "bimschv16_area_category",
      "mixed",
      "gebietskategorie",
    );

    expect(next.properties).toEqual({ bimschv16_area_category: "mixed" });
  });

  it("writes a receiver the way setFeatureProperty writes a feature", () => {
    const properties = { name: "IO 1" };

    expect(
      setReceiverProperty(
        { ...receiver, properties },
        "bimschv16_area_category",
        "mixed",
      ).properties,
    ).toEqual(
      setFeatureProperty(
        { ...feature, properties },
        "bimschv16_area_category",
        "mixed",
      ).properties,
    );
  });
});

describe("the acoustics sign-off", () => {
  const imported: ModelFeature = {
    ...feature,
    properties: { source_acoustics_review_required: true },
  };

  it("leaves the import's own flag alone", () => {
    // The two properties answer different questions: what the import found,
    // and what the reader decided. Collapsing them into one would leave a
    // model that cannot say whether these acoustics were ever guessed.
    const next = markAcousticsReviewed(imported, true);

    expect(next.properties).toEqual({
      source_acoustics_review_required: true,
      source_acoustics_reviewed: true,
    });
  });

  it("retires the review, and puts it back when withdrawn", () => {
    expect(needsAcousticsReview(imported)).toBe(true);

    const signed = markAcousticsReviewed(imported, true);
    expect(getAcousticsReviewed(signed)).toBe(true);
    expect(needsAcousticsReview(signed)).toBe(false);

    const withdrawn = markAcousticsReviewed(signed, false);
    expect(getAcousticsReviewed(withdrawn)).toBe(false);
    expect(needsAcousticsReview(withdrawn)).toBe(true);
  });

  it("removes the key rather than writing false", () => {
    // Absent and `false` say the same thing to every reader of this model, and
    // absent is what keeps a model diff to the values actually chosen.
    const withdrawn = markAcousticsReviewed(
      markAcousticsReviewed(imported, true),
      false,
    );

    expect(withdrawn.properties).toEqual({
      source_acoustics_review_required: true,
    });
  });

  it("says nothing is owed on a source the import never flagged", () => {
    expect(needsAcousticsReview(feature)).toBe(false);
    // Including one someone signed off anyway: the sign-off answers the flag,
    // and without a flag there was never a question.
    expect(needsAcousticsReview(markAcousticsReviewed(feature, true))).toBe(
      false,
    );
  });

  it("accepts a real boolean and nothing else", () => {
    // The same strictness `getFeatureBoolean` applies everywhere else, and the
    // reason the MapLibre filter can use `== true` and agree with this by
    // construction.
    const stringy: ModelFeature = {
      ...feature,
      properties: {
        source_acoustics_review_required: true,
        source_acoustics_reviewed: "true",
      },
    };

    expect(getAcousticsReviewed(stringy)).toBe(false);
    expect(needsAcousticsReview(stringy)).toBe(true);
  });
});
