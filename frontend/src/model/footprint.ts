/**
 * Where a footprint sits, for deciding which box it belongs to.
 *
 * Two imports that cover the same ground — OpenStreetMap and the LGLN LoD2
 * buildings — have to agree on which building is "inside" a bounding box, or
 * a building on the box's edge is either lost or doubled. The server decides
 * by the footprint's area centroid, so this does too.
 *
 * The area centroid, not the vertex average: an L-shaped building whose long
 * arm is drawn with many vertices would have its vertex average dragged into
 * that arm, and possibly out of the footprint altogether.
 */

import type { Geometry, Position } from "./types";

/** An axis-aligned box in WGS84 degrees, in the OSM import's member names. */
export interface LonLatBBox {
  south: number;
  west: number;
  north: number;
  east: number;
}

/** The CRS names a stored coordinate pair is lon/lat under. */
const GEOGRAPHIC_CRS = new Set(["EPSG:4326", "OGC:CRS84", "CRS:84"]);

/**
 * Whether coordinates stored in `crs` are WGS84 longitude/latitude, and so can
 * be compared against a {@link LonLatBBox} as they are.
 */
export function isLonLatCRS(crs: string): boolean {
  return GEOGRAPHIC_CRS.has(crs.trim().toUpperCase());
}

/** Inclusive on all four edges. */
export function bboxContains(bbox: LonLatBBox, [x, y]: Position): boolean {
  return x >= bbox.west && x <= bbox.east && y >= bbox.south && y <= bbox.north;
}

function isPosition(value: unknown): value is Position {
  return (
    Array.isArray(value) &&
    value.length >= 2 &&
    typeof value[0] === "number" &&
    typeof value[1] === "number" &&
    Number.isFinite(value[0]) &&
    Number.isFinite(value[1])
  );
}

/** The ring's vertices, without the closing duplicate of the first one. */
function openRing(ring: unknown): Position[] {
  if (!Array.isArray(ring)) return [];
  const points = ring.filter(isPosition);
  const first = points[0];
  const last = points[points.length - 1];
  if (
    points.length > 1 &&
    first !== undefined &&
    last !== undefined &&
    first[0] === last[0] &&
    first[1] === last[1]
  ) {
    return points.slice(0, -1);
  }
  return points;
}

function vertexAverage(points: Position[]): Position | null {
  if (points.length === 0) return null;
  let sx = 0;
  let sy = 0;
  for (const [x, y] of points) {
    sx += x;
    sy += y;
  }
  return [sx / points.length, sy / points.length];
}

/**
 * One exterior ring's shoelace centroid and its absolute area, or the vertex
 * average with zero area when the ring encloses nothing.
 *
 * Summed relative to the first vertex. In degrees a building's cross products
 * are products of numbers near 10 and 52 that cancel to something near 1e-8,
 * and subtracting the origin first keeps that from being rounding noise.
 */
function ringCentroid(
  ring: unknown,
): { centroid: Position; area: number } | null {
  const points = openRing(ring);
  const origin = points[0];
  if (origin === undefined) return null;

  let area2 = 0;
  let magnitude = 0;
  let cx = 0;
  let cy = 0;
  for (let i = 0; i < points.length; i++) {
    const a = points[i];
    const b = points[(i + 1) % points.length];
    if (a === undefined || b === undefined) return null;
    const ax = a[0] - origin[0];
    const ay = a[1] - origin[1];
    const bx = b[0] - origin[0];
    const by = b[1] - origin[1];
    const cross = ax * by - bx * ay;
    area2 += cross;
    magnitude += Math.abs(cross);
    cx += (ax + bx) * cross;
    cy += (ay + by) * cross;
  }

  // Degenerate: fewer than three distinct vertices, or all of them on a line.
  // Relative to the terms' own size, because an absolute epsilon means
  // something different in degrees than in metres.
  if (
    points.length < 3 ||
    !Number.isFinite(area2) ||
    Math.abs(area2) <= Number.EPSILON * 16 * magnitude
  ) {
    const average = vertexAverage(points);
    return average === null ? null : { centroid: average, area: 0 };
  }

  const factor = 1 / (3 * area2);
  return {
    centroid: [origin[0] + cx * factor, origin[1] + cy * factor],
    area: Math.abs(area2) / 2,
  };
}

/**
 * One polygon's centroid and net area: the exterior's area moment minus each
 * hole's, which is what `geo.PolygonCentroid` on the server computes. A hole
 * that encloses nothing removes nothing. A polygon whose holes leave no area
 * falls back to the exterior's centroid with zero area.
 */
function polygonCentroid(
  polygon: unknown,
): { centroid: Position; area: number } | null {
  if (!Array.isArray(polygon)) return null;
  const [exteriorRing, ...holeRings] = polygon as unknown[];
  const exterior = ringCentroid(exteriorRing);
  if (exterior === null || exterior.area === 0) return exterior;

  let area = exterior.area;
  let mx = exterior.area * exterior.centroid[0];
  let my = exterior.area * exterior.centroid[1];
  for (const ring of holeRings) {
    const hole = ringCentroid(ring);
    if (hole === null || hole.area === 0) continue;
    area -= hole.area;
    mx -= hole.area * hole.centroid[0];
    my -= hole.area * hole.centroid[1];
  }

  if (!(area > Number.EPSILON * 16 * exterior.area)) {
    return { centroid: exterior.centroid, area: 0 };
  }
  return { centroid: [mx / area, my / area], area };
}

/**
 * The area centroid of a footprint, or `null` for a geometry that is not a
 * polygon or holds no usable vertex.
 *
 * Holes are subtracted, as the server does: a courtyard moves the centroid,
 * and a building on the box edge must land on the same side for both. A
 * MultiPolygon is the area-weighted mean of its parts' centroids; one whose
 * parts all enclose nothing falls back to the mean of those parts' centroids.
 */
export function footprintCentroid(geometry: Geometry): Position | null {
  const coords: unknown = geometry.coordinates;
  if (!Array.isArray(coords)) return null;

  let polygons: unknown[];
  if (geometry.type === "Polygon") {
    polygons = [coords];
  } else if (geometry.type === "MultiPolygon") {
    polygons = coords;
  } else {
    return null;
  }

  const parts = polygons
    .map(polygonCentroid)
    .filter((part): part is { centroid: Position; area: number } => {
      return part !== null;
    });
  if (parts.length === 0) return null;

  const total = parts.reduce((sum, part) => sum + part.area, 0);
  if (total > 0) {
    let x = 0;
    let y = 0;
    for (const { centroid, area } of parts) {
      x += centroid[0] * area;
      y += centroid[1] * area;
    }
    return [x / total, y / total];
  }
  return vertexAverage(parts.map((part) => part.centroid));
}
