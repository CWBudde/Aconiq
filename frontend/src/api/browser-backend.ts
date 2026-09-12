/* eslint-disable @typescript-eslint/require-await --
   These methods implement the same async `Backend` interface as the HTTP
   client. Several are synchronous in-memory lookups, but they must stay
   `async` so that a thrown error surfaces as a rejected promise, exactly as
   it does on the HTTP path. */
import type {
  Backend,
  ModelSaveResult,
  OsmImportRequest,
  RunSpec,
} from "./backend";
import type {
  ArtifactRef,
  HealthResponse,
  ModelSaveRequest,
  ProjectStatusResponse,
  RasterMetadata,
  ReceiverTable,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import type { GeoJSONFeatureCollection, ModelFeature } from "@/model/types";
import { useModelStore } from "@/model/model-store";
import {
  getFeatureNumber,
  getFeatureString,
  RLS19_SURFACE_TYPES,
} from "@/model/source-acoustics";
import type { Point2D } from "@/model/geometry";
import { buildParkingSources, polygonParts } from "@/model/rls19-parking";
import { getKernel } from "@/wasm/kernel";
import type {
  Barrier,
  Building,
  ComputeRequest,
  PointReceiver,
  ReceiverOutput,
  RoadSource,
} from "@/wasm/types";
import {
  BrowserStorageError,
  isBrowserStorageError,
  loadPersistedState,
  savePersistedState,
} from "./browser-storage";

/**
 * Where browser-mode runs lived before they moved to IndexedDB. Read once, on
 * the first load that finds IndexedDB empty, and removed after a successful
 * copy; never written again.
 */
const LEGACY_STORAGE_KEY = "aconiq.browser_backend.v1";
/**
 * The shape of the persisted document is `{ version, state }`. Bump the
 * version when `BrowserBackendState` changes incompatibly, and teach
 * `decodePersisted` the old shape or let it start fresh — a document at an
 * unknown version is never guessed at.
 */
export const PERSISTED_STATE_VERSION = 1;
/**
 * Runs kept per browser profile, newest first. Every run stores its receiver
 * table twice (JSON and CSV) plus an export bundle on request, so an unbounded
 * list would eventually exhaust the origin's quota on a run that had already
 * completed. Twenty runs of a typical model stay well inside it.
 */
export const MAX_STORED_RUNS = 20;
const DEFAULT_PROJECT_ID = "browser-project";
const DEFAULT_PROJECT_NAME = "Aconiq Browser Project";
const DEFAULT_PROJECT_PATH = "browser://local-storage";
const DEFAULT_CRS = "WGS84 / web map";
const DEFAULT_OSM_ENDPOINT = "https://overpass-api.de/api/interpreter";

type StoredArtifactContent = {
  mimeType: string;
  kind: string;
  encoding: "json" | "text";
  value: unknown;
};

type StoredRun = {
  run: RunSummary;
  log: RunLog;
  artifacts: Record<string, StoredArtifactContent>;
};

type BrowserBackendState = {
  projectId: string;
  projectName: string;
  projectPath: string;
  crs: string;
  runs: StoredRun[];
};

type OverpassPoint = {
  lat: number;
  lon: number;
};

type OverpassWay = {
  type: "way";
  id: number;
  tags?: Record<string, string>;
  geometry?: OverpassPoint[];
};

type OverpassResponse = {
  elements?: Array<OverpassWay | Record<string, unknown>>;
};

const urlCache = new Map<string, string>();

export const BROWSER_STANDARDS: StandardDescriptor[] = [
  {
    id: "rls19-road",
    description:
      "RLS-19 road noise computed locally in the browser via WebAssembly.",
    // The browser kernel is the same Go module compiled to WASM, so it sits at
    // the same evidence tier the API registry reports for it.
    evidence_tier: "normative",
    default_version: "2019",
    versions: [
      {
        name: "2019",
        default_profile: "default",
        profiles: [
          {
            name: "default",
            supported_source_types: ["line", "area"],
            supported_indicators: ["LrDay", "LrNight"],
            parameters: [
              {
                name: "grid_resolution_m",
                kind: "float",
                required: true,
                default_value: "10",
                description: "Receiver grid spacing in map units",
                min: 0.001,
              },
              {
                name: "grid_padding_m",
                kind: "float",
                required: true,
                default_value: "20",
                description: "Padding around source extent in map units",
                min: 0,
              },
              {
                name: "receiver_height_m",
                kind: "float",
                required: true,
                default_value: "4",
                description: "Receiver height above ground",
                min: 0,
              },
              {
                name: "surface_type",
                kind: "string",
                required: true,
                default_value: "SMA",
                description: "Default road surface type",
                // The whole RLS19_SURFACE_TYPES vocabulary, not a subset: a
                // surface missing here is one a browser-mode run cannot select,
                // and each carries its own Tabelle 4a row.
                enum: [...RLS19_SURFACE_TYPES],
              },
              {
                name: "speed_pkw_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "speed_lkw1_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "speed_lkw2_kph",
                kind: "float",
                required: true,
                default_value: "80",
                min: 0.001,
              },
              {
                name: "speed_krad_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "gradient_percent",
                kind: "float",
                required: true,
                default_value: "0",
                min: -12,
                max: 12,
              },
              {
                name: "traffic_day_pkw",
                kind: "float",
                required: true,
                default_value: "900",
                min: 0,
              },
              {
                name: "traffic_day_lkw1",
                kind: "float",
                required: true,
                default_value: "40",
                min: 0,
              },
              {
                name: "traffic_day_lkw2",
                kind: "float",
                required: true,
                default_value: "60",
                min: 0,
              },
              {
                name: "traffic_day_krad",
                kind: "float",
                required: true,
                default_value: "10",
                min: 0,
              },
              {
                name: "traffic_night_pkw",
                kind: "float",
                required: true,
                default_value: "200",
                min: 0,
              },
              {
                name: "traffic_night_lkw1",
                kind: "float",
                required: true,
                default_value: "10",
                min: 0,
              },
              {
                name: "traffic_night_lkw2",
                kind: "float",
                required: true,
                default_value: "20",
                min: 0,
              },
              {
                name: "traffic_night_krad",
                kind: "float",
                required: true,
                default_value: "2",
                min: 0,
              },
              {
                name: "segment_length_m",
                kind: "float",
                required: true,
                default_value: "1",
                min: 0.001,
              },
              {
                name: "min_distance_m",
                kind: "float",
                required: true,
                default_value: "3",
                min: 0.001,
              },
            ],
          },
        ],
      },
    ],
  },
];

function initialState(): BrowserBackendState {
  return {
    projectId: DEFAULT_PROJECT_ID,
    projectName: DEFAULT_PROJECT_NAME,
    projectPath: DEFAULT_PROJECT_PATH,
    crs: DEFAULT_CRS,
    runs: [],
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStoredRun(value: unknown): value is StoredRun {
  if (!isRecord(value)) return false;
  const { run, log, artifacts } = value;
  return (
    isRecord(run) &&
    typeof run["id"] === "string" &&
    typeof run["started_at"] === "string" &&
    Array.isArray(run["artifacts"]) &&
    isRecord(log) &&
    isRecord(artifacts)
  );
}

/**
 * Turns an untrusted stored state into a usable one. Every field may be
 * absent — a document written by an older build is the normal case, not the
 * exception — so each falls back to its default, and a run entry that is not
 * a run is dropped rather than left to throw later from inside a page.
 */
function decodeState(value: unknown): BrowserBackendState {
  if (!isRecord(value)) {
    throw new BrowserStorageError(
      "corrupt",
      "Stored browser-mode state is not an object",
    );
  }
  const defaults = initialState();
  const text = (key: keyof BrowserBackendState, fallback: string): string => {
    const field = value[key];
    return typeof field === "string" ? field : fallback;
  };
  const runs = Array.isArray(value["runs"]) ? value["runs"] : [];
  return {
    projectId: text("projectId", defaults.projectId),
    projectName: text("projectName", defaults.projectName),
    projectPath: text("projectPath", defaults.projectPath),
    crs: text("crs", defaults.crs),
    runs: runs.filter(isStoredRun),
  };
}

function decodePersisted(value: unknown): BrowserBackendState {
  if (!isRecord(value)) {
    throw new BrowserStorageError(
      "corrupt",
      "Stored browser-mode document is not an object",
    );
  }
  if (value["version"] !== PERSISTED_STATE_VERSION) {
    throw new BrowserStorageError(
      "corrupt",
      `Stored browser-mode document has version ${String(value["version"])}, expected ${String(PERSISTED_STATE_VERSION)}`,
    );
  }
  return decodeState(value["state"]);
}

/**
 * Copies the pre-IndexedDB localStorage document across, once. A copy that
 * cannot be written stays in localStorage and is offered again on the next
 * load; a copy that cannot be read is left alone too, so nothing is destroyed
 * on the way.
 */
async function migrateLegacyState(): Promise<BrowserBackendState | null> {
  let raw: string | null;
  try {
    raw = window.localStorage.getItem(LEGACY_STORAGE_KEY);
  } catch {
    return null;
  }
  if (raw === null) return null;
  let legacy: BrowserBackendState;
  try {
    const parsed: unknown = JSON.parse(raw);
    legacy = decodeState(parsed);
  } catch (error) {
    console.warn(
      "Ignoring unreadable browser-mode runs left in localStorage",
      error,
    );
    return null;
  }
  try {
    await savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: legacy,
    });
    window.localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch (error) {
    console.warn(
      "Browser-mode runs could not be moved from localStorage to IndexedDB; keeping the localStorage copy",
      error,
    );
  }
  return legacy;
}

/**
 * A corrupt or unavailable store never blocks the UI: the backend starts
 * fresh in memory and warns. The corrupt document is left in place until the
 * next successful write replaces it, so a bug in the guard cannot erase data
 * a later build could still have read.
 */
async function loadState(): Promise<BrowserBackendState> {
  let stored: unknown;
  try {
    stored = await loadPersistedState();
  } catch (error) {
    console.warn(
      "Browser-mode runs cannot be persisted in this browser; they will be kept in memory for this session only",
      error,
    );
    return initialState();
  }
  if (stored === null) {
    return (await migrateLegacyState()) ?? initialState();
  }
  try {
    return decodePersisted(stored);
  } catch (error) {
    console.warn(
      "Ignoring unreadable stored browser-mode runs; the next completed run replaces them",
      error,
    );
    return initialState();
  }
}

/**
 * In-memory copy of the persisted state. Every async method awaits
 * `ensureLoaded()` before touching it, and every write goes through
 * `persist()`, which updates this copy before the store — so the cache is
 * never behind what the UI was just told, even if the store is.
 */
let state: BrowserBackendState | null = null;
let loadPromise: Promise<BrowserBackendState> | null = null;

function ensureLoaded(): Promise<BrowserBackendState> {
  if (state !== null) return Promise.resolve(state);
  loadPromise ??= loadState().then(
    (loaded) => {
      state = loaded;
      return loaded;
    },
    (error: unknown) => {
      loadPromise = null;
      throw error;
    },
  );
  return loadPromise;
}

async function persist(next: BrowserBackendState): Promise<void> {
  state = next;
  pruneURLCache(next);
  await savePersistedState({ version: PERSISTED_STATE_VERSION, state: next });
}

/**
 * Stores a new or updated run. On a quota failure the oldest *other* run is
 * evicted and the write retried once; if that still fails the run stays in
 * memory — its results remain viewable for this session — and the caller gets
 * an error whose message says so, because the computation has already
 * succeeded and the dialogs render `error.message`.
 */
async function persistRun(
  current: BrowserBackendState,
  storedRun: StoredRun,
  what: "run" | "export",
): Promise<void> {
  const next = setRun(current, storedRun);
  try {
    await persist(next);
    return;
  } catch (error) {
    if (!isBrowserStorageError(error, "quota")) throw storeFailure(what, error);
  }
  const evicted = evictOldestRun(next, storedRun.run.id);
  if (evicted !== null) {
    try {
      await persist(evicted);
      return;
    } catch (error) {
      if (!isBrowserStorageError(error, "quota")) {
        throw storeFailure(what, error);
      }
    }
    // The eviction never reached the store, so it must not reach the list
    // either: the user would see a run vanish alongside an error about a
    // different one.
    state = next;
  }
  throw new BrowserStorageError(
    "quota",
    `The ${what} completed but could not be stored: the browser's storage quota is exhausted. Its results stay available until this page is reloaded; older runs may need deleting to free space.`,
  );
}

function storeFailure(what: "run" | "export", error: unknown): Error {
  if (!isBrowserStorageError(error)) {
    return error instanceof Error ? error : new Error(String(error));
  }
  return new BrowserStorageError(
    error.reason,
    `The ${what} completed but could not be stored (${error.message}). Its results stay available until this page is reloaded.`,
    { cause: error },
  );
}

/** Drops the oldest run other than `keepId`; `null` when there is none. */
function evictOldestRun(
  current: BrowserBackendState,
  keepId: string,
): BrowserBackendState | null {
  // `setRun` keeps the list newest first, so the victim is the last entry
  // that is not the run being written.
  const runs = [...current.runs];
  for (let index = runs.length - 1; index >= 0; index -= 1) {
    if (runs[index]?.run.id === keepId) continue;
    runs.splice(index, 1);
    return { ...current, runs };
  }
  return null;
}

/**
 * Forgets the in-memory state so the next call loads from the store again.
 * The object-URL cache is deliberately kept: it is keyed by artifact id, and
 * an id never names two payloads, so a URL minted before the reset is still
 * right after it.
 */
export function resetBrowserBackendForTests(): void {
  state = null;
  loadPromise = null;
}

/**
 * Revokes only the object URLs whose artifact no longer exists.
 *
 * Revoking the whole cache on every write broke pages that were still showing
 * a URL: starting a second run invalidated the `<iframe src>` of the report the
 * user was reading, and the "open in browser" link with it. Artifact ids embed
 * the run id and an export timestamp, so an id never names two different
 * payloads and a surviving URL can never be stale.
 */
function pruneURLCache(state: BrowserBackendState): void {
  const liveArtifactIds = new Set<string>();
  for (const storedRun of state.runs) {
    for (const artifactId of Object.keys(storedRun.artifacts)) {
      liveArtifactIds.add(artifactId);
    }
  }
  for (const [artifactId, url] of urlCache) {
    if (liveArtifactIds.has(artifactId)) continue;
    URL.revokeObjectURL(url);
    urlCache.delete(artifactId);
  }
}

function nowISO(): string {
  return new Date().toISOString();
}

function formatRunIndex(index: number): string {
  return String(index + 1).padStart(4, "0");
}

function parseNumber(
  params: Record<string, string>,
  key: string,
  fallback: number,
): number {
  const parsed = Number.parseFloat(params[key] ?? "");
  return Number.isFinite(parsed) ? parsed : fallback;
}

function findArtifact(
  current: BrowserBackendState,
  artifactId: string,
): StoredArtifactContent {
  for (const storedRun of current.runs) {
    const artifact = storedRun.artifacts[artifactId];
    if (artifact) return artifact;
  }
  throw new Error(`Artifact ${artifactId} not found`);
}

/**
 * Replaces or inserts a run and keeps the list newest first, capped at
 * `MAX_STORED_RUNS`; whatever falls off the end is the oldest.
 */
function setRun(
  current: BrowserBackendState,
  storedRun: StoredRun,
): BrowserBackendState {
  const nextRuns = current.runs.filter(
    (entry) => entry.run.id !== storedRun.run.id,
  );
  nextRuns.push(storedRun);
  nextRuns.sort((a, b) => b.run.started_at.localeCompare(a.run.started_at));
  return { ...current, runs: nextRuns.slice(0, MAX_STORED_RUNS) };
}

/**
 * Run ids are minted from the highest id still stored, not from the list
 * length: once the cap evicts runs the length stops growing, and an id would
 * be reused — and `setRun` would then silently replace an older run with the
 * new one, while every artifact id derived from the run id would name two
 * payloads.
 */
function nextRunID(current: BrowserBackendState): string {
  let highest = 0;
  for (const entry of current.runs) {
    const match = /^run-(\d+)$/.exec(entry.run.id);
    if (match?.[1] !== undefined) {
      highest = Math.max(highest, Number.parseInt(match[1], 10));
    }
  }
  return `run-${formatRunIndex(highest)}`;
}

// Exported for the receiver-grid extent test: a Parkplatz is an extended
// footprint, and a grid padded around a point inside it would sit entirely
// within the source. This walks every coordinate, so a polygon contributes all
// its vertices — the CLI needs an explicit extent list to achieve the same.
export function getFeatureBBox(features: ModelFeature[]): {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
} | null {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;

  function visit(coords: unknown): void {
    if (!Array.isArray(coords)) return;
    if (
      coords.length >= 2 &&
      typeof coords[0] === "number" &&
      typeof coords[1] === "number"
    ) {
      const x = coords[0];
      const y = coords[1];
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
      return;
    }
    for (const item of coords) visit(item);
  }

  for (const feature of features) visit(feature.geometry.coordinates);

  if (!Number.isFinite(minX)) return null;
  return { minX, minY, maxX, maxY };
}

function getPolygonBBox(
  rings: number[][][],
): { minX: number; minY: number; maxX: number; maxY: number } | null {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;
  for (const ring of rings) {
    for (const pos of ring) {
      const [x, y] = pos;
      if (x !== undefined && y !== undefined) {
        minX = Math.min(minX, x);
        minY = Math.min(minY, y);
        maxX = Math.max(maxX, x);
        maxY = Math.max(maxY, y);
      }
    }
  }
  if (!Number.isFinite(minX)) return null;
  return { minX, minY, maxX, maxY };
}

function toPoint2D(position: unknown): Point2D | null {
  if (
    Array.isArray(position) &&
    position.length >= 2 &&
    typeof position[0] === "number" &&
    typeof position[1] === "number"
  ) {
    return { x: position[0], y: position[1] };
  }
  return null;
}

function toLineStrings(coords: unknown): Point2D[][] {
  if (!Array.isArray(coords)) return [];
  const maybeLine = coords.map((point) => toPoint2D(point)).filter(Boolean);
  if (maybeLine.length > 0) {
    return [maybeLine as Point2D[]];
  }
  const lines: Point2D[][] = [];
  for (const item of coords) {
    lines.push(...toLineStrings(item));
  }
  return lines;
}

export function buildRoadSources(
  features: ModelFeature[],
  params: Record<string, string>,
): RoadSource[] {
  const sources: RoadSource[] = [];
  for (const feature of features) {
    if (feature.kind !== "source" || feature.sourceType !== "line") continue;
    const centerlines = toLineStrings(feature.geometry.coordinates);
    const junctionType = resolveJunctionType(feature);
    const junctionDistanceM = getFeatureNumber(
      feature,
      "junction_distance_m",
      "road_junction_distance_m",
    );
    const reflectionSurchargeDb = getFeatureNumber(
      feature,
      "reflection_surcharge_db",
    );
    centerlines.forEach((centerline, index) => {
      if (centerline.length < 2) return;
      sources.push({
        id:
          centerlines.length === 1
            ? feature.id
            : `${feature.id}-${String(index + 1)}`,
        centerline,
        surface_type: resolveSurfaceType(feature, params),
        speeds: {
          pkw_kph: resolveFeatureNumber(
            feature,
            params,
            ["speed_pkw_kph"],
            100,
            "road_speed_kph",
          ),
          lkw1_kph: resolveFeatureNumber(
            feature,
            params,
            ["speed_lkw1_kph"],
            100,
            "road_speed_kph",
          ),
          lkw2_kph: resolveFeatureNumber(
            feature,
            params,
            ["speed_lkw2_kph"],
            80,
            "road_speed_kph",
          ),
          krad_kph: resolveFeatureNumber(
            feature,
            params,
            ["speed_krad_kph"],
            100,
            "road_speed_kph",
          ),
        },
        gradient_percent: resolveFeatureNumber(
          feature,
          params,
          ["gradient_percent"],
          0,
          "road_gradient_percent",
        ),
        // Optional on RoadSource: omit the key entirely rather than assigning
        // an explicit `undefined`.
        ...(junctionType !== undefined && { junction_type: junctionType }),
        ...(junctionDistanceM !== undefined && {
          junction_distance_m: junctionDistanceM,
        }),
        ...(reflectionSurchargeDb !== undefined && {
          reflection_surcharge_db: reflectionSurchargeDb,
        }),
        traffic_day: {
          pkw_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_day_pkw"],
            900,
          ),
          lkw1_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_day_lkw1"],
            40,
          ),
          lkw2_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_day_lkw2"],
            60,
          ),
          krad_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_day_krad"],
            10,
          ),
        },
        traffic_night: {
          pkw_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_night_pkw"],
            200,
          ),
          lkw1_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_night_lkw1"],
            10,
          ),
          lkw2_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_night_lkw2"],
            20,
          ),
          krad_per_hour: resolveFeatureNumber(
            feature,
            params,
            ["traffic_night_krad"],
            2,
          ),
        },
      });
    });
  }
  return sources;
}

