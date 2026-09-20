import type { Point2D } from "./geometry";

/**
 * The self-intersection test, ported from `modelgeojson.hasSelfIntersection`.
 *
 * It is a port and not a second opinion. The backend refuses a model whose
 * polygon ring crosses itself, and the browser had no geometry checks at all,
 * so the import preview called an OSM extract clean and the save then refused
 * it — one screen later, with nothing on the earlier screen to explain it. The
 * two answers have to be the same answer, which means the same predicate: the
 * codes in `ValidationIssueParams` are the backend's spellings for the same
 * reason.
 *
 * The right long-term home for this is the WASM kernel, the way `transform`
 * and `contours` are already the Go module's answers rather than the
 * frontend's. Until the kernel exports validation, this file is a copy, and a
 * copy is a thing that drifts — so keep it next to the Go source it came from
 * when either changes.
 */

/**
 * The cost bound, matching `maxSelfIntersectionPoints`.
 *
 * The check compares every pair of segments, so its cost grows with the square
 * of the vertex count. 10,000 points is about 5*10^7 segment-pair tests; a
 * machine-generated import an order of magnitude larger would freeze the tab,
 * which in a browser is worse than on a server. Geometries above the bound are
 * reported as unchecked rather than rejected.
 */
export const SELF_INTERSECTION_POINT_LIMIT = 10_000;

/**
 * The tolerance `orientation` applies, as a fraction of the magnitudes that
 * produced the cross product — `relativeEpsilon` on the Go side.
 *
 * Relative, and that is the whole point. The value is a cross product, so its
 * unit is the coordinate unit squared: two edges of one building measure ~1e-4
 * apart in EPSG:4326 and ~10 in EPSG:25832, which puts their cross product at
 * ~1e-8 against ~1e2. A fixed tolerance therefore answers differently for the
 * same footprint depending only on the CRS it arrived in.
 */
const RELATIVE_EPSILON = 1e-12;

/** Orientation of the turn at `b` along a → b → c: 0 collinear, 1 or 2 otherwise. */
function orientation(a: Point2D, b: Point2D, c: Point2D): number {
  const abx = b.x - a.x;
  const aby = b.y - a.y;
  const bcx = c.x - b.x;
  const bcy = c.y - b.y;

  const value = aby * bcx - abx * bcy;
  const tolerance =
    RELATIVE_EPSILON *
    (Math.abs(abx) + Math.abs(aby)) *
    (Math.abs(bcx) + Math.abs(bcy));

  if (Math.abs(value) <= tolerance) return 0;
  return value > 0 ? 1 : 2;
}

/**
 * Whether `b` lies within the bounding box of `a`–`c`.
 *
 * A box test, which is sound only for a triple `orientation` has already
 * called collinear. That is exactly why the tolerance above has to be tight:
 * a wrong "collinear" turns this into the whole answer, and any two edges of a
 * non-convex footprint whose boxes overlap then read as crossing.
 */
function onSegment(a: Point2D, b: Point2D, c: Point2D): boolean {
  const epsilon = 1e-9;
  return (
    b.x <= Math.max(a.x, c.x) + epsilon &&
    b.x + epsilon >= Math.min(a.x, c.x) &&
    b.y <= Math.max(a.y, c.y) + epsilon &&
    b.y + epsilon >= Math.min(a.y, c.y)
  );
}

function segmentsIntersect(
  a: Point2D,
  b: Point2D,
  c: Point2D,
  d: Point2D,
): boolean {
  const o1 = orientation(a, b, c);
  const o2 = orientation(a, b, d);
  const o3 = orientation(c, d, a);
  const o4 = orientation(c, d, b);

  if (o1 !== o2 && o3 !== o4) return true;
  if (o1 === 0 && onSegment(a, c, b)) return true;
  if (o2 === 0 && onSegment(a, d, b)) return true;
  if (o3 === 0 && onSegment(c, a, d)) return true;
  if (o4 === 0 && onSegment(c, b, d)) return true;

  return false;
}

/**
 * Segments that share an endpoint, which touch by construction rather than by
 * defect. On a closed ring the first and last segment are neighbours too.
 */
function areAdjacentSegments(
  i: number,
  j: number,
  segmentCount: number,
  closed: boolean,
): boolean {
  if (i === j) return true;
  if (j === i + 1) return true;
  return closed && i === 0 && j === segmentCount - 1;
}

/** The answer, or `null` when the geometry exceeded the cost bound. */
export function hasSelfIntersection(
  points: Point2D[],
  closed: boolean,
): boolean | null {
  if (points.length > SELF_INTERSECTION_POINT_LIMIT) return null;
  if (points.length < 4) return false;

  const segmentCount = points.length - 1;

  for (let i = 0; i < segmentCount; i++) {
    const a1 = points[i];
    const a2 = points[i + 1];
    if (!a1 || !a2) continue;

    for (let j = i + 1; j < segmentCount; j++) {
      if (areAdjacentSegments(i, j, segmentCount, closed)) continue;

      const b1 = points[j];
      const b2 = points[j + 1];
      if (!b1 || !b2) continue;

      if (segmentsIntersect(a1, a2, b1, b2)) return true;
    }
  }

  return false;
}
