// RLS-19 §3.4 Parkplatz extraction for browser mode.
//
// This mirrors backend/internal/app/cli/run_extract_rls19_parking.go property
// for property and message for message. The two extractors feed the same
// kernel, so a model that runs in the CLI must run in the browser and produce
// the same scene; src/wasm/parking-vocabulary.test.ts pins the names against
// the Go source so the two cannot drift silently.

import type {
  ParkingFacilityType,
  ParkingLotType,
  ParkingSource,
  Point2D,
} from "@/wasm/types";
import { getFeatureNumber, getFeatureString } from "./source-acoustics";
import type { ModelFeature } from "./types";

// Property keys. Mirrors the const block in run_extract_rls19_parking.go.
export const PROP_PARKING_NUM_SPACES = "rls19_parking_num_spaces";
export const PROP_PARKING_TYPE = "rls19_parking_type";
export const PROP_PARKING_FACILITY_TYPE = "rls19_parking_facility_type";
export const PROP_PARKING_MOVEMENTS_DAY =
  "rls19_parking_movements_per_space_day";
export const PROP_PARKING_MOVEMENTS_NIGHT =
  "rls19_parking_movements_per_space_night";

// Tabelle 6 Parkplatztypen, by name. The ordinal is a row position.
export const RLS19_PARKING_LOT_TYPES: readonly ParkingLotType[] = [
  "pkw",
  "motorrad",
  "lkw-omnibus",
];

// Tabelle 7 Parkplatztypen and their standard movement rates N per space and
// hour, day before night. The table has exactly these two rows, which is why an
// ordinary public car park has no standard rate to fall back to.
export const RLS19_PARKING_FACILITY_RATES: ReadonlyArray<{
  type: ParkingFacilityType;
  day: number;
  night: number;
}> = [
  { type: "park-and-ride", day: 0.3, night: 0.06 },
  { type: "tank-rastanlage", day: 1.5, night: 0.8 },
];

export const RLS19_PARKING_FACILITY_TYPES: readonly ParkingFacilityType[] =
  RLS19_PARKING_FACILITY_RATES.map((row) => row.type);

function normalizeVocabulary(raw: string): string {
  return raw.trim().toLowerCase();
}

/** Rings of a single GeoJSON Polygon, or null when the geometry is not one. */
export function parkingPolygonRings(feature: ModelFeature): Point2D[][] | null {
  if (feature.geometry.type !== "Polygon") return null;

  const rings = feature.geometry.coordinates as unknown;
  if (!Array.isArray(rings) || rings.length === 0) return null;

  const parsed: Point2D[][] = [];
  for (const ring of rings) {
    if (!Array.isArray(ring)) return null;
    const points: Point2D[] = [];
    for (const position of ring) {
      if (
        !Array.isArray(position) ||
        typeof position[0] !== "number" ||
        typeof position[1] !== "number"
      ) {
        return null;
      }
      points.push({ x: position[0], y: position[1] });
    }
    parsed.push(points);
  }
  return parsed;
}

function signedRingArea(ring: Point2D[]): number {
  let sum = 0;
  for (let i = 0; i < ring.length - 1; i++) {
    const a = ring[i];
    const b = ring[i + 1];
    if (!a || !b) return 0;
    sum += a.x * b.y - b.x * a.y;
  }
  return sum / 2;
}

/** Exterior ring less every hole, clamped at zero. Mirrors geo.PolygonArea. */
export function polygonArea(rings: Point2D[][]): number {
  const exterior = rings[0];
  if (!exterior) return 0;

  let total = Math.abs(signedRingArea(exterior));
  for (const hole of rings.slice(1)) {
    total -= Math.abs(signedRingArea(hole));
  }
  return total < 0 ? 0 : total;
}

/**
 * Area centroid of the exterior ring, or null where it is undefined — a ring of
 * fewer than four points, one enclosing no area, or a non-finite result.
 * Mirrors geo.PolygonCentroid, holes ignored for the same reason.
 */
export function polygonCentroid(rings: Point2D[][]): Point2D | null {
  const ring = rings[0];
  if (!ring || ring.length < 4) return null;

  let doubleArea = 0;
  let cx = 0;
  let cy = 0;

  for (let i = 0; i < ring.length - 1; i++) {
    const a = ring[i];
    const b = ring[i + 1];
    if (!a || !b) return null;
    if (
      !Number.isFinite(a.x) ||
      !Number.isFinite(a.y) ||
      !Number.isFinite(b.x) ||
      !Number.isFinite(b.y)
    ) {
      return null;
    }
    const cross = a.x * b.y - b.x * a.y;
    doubleArea += cross;
    cx += (a.x + b.x) * cross;
    cy += (a.y + b.y) * cross;
  }

  if (Math.abs(doubleArea) < 1e-12) return null;

  const factor = 1 / (3 * doubleArea);
  const point = { x: cx * factor, y: cy * factor };
  return Number.isFinite(point.x) && Number.isFinite(point.y) ? point : null;
}