function resolveFeatureNumber(
  feature: ModelFeature,
  params: Record<string, string>,
  keys: string[],
  fallback: number,
  sharedKey?: string,
): number {
  const featureValue =
    sharedKey == null
      ? getFeatureNumber(feature, ...keys)
      : getFeatureNumber(feature, ...keys, sharedKey);
  if (featureValue != null) {
    return featureValue;
  }
  return parseNumber(params, keys[0] ?? "", fallback);
}

/**
 * Validates an arbitrary (user- or OSM-supplied) string against the RLS-19
 * surface list, returning `undefined` when it is not a known surface.
 *
 * `RLS19_SURFACE_TYPES` mirrors the Go `rls19/road.SurfaceType` constants, so a
 * value that survives this check is a valid `SurfaceType` by construction.
 */
function toSurfaceType(
  value: string | undefined,
): RoadSource["surface_type"] | undefined {
  if (value === undefined) return undefined;
  return RLS19_SURFACE_TYPES.find(
    (entry): entry is RoadSource["surface_type"] & typeof entry =>
      entry === value,
  );
}

function resolveSurfaceType(
  feature: ModelFeature,
  params: Record<string, string>,
): RoadSource["surface_type"] {
  return (
    toSurfaceType(
      getFeatureString(feature, "surface_type", "road_surface_type"),
    ) ??
    toSurfaceType(params["surface_type"]) ??
    "SMA"
  );
}

