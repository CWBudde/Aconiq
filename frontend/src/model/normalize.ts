import type {
  CalcArea,
  FeatureKind,
  GeoJSONFeature,
  GeoJSONFeatureCollection,
  Geometry,
  GeometryType,
  ModelFeature,
  ModelReceiver,
  Position,
  SourceType,
} from "./types";
import { createFeatureId, DEFAULT_RECEIVER_HEIGHT_M } from "./types";

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

/** Everything the model store holds, read out of one v1 FeatureCollection. */
export interface NormalizeModelResult {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  skipped: SkippedFeature[];
  /**
   * The CRS the collection declares, or null when it declares none.
   *
   * Null is not EPSG:4326. It means "this file says nothing", which the store
   * answers differently for a replacement than for a merge.
   */
  crs: string | null;
}

/**
 * The CRS a FeatureCollection declares, as an `EPSG:<code>` string.
 *
 * `aconiq import` writes the OGC named-CRS member
 * (`{type: "name", properties: {name: "EPSG:25832"}}` — see
 * `modelgeojson.Model.ToFeatureCollection`), so a file the CLI produced round
 * trips through here. The URN spelling `urn:ogc:def:crs:EPSG::25832` is
 * accepted too, because that is what most GIS tools emit.
 *
 * Dropping this member is not harmless. The coordinates in a projected file are
 * metric eastings and northings; labelling them EPSG:4326 hands them to the
 * kernel's `transform` as longitude and latitude, which refuses them as out of
 * range — and one that happened to fall inside the lon/lat range would instead
 * be projected as though it really were degrees.
 *
 * Anything unrecognised returns null rather than a guess: the store's fallback
 * is at least explicit, where a half-parsed name would not be.
 */
export function readCollectionCRS(
  collection: GeoJSONFeatureCollection,
): string | null {
  const properties = collection.crs?.["properties"];
  if (typeof properties !== "object" || properties === null) return null;

  const name = (properties as { name?: unknown }).name;
  if (typeof name !== "string") return null;

  const code = /^(?:urn:ogc:def:crs:)?EPSG::?(\d+)$/i.exec(name.trim())?.[1];
  return code === undefined ? null : `EPSG:${code}`;
}

/**
 * One feature's place in the model. `kind` here is the outcome, not the
 * GeoJSON `kind` property: a feature that declared a kind the model knows but
 * failed its own rules still lands in `skipped`.
 */
type ModelEntry =
  | { kind: "feature"; feature: ModelFeature }
  | { kind: "receiver"; receiver: ModelReceiver }
  | { kind: "calc-area"; area: CalcArea }
  | { kind: "skipped"; reason: string };

/**
 * The full model: features, receivers and the calculation area — the only way
 * a FeatureCollection is read.
 *
 * There used to be a second, feature-only reader beside it, whose `VALID_KINDS`
 * covered the three kinds the map draws and reported a receiver as an unknown
 * kind. Anything reading a project through that one dropped every placed
 * receiver and the drawn area, and the first save afterwards wrote the loss
 * back into the project. It is gone rather than documented: a comment asking
 * callers not to use it would have left the entry point in the codebase.
 */
export function normalizeModelGeoJSON(
  collection: GeoJSONFeatureCollection,
): NormalizeModelResult {
  const features: ModelFeature[] = [];
  const receivers: ModelReceiver[] = [];
  const skipped: SkippedFeature[] = [];
  let calcArea: CalcArea | null = null;

  forEachEntry(collection, (index, entry) => {
    switch (entry.kind) {
      case "feature":
        features.push(entry.feature);
        break;
      case "receiver":
        receivers.push(entry.receiver);
        break;
      case "calc-area":
        // At most one, and the first wins — the same rule the backend
        // validator enforces as `model.calc_area.duplicate`. A model carrying
        // two is one the UI cannot produce, so this only ever fires for a
        // file written by something else.
        if (calcArea === null) {
          calcArea = entry.area;
        } else {
          skipped.push({
            index,
            reason: `feature[${String(index)}]: the model holds at most one calculation area`,
          });
        }
        break;
      case "skipped":
        skipped.push({ index, reason: entry.reason });
        break;
    }
  });

  return {
    features,
    receivers,
    calcArea,
    skipped,
    crs: readCollectionCRS(collection),
  };
}

function forEachEntry(
  collection: GeoJSONFeatureCollection,
  visit: (index: number, entry: ModelEntry) => void,
): void {
  for (let i = 0; i < collection.features.length; i++) {
    const raw = collection.features[i];
    if (!raw) continue;
    visit(i, normalizeEntry(raw, i));
  }
}

function unknownKindReason(index: number, kind: string): string {
  return `feature[${String(index)}]: unknown kind "${kind}"`;
}

