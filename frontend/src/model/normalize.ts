import type {
  FeatureKind,
  GeoJSONFeature,
  GeoJSONFeatureCollection,
  Geometry,
  GeometryType,
  ModelFeature,
  SourceType,
} from "./types";
import { createFeatureId } from "./types";

const VALID_KINDS = new Set<string>(["source", "building", "barrier"]);
const VALID_SOURCE_TYPES = new Set<string>(["point", "line", "area"]);
const VALID_GEOM_TYPES = new Set<string>([
  "Point",
  "MultiPoint",
  "LineString",
  "MultiLineString",
  "Polygon",
  "MultiPolygon",
]);

interface SkippedFeature {
  index: number;
  reason: string;
}

const METERS_PER_LEVEL = 3;

export interface NormalizeResult {
  features: ModelFeature[];
  skipped: SkippedFeature[];
}

export function normalizeGeoJSON(
  collection: GeoJSONFeatureCollection,
): NormalizeResult {
  const features: ModelFeature[] = [];
  const skipped: SkippedFeature[] = [];

  for (let i = 0; i < collection.features.length; i++) {
    const raw = collection.features[i];
    if (!raw) continue;
    const result = normalizeFeature(raw, i);
    if (result.ok) {
      features.push(result.feature);
    } else {
      skipped.push({ index: i, reason: result.reason });
    }
  }

  return { features, skipped };
}

type NormalizeFeatureResult =
  | { ok: true; feature: ModelFeature }
  | { ok: false; reason: string };

function normalizeFeature(
  raw: GeoJSONFeature,
  index: number,
): NormalizeFeatureResult {
  const props = raw.properties;
  const rawKind = props["kind"];
  const kindRaw = (typeof rawKind === "string" ? rawKind : "")
    .toLowerCase()
    .trim();

  if (!VALID_KINDS.has(kindRaw)) {
    return {
      ok: false,
      reason: `feature[${String(index)}]: unknown kind "${kindRaw}"`,
    };
  }

  const kind = kindRaw as FeatureKind;
  const geomType = raw.geometry.type;

  if (!VALID_GEOM_TYPES.has(geomType)) {
    return {
      ok: false,
      reason: `feature[${String(index)}]: unsupported geometry type "${geomType}"`,
    };
  }

  const id = raw.id != null ? String(raw.id) : createFeatureId();

  const feature: ModelFeature = {
    id,
    kind,
    geometry: {
      type: geomType as GeometryType,
      coordinates: raw.geometry.coordinates as Geometry["coordinates"],
    },
  };

  if (kind === "source") {
    const rawSt = props["source_type"];
    const st = (typeof rawSt === "string" ? rawSt : "").toLowerCase().trim();
    if (VALID_SOURCE_TYPES.has(st)) {
      feature.sourceType = st as SourceType;
    }
  }

  if (kind === "building" || kind === "barrier") {
    const h = inferHeightMeters(kind, props);
    if (Number.isFinite(h) && h > 0) {
      feature.heightM = h;
    }
  }

  return { ok: true, feature };
}

function inferHeightMeters(
  kind: FeatureKind,
  props: Record<string, unknown>,
): number {
  const explicit = Number(props["height_m"]);
  if (Number.isFinite(explicit) && explicit > 0) {
    return explicit;
  }

  const height = parseHeightLike(props["height"]);
  if (Number.isFinite(height) && height > 0) {
    return height;
  }

  if (kind === "building") {
    const levels = Number(props["building:levels"]);
    if (Number.isFinite(levels) && levels > 0) {
      return levels * METERS_PER_LEVEL;
    }
    if (typeof props["building"] === "string" && props["building"] !== "") {
      return 9;
    }
  }

  if (kind === "barrier") {
    if (typeof props["barrier"] === "string" && props["barrier"] !== "") {
      return 2;
    }
  }

  return Number.NaN;
}

function parseHeightLike(value: unknown): number {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value !== "string") {
    return Number.NaN;
  }
  const normalized = value.replace(/\s*m$/i, "").trim();
  return Number(normalized);
}