function resolveJunctionType(
  feature: ModelFeature,
): RoadSource["junction_type"] | undefined {
  const value = getFeatureString(
    feature,
    "junction_type",
    "road_junction_type",
  );
  switch (value) {
    case "none":
      return 0;
    case "signalized":
      return 1;
    case "roundabout":
      return 2;
    case "other":
      return 3;
    default:
      return undefined;
  }
}

function buildBarriers(features: ModelFeature[]): Barrier[] {
  const barriers: Barrier[] = [];
  for (const feature of features) {
    if (feature.kind !== "barrier") continue;
    const lines = toLineStrings(feature.geometry.coordinates);
    lines.forEach((geometry, index) => {
      if (geometry.length < 2) return;
      barriers.push({
        id:
          lines.length === 1
            ? feature.id
            : `${feature.id}-${String(index + 1)}`,
        geometry,
        height_m: feature.heightM ?? 2,
      });
    });
  }
  return barriers;
}

// buildBuildings mirrors extractRLS19Buildings: a building is a barrier and a
// reflector at once, and the reflection loss falls back to the RLS-19 Tabelle 8
// facade row, 0.5 dB, when the feature does not state one.
//
// A MultiPolygon becomes one building per part, with the same suffixed IDs the
// CLI assigns, rather than being dropped: a skipped building is a receiver
// computed as though nothing stood there, which is the quiet-zero failure this
// whole change set exists to remove. The field checks mirror Building.Validate,
// because PropagationConfig.Validate does not inspect buildings — an unchecked
// zero height would compute happily and shield nothing.
export function buildBuildings(features: ModelFeature[]): Building[] {
  const buildings: Building[] = [];

  for (const feature of features) {
    if (feature.kind !== "building") continue;

    const parts = polygonParts(feature);
    if (parts === null) {
      throw new Error(
        `feature "${feature.id}": a building must be a Polygon or a MultiPolygon`,
      );
    }

    const heightM = feature.heightM;
    if (heightM === undefined || !Number.isFinite(heightM) || heightM <= 0) {
      throw new Error(
        `feature "${feature.id}": building height_m must be finite and > 0`,
      );
    }

    const reflectionLossDb =
      getFeatureNumber(feature, "reflection_loss_db") ?? 0.5;
    if (!Number.isFinite(reflectionLossDb) || reflectionLossDb < 0) {
      throw new Error(
        `feature "${feature.id}": building reflection_loss_db must be finite and >= 0`,
      );
    }

    parts.forEach((rings, index) => {
      const footprint = rings[0];
      if (!footprint || footprint.length < 3) {
        throw new Error(
          `feature "${feature.id}": building footprint must contain at least 3 vertices`,
        );
      }

      buildings.push({
        id:
          parts.length === 1
            ? feature.id
            : `${feature.id}-${String(index + 1).padStart(2, "0")}`,
        footprint,
        height_m: heightM,
        reflection_loss_db: reflectionLossDb,
      });
    });
  }

  return buildings;
}