function normalizeEntry(raw: GeoJSONFeature, index: number): ModelEntry {
  const props = raw.properties;
  const rawKind = props["kind"];
  const kindRaw = (typeof rawKind === "string" ? rawKind : "")
    .toLowerCase()
    .trim();

  if (kindRaw === "receiver") return normalizeReceiver(raw, index);
  if (kindRaw === "calc-area") return normalizeCalcArea(raw, index);

  if (!VALID_KINDS.has(kindRaw)) {
    return { kind: "skipped", reason: unknownKindReason(index, kindRaw) };
  }

  const kind = kindRaw as FeatureKind;
  const geomType = raw.geometry.type;

  if (!VALID_GEOM_TYPES.has(geomType)) {
    return {
      kind: "skipped",
      reason: `feature[${String(index)}]: unsupported geometry type "${geomType}"`,
    };
  }

  const normalizedProps = normalizeProperties(props);

  const feature: ModelFeature = {
    id: resolveFeatureID(raw),
    kind,
    ...(normalizedProps !== undefined && { properties: normalizedProps }),
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

  return { kind: "feature", feature };
}

/**
 * A receiver, mirroring the backend's rule: the geometry must be a Point.
 *
 * A missing or unusable height falls back to the UI's own default instead of
 * dropping the feature. Dropping is exactly the loss this normalizer exists to
 * fix — a receiver the user placed on the map is not something to discard over
 * a property a writer left out.
 *
 * The properties are kept for the same reason, and the same way a feature's
 * are: a receiver carries `bimschv16_area_category`, which is the only thing
 * that makes it assessable, and reading it into a point alone loses it on the
 * next save.
 */
function normalizeReceiver(raw: GeoJSONFeature, index: number): ModelEntry {
  if (raw.geometry.type !== "Point") {
    return {
      kind: "skipped",
      reason: `feature[${String(index)}]: receiver geometry must be Point, got "${raw.geometry.type}"`,
    };
  }

  const height = Number(raw.properties["height_m"]);
  const normalizedProps = normalizeProperties(raw.properties);

  return {
    kind: "receiver",
    receiver: {
      id: resolveFeatureID(raw),
      heightM:
        Number.isFinite(height) && height > 0
          ? height
          : DEFAULT_RECEIVER_HEIGHT_M,
      ...(normalizedProps !== undefined && { properties: normalizedProps }),
      geometry: {
        type: "Point",
        coordinates: raw.geometry.coordinates as Position,
      },
    },
  };
}

/**
 * The calculation area, mirroring the backend's rule: Polygon and not
 * MultiPolygon. The backend refuses a multi-part area rather than reducing it
 * to its envelope, because a disjoint one would silently become a single bbox
 * spanning ground the user drew around; accepting one here would mean the map
 * showed an area no run could honour.
 */
function normalizeCalcArea(raw: GeoJSONFeature, index: number): ModelEntry {
  if (raw.geometry.type !== "Polygon") {
    return {
      kind: "skipped",
      reason: `feature[${String(index)}]: calculation area geometry must be Polygon, got "${raw.geometry.type}"`,
    };
  }

  // The stored id is kept when the project has one, and only then: an area the
  // user draws carries none, and `to-geojson.ts` derives a stable id for it at
  // emit time. Minting one here instead would put a fresh UUID on every
  // hydrated area and rename it in the project on the next save.
  const id = explicitFeatureID(raw);

  return {
    kind: "calc-area",
    area: {
      ...(id === "" ? {} : { id }),
      geometry: {
        type: "Polygon",
        coordinates: raw.geometry.coordinates as Position[][],
      },
    },
  };
}

/**
 * The id the backend would resolve for this feature, resolved the same way.
 *
 * `Model.ToFeatureCollection` writes the id into `properties.id` and leaves
 * the GeoJSON `id` member unset, and `featureID` in the same Go file reads
 * `properties.id` first and falls back to the member. Reading only the member
 * — as this did — meant every feature hydrated from a project arrived without
 * an id, got a fresh UUID, and the first save afterwards rewrote every id the
 * project had.
 */
function resolveFeatureID(raw: GeoJSONFeature): string {
  const explicit = explicitFeatureID(raw);

  return explicit === "" ? createFeatureId() : explicit;
}

/**
 * The id the feature carries, or `""` when it carries none.
 *
 * Split out of `resolveFeatureID` for the calculation area, which must be able
 * to tell "the project named this" from "nothing named it" rather than take a
 * minted id it would then write back.
 */
function explicitFeatureID(raw: GeoJSONFeature): string {
  const fromProperties = stringifyID(raw.properties["id"]);
  if (fromProperties !== "") return fromProperties;

  return stringifyID(raw.id);
}

/** `stringifyID` in `backend/internal/geo/modelgeojson/normalize.go`. */
function stringifyID(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") return value.trim();
  if (typeof value === "number")
    return Number.isFinite(value) ? String(value) : "";
  return "";
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

function normalizeProperties(
  props: Record<string, unknown>,
): Record<string, unknown> | undefined {
  const normalizedEntries = Object.entries(props).filter(
    ([key]) => key.trim() !== "",
  );
  if (normalizedEntries.length === 0) {
    return undefined;
  }

  return Object.fromEntries(normalizedEntries);
}