function parseLotType(feature: ModelFeature): ParkingLotType {
  const raw = getFeatureString(feature, PROP_PARKING_TYPE);
  if (raw === undefined) {
    throw new Error(
      `feature "${feature.id}": property "${PROP_PARKING_TYPE}" is required, expected one of ${RLS19_PARKING_LOT_TYPES.join(", ")}; it selects the Tabelle 6 row and has no default`,
    );
  }

  const name = normalizeVocabulary(raw);
  const match = RLS19_PARKING_LOT_TYPES.find((type) => type === name);
  if (match === undefined) {
    throw new Error(
      `feature "${feature.id}": property "${PROP_PARKING_TYPE}": unknown Parkplatztyp "${raw}", expected one of ${RLS19_PARKING_LOT_TYPES.join(", ")}`,
    );
  }
  return match;
}

function parseNumSpaces(feature: ModelFeature): number {
  const value = getFeatureNumber(feature, PROP_PARKING_NUM_SPACES);
  if (value === undefined) {
    throw new Error(
      `feature "${feature.id}": property "${PROP_PARKING_NUM_SPACES}" is required: the number of Stellplätze n has no default`,
    );
  }
  if (value < 1 || Math.trunc(value) !== value) {
    throw new Error(
      `feature "${feature.id}": property "${PROP_PARKING_NUM_SPACES}" must be an integer >= 1`,
    );
  }
  return value;
}

/**
 * Both movement rates N. A stated facility type seeds them from Tabelle 7 and
 * an explicit rate overrides its own period; with no facility type there is
 * nothing to seed from and both are required. An explicit 0 stays legal — it
 * means a period with no movements — which is why the seeds are nullable rather
 * than zero-defaulted.
 */
function parseMovementRates(feature: ModelFeature): {
  day: number | null;
  night: number | null;
} {
  let day: number | null = null;
  let night: number | null = null;

  const facilityRaw = getFeatureString(feature, PROP_PARKING_FACILITY_TYPE);
  if (facilityRaw !== undefined) {
    const name = normalizeVocabulary(facilityRaw);
    const row = RLS19_PARKING_FACILITY_RATES.find((r) => r.type === name);
    if (row === undefined) {
      throw new Error(
        `feature "${feature.id}": property "${PROP_PARKING_FACILITY_TYPE}": unknown Parkplatztyp "${facilityRaw}", expected one of ${RLS19_PARKING_FACILITY_TYPES.join(", ")}`,
      );
    }
    day = row.day;
    night = row.night;
  }

  const dayOverride = getFeatureNumber(feature, PROP_PARKING_MOVEMENTS_DAY);
  if (dayOverride !== undefined) day = dayOverride;

  const nightOverride = getFeatureNumber(feature, PROP_PARKING_MOVEMENTS_NIGHT);
  if (nightOverride !== undefined) night = nightOverride;

  for (const [key, rate] of [
    [PROP_PARKING_MOVEMENTS_DAY, day],
    [PROP_PARKING_MOVEMENTS_NIGHT, night],
  ] as const) {
    if (rate === null) {
      throw new Error(
        `feature "${feature.id}": property "${key}" is required unless "${PROP_PARKING_FACILITY_TYPE}" states a Tabelle 7 Parkplatztyp; state 0 explicitly for a period with no movements`,
      );
    }
  }

  return { day, night };
}

/**
 * Parkplatz sources in model feature order, plus every polygon vertex.
 *
 * The vertices are what the receiver grid needs: a lot is an extended
 * footprint, and padding a grid around its centroid alone would put the whole
 * grid inside the source.
 */
export function buildParkingSources(features: ModelFeature[]): {
  sources: ParkingSource[];
  extent: Point2D[];
} {
  const sources: ParkingSource[] = [];
  const extent: Point2D[] = [];

  features.forEach((feature, index) => {
    if (feature.kind !== "source" || feature.sourceType !== "area") return;

    const rings = parkingPolygonRings(feature);
    if (rings === null) {
      throw new Error(
        `feature "${feature.id}": a parking source must be a single Polygon; model each Teilfläche (§3.4, Bild 10) as its own feature with its own ${PROP_PARKING_NUM_SPACES}`,
      );
    }

    const center = polygonCentroid(rings);
    if (center === null) {
      throw new Error(
        `feature "${feature.id}": parking polygon encloses no area, so it has no centroid to propagate from`,
      );
    }

    const rates = parseMovementRates(feature);

    sources.push({
      id:
        feature.id.trim() || `rls19-parking-${String(index).padStart(3, "0")}`,
      center,
      elevation_m: getFeatureNumber(feature, "elevation_m") ?? 0,
      area_m2: polygonArea(rings),
      num_spaces: parseNumSpaces(feature),
      parking_type: parseLotType(feature),
      movements_per_space_day: rates.day,
      movements_per_space_night: rates.night,
    });

    for (const ring of rings) extent.push(...ring);
  });

  return { sources, extent };
}