function buildReceiverGrid(
  bbox: { minX: number; minY: number; maxX: number; maxY: number },
  params: Record<string, string>,
): { receivers: PointReceiver[]; width: number; height: number } {
  const resolution = parseNumber(params, "grid_resolution_m", 10);
  const padding = parseNumber(params, "grid_padding_m", 20);
  const receiverHeight = parseNumber(params, "receiver_height_m", 4);

  const minX = bbox.minX - padding;
  const minY = bbox.minY - padding;
  const maxX = bbox.maxX + padding;
  const maxY = bbox.maxY + padding;

  const width = Math.max(1, Math.floor((maxX - minX) / resolution) + 1);
  const height = Math.max(1, Math.floor((maxY - minY) / resolution) + 1);

  const receivers: PointReceiver[] = [];
  let seq = 1;
  for (let row = 0; row < height; row++) {
    const y = minY + row * resolution;
    for (let col = 0; col < width; col++) {
      const x = minX + col * resolution;
      receivers.push({
        id: `R${String(seq).padStart(4, "0")}`,
        point: { x, y },
        height_m: receiverHeight,
      });
      seq++;
    }
  }

  return { receivers, width, height };
}

async function sha256Hex(value: string): Promise<string> {
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((part) => part.toString(16).padStart(2, "0"))
    .join("");
}

