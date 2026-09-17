/** Feature kind — matches Go backend's normalized model */
export type FeatureKind = "source" | "building" | "barrier";

/** Source geometry subtype */
export type SourceType = "point" | "line" | "area";

/** GeoJSON geometry types we support */
export type GeometryType =
  | "Point"
  | "MultiPoint"
  | "LineString"
  | "MultiLineString"
  | "Polygon"
  | "MultiPolygon";

/** A position is [lng, lat] or [x, y] */
export type Position = [number, number];

/** GeoJSON geometry object */
export interface Geometry {
  type: GeometryType;
  coordinates: Position | Position[] | Position[][] | Position[][][];
}

/** A normalized model feature (mirrors Go Feature struct) */
export interface ModelFeature {
  id: string;
  kind: FeatureKind;
  sourceType?: SourceType;
  heightM?: number;
  properties?: Record<string, unknown>;
  geometry: Geometry;
}

/** Validation issue severity */
export type IssueSeverity = "error" | "warning";

/** A single validation finding */
export interface ValidationIssue {
  level: IssueSeverity;
  code: string;
  featureId: string;
  message: string;
}

/** Full validation report */
export interface ValidationReport {
  valid: boolean;
  errors: ValidationIssue[];
  warnings: ValidationIssue[];
  checkedAt: string;
}

/** GeoJSON FeatureCollection for import/export */
export interface GeoJSONFeatureCollection {
  type: "FeatureCollection";
  features: GeoJSONFeature[];
  crs?: Record<string, unknown>;
}

export interface GeoJSONFeature {
  type: "Feature";
  id?: string | number;
  properties: Record<string, unknown>;
  geometry: {
    type: string;
    coordinates: unknown;
  };
}

/** Create a new feature ID */
export function createFeatureId(): string {
  return crypto.randomUUID();
}

/** Check if a geometry type is compatible with a source type */
export function isGeometryCompatible(
  geometryType: GeometryType,
  sourceType: SourceType,
): boolean {
  switch (sourceType) {
    case "point":
      return geometryType === "Point" || geometryType === "MultiPoint";
    case "line":
      return (
        geometryType === "LineString" || geometryType === "MultiLineString"
      );
    case "area":
      return geometryType === "Polygon" || geometryType === "MultiPolygon";
  }
}

/** An explicit receiver point for noise calculation */
export interface ModelReceiver {
  id: string;
  heightM: number;
  /**
   * The properties the receiver arrived with, kept the way {@link ModelFeature}
   * keeps a feature's.
   *
   * A receiver is not only a point: `docs/geojson-schema-v1.md` gives it
   * `bimschv16_area_category`, and `assessment/bimschv16` refuses to assess a
   * receiver that carries none. Holding id, height and geometry alone meant an
   * import read the category and dropped it, and the next save wrote a receiver
   * the assessment then skipped as missing its area category.
   */
  properties?: Record<string, unknown>;
  geometry: { type: "Point"; coordinates: Position };
}

/**
 * The height the UI gives a receiver when nothing else says. It is the new
 * feature dialog's fallback, and it is also what the model normalizer defaults
 * a receiver to when a file carries no usable `height_m` — dropping a receiver
 * someone placed on the map over a missing property is the worse answer.
 */
export const DEFAULT_RECEIVER_HEIGHT_M = 4;

/** Create a new receiver ID */
export function createReceiverId(): string {
  return crypto.randomUUID();
}

/** Calculation area polygon that constrains the receiver grid extent */
export interface CalcArea {
  /**
   * The id the project already stores for this area, when it has one.
   *
   * Absent for an area the user just drew — `to-geojson.ts` mints a stable one
   * at emit time. Present for one read back out of the project, so that saving
   * a hydrated model does not rename a feature the project already named.
   */
  id?: string;
  /**
   * The properties the area arrived with, kept the way {@link ModelFeature}
   * and {@link ModelReceiver} keep theirs.
   *
   * An area is more than its outline to whatever wrote it. `aconiq import
   * --soundplan` puts `soundplan_base_elevation_m` here, read off the first
   * vertex of the bundle's `CalcArea.geo`; holding id and geometry alone meant
   * the first save from the map wrote an area without it, silently and with no
   * recovery short of re-importing.
   */
  properties?: Record<string, unknown>;
  geometry: { type: "Polygon"; coordinates: Position[][] };
}
