import { describe, expect, it } from "vitest";
import {
  getFeatureString,
  getReceiverString,
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