function buildReceiverTable(outputs: ReceiverOutput[]): ReceiverTable {
  return {
    indicator_order: ["LrDay", "LrNight"],
    unit: "dB(A)",
    records: outputs.map((output) => ({
      id: output.Receiver.id,
      x: output.Receiver.point.x,
      y: output.Receiver.point.y,
      height_m: output.Receiver.height_m,
      values: {
        LrDay: output.Indicators.lr_day,
        LrNight: output.Indicators.lr_night,
      },
    })),
  };
}

function buildReceiverCSV(table: ReceiverTable): string {
  const headers = ["id", "x", "y", "height_m", ...table.indicator_order];
  const rows = table.records.map((record) => [
    record.id,
    String(record.x),
    String(record.y),
    String(record.height_m),
    ...table.indicator_order.map((indicator) =>
      String(record.values[indicator] ?? ""),
    ),
  ]);
  return [headers, ...rows]
    .map((row) =>
      row.map((value) => `"${value.replaceAll('"', '""')}"`).join(","),
    )
    .join("\n");
}

function makeArtifact(
  runId: string,
  suffix: string,
  kind: string,
  path: string,
  createdAt: string,
): ArtifactRef {
  return {
    id: `artifact-${runId}-${suffix}`,
    kind,
    path,
    created_at: createdAt,
  };
}

// Receiver values are an open Record, and stored runs are replayed from
// IndexedDB, so an indicator can legitimately be absent (e.g. a run written
// by an older build). Render it as an empty cell instead of throwing, matching
// how buildReceiverCSV already handles missing indicators.
function formatIndicator(values: Record<string, number>, key: string): string {
  return values[key]?.toFixed(1) ?? "";
}

