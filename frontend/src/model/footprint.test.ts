import { describe, expect, it } from "vitest";
import { bboxContains, footprintCentroid, isLonLatCRS } from "./footprint";
import type { Geometry, Position } from "./types";

function polygon(ring: Position[]): Geometry {
  return { type: "Polygon", coordinates: [ring] };
}

describe("footprintCentroid", () => {
  it("is the middle of a square", () => {
    const c = footprintCentroid(
      polygon([
        [0, 0],
        [2, 0],
        [2, 2],
        [0, 2],
        [0, 0],
      ]),
    );
    expect(c?.[0]).toBeCloseTo(1, 12);
    expect(c?.[1]).toBeCloseTo(1, 12);
  });

  // The reason the vertex average is not acceptable: an L drawn with many
  // vertices along one arm drags the average into that arm. The area centroid
  // of this L (a 2×2 square minus its upper-right 1×1) is (5/6, 5/6).
  it("weights an L-shaped footprint by area, not by vertex count", () => {
    const ring: Position[] = [
      [0, 0],
      [2, 0],
      // Extra vertices along the bottom arm.
      [2, 0.25],
      [2, 0.5],
      [2, 0.75],
      [2, 1],
      [1, 1],
      [1, 2],
      [0, 2],
      [0, 0],
    ];
    const c = footprintCentroid(polygon(ring));
    expect(c?.[0]).toBeCloseTo(5 / 6, 12);
    expect(c?.[1]).toBeCloseTo(5 / 6, 12);

    const open = ring.slice(0, -1);
    const average = open.reduce((acc, [x]) => acc + x, 0) / open.length;
    expect(average).not.toBeCloseTo(5 / 6, 2);
  });

  it("does not depend on winding or on the ring being closed", () => {
    const cw = footprintCentroid(
      polygon([
        [0, 0],
        [0, 2],
        [4, 2],
        [4, 0],
      ]),
    );
    expect(cw?.[0]).toBeCloseTo(2, 12);
    expect(cw?.[1]).toBeCloseTo(1, 12);
  });

  it("stays accurate for a building-sized ring in degrees", () => {
    // ~10 m × 10 m at 52°N, where the cross products nearly cancel.
    const x0 = 9.7321;
    const y0 = 52.3745;
    const dx = 0.000146;
    const dy = 0.00009;
    const c = footprintCentroid(
      polygon([
        [x0, y0],
        [x0 + dx, y0],
        [x0 + dx, y0 + dy],
        [x0, y0 + dy],
        [x0, y0],
      ]),
    );
    expect(c?.[0]).toBeCloseTo(x0 + dx / 2, 10);
    expect(c?.[1]).toBeCloseTo(y0 + dy / 2, 10);
  });

  it("falls back to the vertex average for a degenerate ring", () => {
    const c = footprintCentroid(
      polygon([
        [0, 0],
        [1, 1],
        [2, 2],
        [0, 0],
      ]),
    );
    expect(c).toEqual([1, 1]);
  });

  it("subtracts holes, as the server does", () => {
    const c = footprintCentroid({
      type: "Polygon",
      coordinates: [
        [
          [0, 0],
          [4, 0],
          [4, 4],
          [0, 4],
          [0, 0],
        ],
        [
          [2.5, 2.5],
          [3.5, 2.5],
          [3.5, 3.5],
          [2.5, 3.5],
          [2.5, 2.5],
        ],
      ],
    });
    // Exterior 16 at (2, 2), hole 1 at (3, 3): (32 - 3) / 15.
    expect(c?.[0]).toBeCloseTo(29 / 15, 12);
    expect(c?.[1]).toBeCloseTo(29 / 15, 12);
  });

  it("weights the parts of a MultiPolygon by area", () => {
    const c = footprintCentroid({
      type: "MultiPolygon",
      coordinates: [
        [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
        [
          [
            [10, 0],
            [13, 0],
            [13, 1],
            [10, 1],
            [10, 0],
          ],
        ],
      ],
    });
    // (0.5·1 + 11.5·3) / 4
    expect(c?.[0]).toBeCloseTo(8.75, 12);
    expect(c?.[1]).toBeCloseTo(0.5, 12);
  });

  it("answers null for a geometry that is not a footprint", () => {
    expect(
      footprintCentroid({
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      }),
    ).toBeNull();
    expect(footprintCentroid({ type: "Polygon", coordinates: [] })).toBeNull();
  });
});

describe("bboxContains", () => {
  const bbox = { south: 52, west: 9, north: 53, east: 10 };

  it("is inclusive on the edges", () => {
    expect(bboxContains(bbox, [9, 52])).toBe(true);
    expect(bboxContains(bbox, [10, 53])).toBe(true);
    expect(bboxContains(bbox, [9.5, 52.5])).toBe(true);
  });

  it("reads x as longitude and y as latitude", () => {
    expect(bboxContains(bbox, [52.5, 9.5])).toBe(false);
  });
});

describe("isLonLatCRS", () => {
  it("accepts the WGS84 spellings and nothing projected", () => {
    expect(isLonLatCRS("EPSG:4326")).toBe(true);
    expect(isLonLatCRS("ogc:crs84")).toBe(true);
    expect(isLonLatCRS("EPSG:25832")).toBe(false);
  });
});
