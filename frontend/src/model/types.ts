/** Feature kind — matches Go backend's normalized model */
export type FeatureKind = "source" | "building" | "barrier" | "ground-zone";

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

/** A finding that interpolates nothing. */
export type NoIssueParams = Record<string, never>;

/**
 * Every finding `validate.ts` can produce, and the values its sentence needs.
 *
 * This map is the vocabulary. A finding is a code plus parameters and carries
 * no prose, because prose is a locale's business: `validation-message.ts`
 * turns the pair into a sentence and is the only place that knows any. The map
 * is what makes both halves total — a code missing from here cannot be pushed,
 * and a code missing from the renderer's `switch` is a compile error.
 *
 * The parameter values are deliberately narrow. A name the user must type back
 * — a property key like `traffic_day_lkw1`, an enum member like `asphalt` — is
 * carried raw and rendered raw, which is the rule Phase C settled for parameter
 * labels: the raw name stays on screen because it is what `--param` takes.
 * Nothing here is a number the reader would expect formatted for their locale;
 * `parts` is a count of polygon rings in a refusal, not a measurement.
 */
export interface ValidationIssueParams {
  "model.empty": NoIssueParams;

  "feature.id.duplicate": NoIssueParams;
  "receiver.id.duplicate": NoIssueParams;

  "source.type.required": NoIssueParams;
  "source.geometry.mismatch": { geometry: string; sourceType: string };

  "building.height.required": NoIssueParams;
  "building.height.invalid": NoIssueParams;
  "building.geometry.invalid": NoIssueParams;

  "barrier.height.required": NoIssueParams;
  "barrier.height.invalid": NoIssueParams;
  "barrier.geometry.invalid": NoIssueParams;

  "groundzone.factor.required": NoIssueParams;
  "groundzone.factor.invalid": NoIssueParams;
  "groundzone.geometry.invalid": NoIssueParams;

  "receiver.coordinates.invalid": NoIssueParams;
  "receiver.height.invalid": NoIssueParams;

  "source.rls19.surface_type.invalid": { value: string };
  "source.rls19.junction_type.invalid": { value: string };
  "source.rls19.gradient.invalid": { min: number; max: number };
  "source.rls19.junction_distance.invalid": NoIssueParams;
  "source.rls19.reflection_surcharge.invalid": NoIssueParams;
  "source.rls19.road_speed.invalid": NoIssueParams;
  "source.rls19.speed.invalid": { field: string };
  "source.rls19.traffic.invalid": { field: string };
  "source.rls19.review_required": NoIssueParams;

  "source.rls19.parking.geometry.multipart": { parts: number; field: string };
  "source.rls19.parking.num_spaces.missing": { field: string };
  "source.rls19.parking.num_spaces.invalid": { field: string };
  "source.rls19.parking.parking_type.missing": {
    field: string;
    expected: string;
  };
  "source.rls19.parking.parking_type.invalid": { value: string };
  "source.rls19.parking.facility_type.invalid": {
    field: string;
    value: string;
    expected: string;
  };
  "source.rls19.parking.movements.missing": {
    field: string;
    facilityField: string;
  };
  "source.rls19.parking.movements.invalid": { field: string };
}

/** The codes `ValidationIssueParams` declares, as a type. */
export type ValidationCode = keyof ValidationIssueParams;

/**
 * A single validation finding.
 *
 * A union over the codes rather than one shape with a loose `params` bag, so
 * that a construction site naming the wrong parameter — or forgetting one —
 * does not compile. `params` is always present; a finding that interpolates
 * nothing carries `{}`.
 */
export type ValidationIssue = {
  [Code in ValidationCode]: {
    level: IssueSeverity;
    code: Code;
    featureId: string;
    params: ValidationIssueParams[Code];
  };
}[ValidationCode];

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