function browserExportHTML(run: RunSummary, table: ReceiverTable): string {
  const previewRows = table.records.slice(0, 20);
  const rowHtml = previewRows
    .map(
      (record) =>
        `<tr><td>${record.id}</td><td>${record.x.toFixed(2)}</td><td>${record.y.toFixed(2)}</td><td>${formatIndicator(record.values, "LrDay")}</td><td>${formatIndicator(record.values, "LrNight")}</td></tr>`,
    )
    .join("");
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Aconiq Export ${run.id}</title>
    <style>
      body { font-family: "IBM Plex Sans", sans-serif; margin: 2rem; color: #1f2937; }
      table { border-collapse: collapse; width: 100%; margin-top: 1rem; }
      th, td { border: 1px solid #d1d5db; padding: 0.5rem; text-align: left; font-size: 0.875rem; }
      th { background: #f3f4f6; }
      .meta { color: #6b7280; font-size: 0.875rem; }
    </style>
  </head>
  <body>
    <h1>Aconiq Export</h1>
    <p class="meta">Run ${run.id} · ${run.standard_id} / ${run.version}${run.profile ? ` / ${run.profile}` : ""}</p>
    <p>Receiver preview (${String(previewRows.length)} of ${String(table.records.length)})</p>
    <table>
      <thead>
        <tr><th>ID</th><th>X</th><th>Y</th><th>LrDay</th><th>LrNight</th></tr>
      </thead>
      <tbody>${rowHtml}</tbody>
    </table>
  </body>
</html>`;
}

function browserExportMarkdown(run: RunSummary, table: ReceiverTable): string {
  const previewRows = table.records
    .slice(0, 10)
    .map(
      (record) =>
        `| ${record.id} | ${record.x.toFixed(2)} | ${record.y.toFixed(2)} | ${formatIndicator(record.values, "LrDay")} | ${formatIndicator(record.values, "LrNight")} |`,
    )
    .join("\n");
  return `# Aconiq Export

Run: \`${run.id}\`

Standard: \`${run.standard_id}\` / \`${run.version}\`${run.profile ? ` / \`${run.profile}\`` : ""}

## Receiver Preview

| ID | X | Y | LrDay | LrNight |
| --- | ---: | ---: | ---: | ---: |
${previewRows}
`;
}

function findRunByID(current: BrowserBackendState, runId: string): StoredRun {
  const storedRun = current.runs.find((entry) => entry.run.id === runId);
  if (!storedRun) {
    throw new Error(`Run ${runId} not found`);
  }
  return storedRun;
}

export const browserBackend = {
  capabilities: {
    kind: "browser",
    canExport: true,
    runsAgainstSavedModel: false,
    runsChangeExternally: false,
  },

  async getHealth(): Promise<HealthResponse> {
    await getKernel();
    return {
      status: "ok",
      version: "wasm-browser",
      time: nowISO(),
    };
  },

  async getProjectStatus(): Promise<ProjectStatusResponse> {
    const current = await ensureLoaded();
    const features = useModelStore.getState().features;
    const lastRun = current.runs
      .map((entry) => entry.run)
      .sort((a, b) => b.started_at.localeCompare(a.started_at))[0];

    return {
      project_id: current.projectId,
      name:
        features.length > 0
          ? `${current.projectName} (${String(features.length)} features)`
          : current.projectName,
      project_path: current.projectPath,
      manifest_version: 1,
      crs: current.crs,
      scenario_count: 1,
      run_count: current.runs.length,
      ...(lastRun
        ? {
            last_run: {
              id: lastRun.id,
              status: lastRun.status,
              standard_id: lastRun.standard_id,
              version: lastRun.version,
              ...(lastRun.profile ? { profile: lastRun.profile } : {}),
              started_at: lastRun.started_at,
              finished_at: lastRun.finished_at,
            },
          }
        : {}),
    };
  },

  async getStandards(): Promise<StandardDescriptor[]> {
    return BROWSER_STANDARDS;
  },

  async getRuns(): Promise<RunSummary[]> {
    const current = await ensureLoaded();
    return current.runs
      .map((entry) => entry.run)
      .sort((a, b) => b.started_at.localeCompare(a.started_at));
  },

  async getRunLog(runId: string): Promise<RunLog> {
    return findRunByID(await ensureLoaded(), runId).log;
  },

  async getArtifactContent<T>(artifactId: string): Promise<T> {
    return findArtifact(await ensureLoaded(), artifactId).value as T;
  },

  /**
   * Synchronous because pages put the result straight into `<iframe src>`
   * and `<a href>`. It reads the in-memory state only, which is populated by
   * the first async call — and an artifact id can only come from one of
   * those (`getRuns`, `startRun`, `createExport`), so by the time a page
   * holds an id to ask about, the state is loaded. The throw below guards
   * the assumption; it is not a path the UI reaches.
   */
  getArtifactURL(artifactId: string): string {
    const cached = urlCache.get(artifactId);
    if (cached) return cached;
    if (state === null) {
      throw new Error(
        `Artifact ${artifactId} requested before the browser backend loaded its stored runs`,
      );
    }
    const content = findArtifact(state, artifactId);
    const body =
      content.encoding === "json"
        ? JSON.stringify(content.value, null, 2)
        : String(content.value);
    const url = URL.createObjectURL(
      new Blob([body], { type: content.mimeType }),
    );
    urlCache.set(artifactId, url);
    return url;
  },

  async importFromOSM(
    req: OsmImportRequest,
  ): Promise<GeoJSONFeatureCollection> {
    const endpoint = req.overpass_endpoint || DEFAULT_OSM_ENDPOINT;
    const bbox = [req.south, req.west, req.north, req.east].join(",");
    const query = `[out:json][timeout:25];
(
  way["highway"](${bbox});
  way["railway"~"^(rail|tram)$"](${bbox});
  way["building"](${bbox});
  way["barrier"~"^(wall|fence)$"](${bbox});
);
out geom;`;

    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "text/plain;charset=UTF-8",
      },
      body: query,
    });
    if (!response.ok) {
      throw new Error(`Request failed: ${String(response.status)}`);
    }
    const data = (await response.json()) as OverpassResponse;
    const features = (data.elements ?? [])
      .filter((element): element is OverpassWay => {
        return element["type"] === "way";
      })
      .map((way) => overpassWayToFeature(way))
      .filter(
        (feature): feature is GeoJSONFeatureCollection["features"][number] => {
          return feature !== null;
        },
      );
    return { type: "FeatureCollection", features };
  },

  async startRun(spec: RunSpec): Promise<RunSummary> {
    // Load the kernel before reading the model, so a kernel that cannot load
    // is the failure reported, not a model finding it would never compute.
    const kernel = await getKernel();

    if (spec.standardId !== "rls19-road") {
      throw new Error(
        `Standard ${spec.standardId} is not available in browser mode`,
      );
    }

    const features = useModelStore.getState().features;
    const sources = buildRoadSources(features, spec.params);
    const parking = buildParkingSources(features);
    if (sources.length === 0 && parking.sources.length === 0) {
      throw new Error(
        "model does not contain any rls19-road line source or parking area feature",
      );
    }

    const barriers = buildBarriers(features);
    const buildings = buildBuildings(features);

    let gridReceivers: PointReceiver[];
    let rasterWidth: number;
    let rasterHeight: number;

    if (spec.receiverMode === "custom") {
      const storeReceivers = useModelStore.getState().receivers;
      if (storeReceivers.length === 0) {
        throw new Error(
          "Custom receiver mode requires at least one receiver placed in the map workspace",
        );
      }
      const sorted = [...storeReceivers].sort((a, b) =>
        a.id.localeCompare(b.id),
      );
      gridReceivers = sorted.map((r) => ({
        id: r.id,
        point: { x: r.geometry.coordinates[0], y: r.geometry.coordinates[1] },
        height_m: r.heightM,
      }));
      rasterWidth = 1;
      rasterHeight = sorted.length;
    } else {
      const calcArea = useModelStore.getState().calcArea;
      let bbox: {
        minX: number;
        minY: number;
        maxX: number;
        maxY: number;
      } | null = null;
      if (calcArea) {
        bbox = getPolygonBBox(calcArea.geometry.coordinates);
      }
      if (!bbox) {
        bbox = getFeatureBBox(
          features.filter((feature) => feature.kind === "source"),
        );
      }
      if (!bbox) {
        throw new Error(
          "Could not derive source extent from the current model",
        );
      }
      const receiverGrid = buildReceiverGrid(bbox, spec.params);
      gridReceivers = receiverGrid.receivers;
      rasterWidth = receiverGrid.width;
      rasterHeight = receiverGrid.height;
    }

    const startedAt = nowISO();
    const runId = nextRunID(await ensureLoaded());
    const basePath = `${DEFAULT_PROJECT_PATH}/runs/${runId}`;

    const request: ComputeRequest = {
      receivers: gridReceivers,
      sources,
      barriers,
      config: {
        SegmentLengthM: parseNumber(spec.params, "segment_length_m", 1),
        MinDistanceM: parseNumber(spec.params, "min_distance_m", 3),
        ReceiverHeightM: parseNumber(spec.params, "receiver_height_m", 4),
        Buildings: buildings,
        ParkingSources: parking.sources,
      },
    };

    const outputs = await kernel.rls19Road(request);
    const receiverTable = buildReceiverTable(outputs);
    const receiverCSV = buildReceiverCSV(receiverTable);
    const rasterMetadata: RasterMetadata = {
      width: rasterWidth,
      height: rasterHeight,
      bands: 2,
      nodata: -9999,
      unit: "dB(A)",
      band_names: ["LrDay", "LrNight"],
    };
    const summary = {
      run_id: runId,
      status: "completed",
      grid_width: rasterWidth,
      grid_height: rasterHeight,
      source_count: sources.length,
      parking_source_count: parking.sources.length,
      receiver_count: outputs.length,
      reporting_precision_db: 0.1,
    };

    const hashPayload = outputs.map((output) => ({
      receiver_id: output.Receiver.id,
      indicators: output.Indicators,
    }));
    const outputHash = await sha256Hex(JSON.stringify(hashPayload));
    const finishedAt = nowISO();
    const createdAt = finishedAt;

    const receiversJSONArtifact = makeArtifact(
      runId,
      "receivers-json",
      "run.result.receiver_table_json",
      `${basePath}/results/receivers.json`,
      createdAt,
    );
    const receiversCSVArtifact = makeArtifact(
      runId,
      "receivers-csv",
      "run.result.receiver_table_csv",
      `${basePath}/results/receivers.csv`,
      createdAt,
    );
    const rasterMetaArtifact = makeArtifact(
      runId,
      "raster-meta",
      "run.result.raster_metadata",
      `${basePath}/results/rls19-road.json`,
      createdAt,
    );
    const rasterBinArtifact = makeArtifact(
      runId,
      "raster-bin",
      "run.result.raster_binary",
      `${basePath}/results/rls19-road.bin`,
      createdAt,
    );
    const summaryArtifact = makeArtifact(
      runId,
      "summary",
      "run.result.summary",
      `${basePath}/results/run-summary.json`,
      createdAt,
    );
    const artifacts: ArtifactRef[] = [
      receiversJSONArtifact,
      receiversCSVArtifact,
      rasterMetaArtifact,
      rasterBinArtifact,
      summaryArtifact,
    ];

    const run: RunSummary = {
      id: runId,
      scenario_id: "default",
      standard_id: spec.standardId,
      version: spec.version,
      ...(spec.profile ? { profile: spec.profile } : {}),
      status: "completed",
      started_at: startedAt,
      finished_at: finishedAt,
      log_path: `${basePath}/run.log`,
      artifacts,
    };

    const log: RunLog = {
      run_id: runId,
      lines: [
        `${startedAt} run started`,
        `${startedAt} model=browser`,
        `${startedAt} rls19_road_sources=${String(sources.length)}`,
        `${startedAt} rls19_parking_sources=${String(parking.sources.length)}`,
        `${startedAt} rls19_buildings=${String(buildings.length)}`,
        `${startedAt} receivers=${String(gridReceivers.length)}`,
        `${startedAt} stage=compute`,
        `${finishedAt} output_hash=${outputHash}`,
        `${finishedAt} persisted=browser`,
        `${finishedAt} run completed`,
      ],
    };

    const artifactMap: Record<string, StoredArtifactContent> = {
      [receiversJSONArtifact.id]: {
        kind: receiversJSONArtifact.kind,
        mimeType: "application/json",
        encoding: "json",
        value: receiverTable,
      },
      [receiversCSVArtifact.id]: {
        kind: receiversCSVArtifact.kind,
        mimeType: "text/csv",
        encoding: "text",
        value: receiverCSV,
      },
      [rasterMetaArtifact.id]: {
        kind: rasterMetaArtifact.kind,
        mimeType: "application/json",
        encoding: "json",
        value: rasterMetadata,
      },
      [rasterBinArtifact.id]: {
        kind: rasterBinArtifact.kind,
        mimeType: "application/octet-stream",
        encoding: "text",
        value: outputHash,
      },
      [summaryArtifact.id]: {
        kind: summaryArtifact.kind,
        mimeType: "application/json",
        encoding: "json",
        value: { ...summary, output_hash: outputHash },
      },
    };

    await persistRun(
      await ensureLoaded(),
      { run, log, artifacts: artifactMap },
      "run",
    );
    return run;
  },

  async createExport(runId: string): Promise<RunSummary> {
    const current = await ensureLoaded();
    const storedRun = findRunByID(current, runId);
    const tableArtifact = storedRun.run.artifacts.find(
      (artifact) => artifact.kind === "run.result.receiver_table_json",
    );
    if (!tableArtifact) {
      throw new Error("Run has no receiver table artifact");
    }
    const receiverTable = storedRun.artifacts[tableArtifact.id]
      ?.value as ReceiverTable;
    const exportedAt = nowISO();
    const exportBase = `${DEFAULT_PROJECT_PATH}/exports/${runId}-${exportedAt.replaceAll(":", "").replaceAll(".", "")}`;

    const context = {
      exported_at: exportedAt,
      project_id: DEFAULT_PROJECT_ID,
      run: storedRun.run,
      receiver_table: receiverTable,
    };
    const html = browserExportHTML(storedRun.run, receiverTable);
    const markdown = browserExportMarkdown(storedRun.run, receiverTable);
    const bundleSummary = {
      export_id: `${runId}-${exportedAt}`,
      run_id: runId,
      exported_at: exportedAt,
      copied_files: [
        "results/receivers.json",
        "results/receivers.csv",
        "results/run-summary.json",
      ],
      generated_reports: [
        "report/report-context.json",
        "report/report.md",
        "report/report.html",
      ],
    };

    const exportStamp = String(Date.now());
    const bundleArtifact = makeArtifact(
      runId,
      `export-bundle-${String(storedRun.run.artifacts.filter((artifact) => artifact.kind.startsWith("export.")).length + 1)}`,
      "export.bundle",
      `${exportBase}/export-summary.json`,
      exportedAt,
    );
    const contextArtifact = makeArtifact(
      runId,
      `export-context-${exportStamp}`,
      "export.report_context_json",
      `${exportBase}/report/report-context.json`,
      exportedAt,
    );
    const markdownArtifact = makeArtifact(
      runId,
      `export-markdown-${exportStamp}`,
      "export.report_markdown",
      `${exportBase}/report/report.md`,
      exportedAt,
    );
    const htmlArtifact = makeArtifact(
      runId,
      `export-html-${exportStamp}`,
      "export.report_html",
      `${exportBase}/report/report.html`,
      exportedAt,
    );
    const exportArtifacts: ArtifactRef[] = [
      bundleArtifact,
      contextArtifact,
      markdownArtifact,
      htmlArtifact,
    ];

    const nextStoredRun: StoredRun = {
      run: {
        ...storedRun.run,
        artifacts: [...storedRun.run.artifacts, ...exportArtifacts],
      },
      log: storedRun.log,
      artifacts: {
        ...storedRun.artifacts,
        [bundleArtifact.id]: {
          kind: bundleArtifact.kind,
          mimeType: "application/json",
          encoding: "json",
          value: bundleSummary,
        },
        [contextArtifact.id]: {
          kind: contextArtifact.kind,
          mimeType: "application/json",
          encoding: "json",
          value: context,
        },
        [markdownArtifact.id]: {
          kind: markdownArtifact.kind,
          mimeType: "text/markdown",
          encoding: "text",
          value: markdown,
        },
        [htmlArtifact.id]: {
          kind: htmlArtifact.kind,
          mimeType: "text/html",
          encoding: "text",
          value: html,
        },
      },
    };

    await persistRun(current, nextStoredRun, "export");
    return nextStoredRun.run;
  },

  /**
   * In browser mode the model store is the project: a run reads it directly,
   * so there is nothing to write.
   */
  async saveModel(req: ModelSaveRequest): Promise<ModelSaveResult> {
    return { featureCount: req.model.features.length, warnings: [] };
  },
} satisfies Backend;

export function overpassWayToFeature(
  way: OverpassWay,
): GeoJSONFeatureCollection["features"][number] | null {
  const geometry = sanitizeOverpassGeometry(way.geometry);
  if (geometry.length < 2) return null;
  const tags = way.tags ?? {};
  const featureId = `osm-way-${String(way.id)}`;
  const properties: Record<string, unknown> = {
    osm_id: String(way.id),
  };

  if (tags["highway"]) {
    properties["kind"] = "source";
    properties["source_type"] = "line";
    properties["highway"] = tags["highway"];
    const speed = parseMaxspeedTag(tags["maxspeed"]);
    if (speed != null) {
      properties["road_speed_kph"] = speed;
      properties["road_speed_kph_inferred"] = true;
    }
    const surfaceType = mapOSMSurfaceToRLS19(tags["surface"]);
    if (surfaceType) {
      properties["surface_type"] = surfaceType;
      properties["surface_type_inferred"] = true;
    }
    properties["source_acoustics_review_required"] =
      speed == null || surfaceType == null;
    return {
      type: "Feature",
      id: featureId,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  if (tags["railway"]) {
    properties["kind"] = "source";
    properties["source_type"] = "line";
    properties["railway"] = tags["railway"];
    return {
      type: "Feature",
      id: featureId,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  if (tags["building"]) {
    const ring: [number, number][] = geometry.map((point) => [
      point.lon,
      point.lat,
    ]);
    if (ring.length < 3) return null;
    const first = ring[0];
    const last = ring[ring.length - 1];
    // Unreachable given the length check above; keeps the indexed access sound.
    if (!first || !last) return null;
    if (first[0] !== last[0] || first[1] !== last[1]) {
      ring.push([first[0], first[1]]);
    }
    properties["kind"] = "building";
    properties["building"] = tags["building"];
    if (tags["building:levels"]) {
      properties["building:levels"] = tags["building:levels"];
    }
    properties["height_m"] = parseTagHeight(tags["height"]) ?? 9;
    return {
      type: "Feature",
      id: featureId,
      properties,
      geometry: {
        type: "Polygon",
        coordinates: [ring],
      },
    };
  }

  if (tags["barrier"]) {
    properties["kind"] = "barrier";
    properties["barrier"] = tags["barrier"];
    properties["height_m"] = parseTagHeight(tags["height"]) ?? 2;
    return {
      type: "Feature",
      id: featureId,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  return null;
}

function parseTagHeight(value: string | undefined): number | null {
  if (!value) return null;
  const parsed = Number.parseFloat(
    value.replace(" m", "").replace("m", "").trim(),
  );
  return Number.isFinite(parsed) ? parsed : null;
}

function parseMaxspeedTag(value: string | undefined): number | null {
  if (!value) {
    return null;
  }
  const parsed = Number.parseFloat(value.replace(/km\/h/i, "").trim());
  return Number.isFinite(parsed) ? parsed : null;
}

function mapOSMSurfaceToRLS19(
  value: string | undefined,
): RoadSource["surface_type"] | null {
  switch ((value ?? "").trim().toLowerCase()) {
    case "asphalt":
    case "paved":
      return "SMA";
    case "concrete":
      return "Beton";
    case "sett":
    case "cobblestone":
    case "paving_stones":
      return "Pflaster";
    case "unpaved":
    case "compacted":
    case "gravel":
      return "beschaedigt";
    default:
      return null;
  }
}

function sanitizeOverpassGeometry(
  // The elements come from an untrusted Overpass response, so a null hole is
  // possible despite what `OverpassWay` declares.
  geometry: (OverpassPoint | null | undefined)[] | undefined,
): OverpassPoint[] {
  if (!geometry) return [];
  return geometry.filter((point): point is OverpassPoint => {
    return (
      point != null && Number.isFinite(point.lon) && Number.isFinite(point.lat)
    );
  });
}
