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
  geometry: { type: "Point"; coordinates: Position };
}

/** Create a new receiver ID */
export function createReceiverId(): string {
  return crypto.randomUUID();
}

/** Calculation area polygon that constrains the receiver grid extent */
export interface CalcArea {
  geometry: { type: "Polygon"; coordinates: Position[][] };
}
