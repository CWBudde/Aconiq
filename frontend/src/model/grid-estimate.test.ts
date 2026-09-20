import { describe, expect, it } from "vitest";
import {
  DEFAULT_GRID_PADDING_M,
  DEFAULT_GRID_RESOLUTION_M,
  getFeatureBBox,
  gridShape,
  gridSizingFromParams,
  RECEIVER_WARNING_THRESHOLD,
  resolutionForReceiverBudget,
  resolveGridExtent,
} from "./grid-estimate";
import type { CalcArea, ModelFeature } from "./types";

/**
 * The arithmetic the run dialog quotes and the browser backend builds from.
 * `api/browser-parity.test.ts` pins that the receivers this shape produces are
 * the CLI's; what is pinned here is the shape itself, because the preview
 * shows it to the reader before any of that runs.
 */

function source(
  coordinates: ModelFeature["geometry"]["coordinates"],
  type: ModelFeature["geometry"]["type"],
): ModelFeature {
  return {
    id: "src",
    kind: "source",
    sourceType: "line",
    properties: {},
    geometry: { type, coordinates },
  };
}

const ROAD = source(
  [
    [0, 0],
    [1000, 500],
  ],
  "LineString",
);

/** A 200 × 100 m lot, offset from the road so the two extents differ. */
const LOT = source(
  [
    [
      [0, 0],
      [200, 0],
      [200, 100],
      [0, 100],
      [0, 0],
    ],
  ],
  "Polygon",
);

function calcArea(rings: number[][][]): CalcArea {
  return {
    id: "calc-area",
    geometry: {
      type: "Polygon",
      coordinates: rings as CalcArea["geometry"]["coordinates"],
    },
  };
}

describe("gridSizingFromParams", () => {
  it("reads the two grid parameters", () => {
    expect(
      gridSizingFromParams({ grid_resolution_m: "25", grid_padding_m: "50" }),
    ).toEqual({ resolutionM: 25, paddingM: 50 });
  });

  it("falls back where a field is missing or not a number", () => {
    // The dialog's fields are free text, so this is the path a half-typed
    // value takes. It has to land on the value the run would use.
    expect(gridSizingFromParams({})).toEqual({
      resolutionM: DEFAULT_GRID_RESOLUTION_M,
      paddingM: DEFAULT_GRID_PADDING_M,
    });
    expect(
      gridSizingFromParams({ grid_resolution_m: "", grid_padding_m: "abc" }),
    ).toEqual({
      resolutionM: DEFAULT_GRID_RESOLUTION_M,
      paddingM: DEFAULT_GRID_PADDING_M,
    });
  });
});

describe("getFeatureBBox", () => {
  // The CLI had to be taught this explicitly: padding a grid around a lot's
  // centroid alone puts the whole grid inside the source.
  it("covers every vertex of an area source, not just its middle", () => {
    expect(getFeatureBBox([LOT])).toEqual({
      minX: 0,
      minY: 0,
      maxX: 200,
      maxY: 100,
    });
  });

  it("is null for features that hold no coordinate", () => {
    expect(getFeatureBBox([])).toBeNull();
    expect(getFeatureBBox([source([], "MultiPoint")])).toBeNull();
  });
});

describe("resolveGridExtent", () => {
  it("takes the calculation area when there is one", () => {
    const extent = resolveGridExtent({
      features: [ROAD],
      calcArea: calcArea([
        [
          [-50, -50],
          [50, -50],
          [50, 50],
          [-50, 50],
          [-50, -50],
        ],
      ]),
    });
    expect(extent).toEqual({ minX: -50, minY: -50, maxX: 50, maxY: 50 });
  });

  it("takes the sources' bounding box when there is no calculation area", () => {
    expect(
      resolveGridExtent({ features: [ROAD, LOT], calcArea: null }),
    ).toEqual({
      minX: 0,
      minY: 0,
      maxX: 1000,
      maxY: 500,
    });
  });

  it("ignores everything that is not a source", () => {
    const building: ModelFeature = {
      id: "b",
      kind: "building",
      heightM: 10,
      properties: {},
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [9000, 9000],
            [9100, 9000],
            [9100, 9100],
            [9000, 9000],
          ],
        ],
      },
    };
    expect(
      resolveGridExtent({ features: [LOT, building], calcArea: null }),
    ).toEqual({ minX: 0, minY: 0, maxX: 200, maxY: 100 });
  });

  it("falls through to the sources for a degenerate calculation area", () => {
    // An area whose ring holds no usable coordinate is not an instruction to
    // produce no grid — which is what `computeRLS19Road` did before this
    // moved, and has to go on doing.
    expect(
      resolveGridExtent({ features: [LOT], calcArea: calcArea([[]]) }),
    ).toEqual({ minX: 0, minY: 0, maxX: 200, maxY: 100 });
  });

  it("is null with neither a calculation area nor a source", () => {
    expect(resolveGridExtent({ features: [], calcArea: null })).toBeNull();
  });
});

