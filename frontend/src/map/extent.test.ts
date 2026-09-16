import { describe, expect, it } from "vitest";
import {
  computeWorkspaceBounds,
  fitViewToWorkspace,
  toLngLatBounds,
} from "./extent";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

const features: ModelFeature[] = [
  {
    id: "source-1",
    kind: "source",
    sourceType: "point",
    geometry: {
      type: "Point",
      coordinates: [10, 50],
    },
  },
];
const receivers: ModelReceiver[] = [
  {
    id: "receiver-1",
    geometry: {
      type: "Point",
      coordinates: [11, 51],
    },
    heightM: 4,
  },
];
const calcArea: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [9, 49],
        [12, 49],
        [12, 52],
        [9, 52],
        [9, 49],
      ],
    ],
  },
};

describe("fitViewToWorkspace", () => {
  it("centers on workspace geometry when present", () => {
    const view = fitViewToWorkspace(
      features,
      receivers,
      calcArea,
      [10.45, 51.16],
      "EPSG:4326",
    );

    expect(view.center[0]).toBeCloseTo(10.5, 1);
    expect(view.center[1]).toBeCloseTo(50.5, 1);
    expect(view.zoom).toBeLessThanOrEqual(12);
  });

  it("falls back to the provided center when no geometry is available", () => {
    const view = fitViewToWorkspace([], [], null, [10.45, 51.16], "EPSG:4326");

    expect(view.center).toEqual([10.45, 51.16]);
    expect(view.zoom).toBe(6);
  });

  it("falls back to the provided center for a workspace that is not lon/lat", () => {
    // It runs in a `useState` initialiser and cannot await a projection, so a
    // metric workspace gets no centre computed from it. Centring on the mean
    // of the eastings and northings put the map on [667000, 5644000], which
    // MapLibre clamps to the edge of the world.
    const metric: ModelFeature[] = [
      {
        id: "source-1",
        kind: "source",
        sourceType: "point",
        geometry: { type: "Point", coordinates: [667000, 5644000] },
      },
    ];

    const view = fitViewToWorkspace(
      metric,
      [],
      null,
      [10.45, 51.16],
      "EPSG:25832",
    );

    expect(view.center).toEqual([10.45, 51.16]);
    expect(view.zoom).toBe(6);
  });
});

describe("computeWorkspaceBounds", () => {
  it("visits features, receivers and the calculation area", () => {
    expect(computeWorkspaceBounds(features, receivers, calcArea)).toEqual({
      west: 9,
      south: 49,
      east: 12,
      north: 52,
    });
  });

  it("covers a workspace that holds receivers and nothing else", () => {
    // The copy `model-layers.tsx` kept visited features alone, so a
    // receiver-only import never came into view: it computed no bounds at all.
    expect(computeWorkspaceBounds([], receivers, null)).toEqual({
      west: 11,
      south: 51,
      east: 11,
      north: 51,
    });
  });

  it("answers null for a workspace with no coordinate in it", () => {
    expect(computeWorkspaceBounds([], [], null)).toBeNull();
  });
});

describe("toLngLatBounds", () => {
  it("passes a lon/lat extent through", () => {
    expect(toLngLatBounds({ west: 9, south: 49, east: 12, north: 52 })).toEqual(
      [
        [9, 49],
        [12, 52],
      ],
    );
  });

  it("refuses an extent in metres", () => {
    // The case reprojection cannot fix: a store labelled EPSG:4326 that in
    // fact holds eastings and northings. MapLibre answers that with
    // `Invalid LngLat latitude value`, thrown out of the effect that fitted it.
    expect(
      toLngLatBounds({
        west: 666000,
        south: 5643000,
        east: 668000,
        north: 5645000,
      }),
    ).toBeNull();
  });

  it("refuses a longitude past the antimeridian and a non-finite value", () => {
    expect(
      toLngLatBounds({ west: -181, south: 0, east: 0, north: 1 }),
    ).toBeNull();
    expect(
      toLngLatBounds({
        west: 0,
        south: 0,
        east: Number.NaN,
        north: 1,
      }),
    ).toBeNull();
  });
});