describe("gridShape", () => {
  const extent = { minX: 0, minY: 0, maxX: 200, maxY: 100 };

  it("pads all four sides and counts the cells", () => {
    // 200 + 2 × 20 = 240 m wide, 100 + 2 × 20 = 140 m tall, at 10 m.
    expect(gridShape(extent, { resolutionM: 10, paddingM: 20 })).toEqual({
      width: 25,
      height: 15,
      count: 375,
      originX: -20,
      originY: -20,
    });
  });

  it("puts the origin on the padded south-west corner", () => {
    const shape = gridShape(extent, { resolutionM: 10, paddingM: 50 });
    expect(shape?.originX).toBe(-50);
    expect(shape?.originY).toBe(-50);
  });

  it("keeps at least one receiver for an extent smaller than a cell", () => {
    const shape = gridShape(
      { minX: 0, minY: 0, maxX: 1, maxY: 1 },
      { resolutionM: 100, paddingM: 0 },
    );
    expect(shape).toEqual({
      width: 1,
      height: 1,
      count: 1,
      originX: 0,
      originY: 0,
    });
  });

  it("reproduces the 2 km site the default resolution walks into", () => {
    // The case this preview exists for: a plausible site, an untouched 10 m,
    // and a count nothing in the dialog used to mention.
    const shape = gridShape(
      { minX: 0, minY: 0, maxX: 2000, maxY: 2000 },
      { resolutionM: 10, paddingM: 20 },
    );
    expect(shape?.count).toBe(205 * 205);
    expect(shape?.count).toBeGreaterThan(RECEIVER_WARNING_THRESHOLD);
  });

  it("refuses a resolution that cannot describe a grid", () => {
    // Zero used to make the cell count infinite and freeze the tab in
    // `buildReceiverGrid`'s loop.
    expect(gridShape(extent, { resolutionM: 0, paddingM: 20 })).toBeNull();
    expect(gridShape(extent, { resolutionM: -5, paddingM: 20 })).toBeNull();
    expect(
      gridShape(extent, { resolutionM: Number.NaN, paddingM: 20 }),
    ).toBeNull();
    expect(
      gridShape(extent, { resolutionM: 10, paddingM: Number.NaN }),
    ).toBeNull();
  });
});

describe("resolutionForReceiverBudget", () => {
  const site = { minX: 0, minY: 0, maxX: 2000, maxY: 2000 };

  it("offers the finest ladder step that stays within the budget", () => {
    const suggested = resolutionForReceiverBudget(
      site,
      20,
      RECEIVER_WARNING_THRESHOLD,
    );
    expect(suggested).not.toBeNull();

    const shape = gridShape(site, {
      resolutionM: suggested as number,
      paddingM: 20,
    });
    expect(shape?.count).toBeLessThanOrEqual(RECEIVER_WARNING_THRESHOLD);

    // And it really is the finest: one step down is over.
    const finer = gridShape(site, { resolutionM: 10, paddingM: 20 });
    expect(finer?.count).toBeGreaterThan(RECEIVER_WARNING_THRESHOLD);
  });

  it("is null where even the coarsest step would not fit", () => {
    expect(resolutionForReceiverBudget(site, 20, 0)).toBeNull();
  });
});
