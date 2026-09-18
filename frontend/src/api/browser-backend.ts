/* eslint-disable @typescript-eslint/require-await --
   These methods implement the same async `Backend` interface as the HTTP
   client. Several are synchronous in-memory lookups, but they must stay
   `async` so that a thrown error surfaces as a rejected promise, exactly as
   it does on the HTTP path. */
import type {
  Backend,
  ContourOptions,
  DeleteRunResult,
  ModelSaveResult,
  OsmImportRequest,
  RunContours,
  RunSpec,
} from "./backend";
import type {
  ArtifactRef,
  HealthResponse,
  ModelResponse,
  ModelSaveRequest,
  ProjectStatusResponse,
  RasterGeoreference,
  RasterMetadata,
  ReceiverTable,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import type { GeoJSONFeatureCollection, ModelFeature } from "@/model/types";
import { useModelStore } from "@/model/model-store";
import { resolveComputeModel } from "@/model/compute-crs";
import {
  getFeatureNumber,
  getFeatureString,
  RLS19_SURFACE_TYPES,
} from "@/model/source-acoustics";
import type { Point2D } from "@/model/geometry";
import { buildParkingSources, polygonParts } from "@/model/rls19-parking";
import { buildRasterBinary } from "@/model/raster-bin";
import { buildReceiverTableCSV } from "@/model/receiver-csv";
import { type AconiqKernel, getKernel } from "@/wasm/kernel";
import type {
  Barrier,
  Building,
  ComputeRequest,
  PointReceiver,
  ReceiverOutput,
  RoadSource,
  TransformRequest,
  TransformResponse,
} from "@/wasm/types";
import {
  BrowserStorageError,
  deleteArtifactBytes,
  isBrowserStorageError,
  loadArtifactBytes,
  loadPersistedState,
  saveArtifactBytes,
  savePersistedState,
  savePersistedStateForgetting,
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
const DEFAULT_OSM_ENDPOINT = "https://overpass-api.de/api/interpreter";

/**
 * What a run stored before browser mode recorded a CRS is labelled with.
 *
 * Such a run has x and y in degrees, because it was computed without the
 * projection this file now applies, and `PERSISTED_STATE_VERSION` deliberately
 * did not move for the change. Saying so is the point: labelling it "EPSG:4326"
 * would be true of the coordinates and misleading about the levels, and
 * labelling it with the compute CRS would be a straight lie.
 */
const CRS_NOT_RECORDED = "CRS not recorded";

/**
 * One stored artifact's content.
 *
 * `encoding` says how `value` is to be read: a `json` value is the decoded
 * object, a `text` value is the string, and a `binary` value is **not here at
 * all** — the bytes live under their own IndexedDB key
 * (`browser-storage.saveArtifactBytes`) and `value` is null. The document is
 * written whole on every change, so a raster binary inside it would be
 * structured-cloned, runs and all, on every save.
 */
type StoredArtifactContent = {
  mimeType: string;
  kind: string;
  encoding: "json" | "text" | "binary";
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
  runs: StoredRun[];
  /**
   * The highest run index ever minted, whether or not that run is still
   * stored. Run ids are minted from this and never from the stored list — see
   * `nextRunID`.
   *
   * Added without bumping `PERSISTED_STATE_VERSION`, deliberately. A document
   * written before this field existed simply lacks it, and `decodeState`
   * derives it; a bump would instead make an older build read the new document
   * as corrupt and throw the user's twenty runs away, which is a worse trade
   * than the one it would be protecting against.
   *
   * What the missing bump does cost is named rather than hidden: an older
   * build ignores this field, so after a newer build has deleted the
   * highest-numbered run, that older build mints the next id from the
   * surviving list and reuses the deleted one. It takes a stale tab against a
   * newer document to reach — the app ships as one bundle, so "an older build"
   * is not a supported configuration — and it costs a colliding id, where the
   * bump costs every run in the document.
   */
  runHighWaterMark: number;
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

function initialState(): BrowserBackendState {
  return {
    projectId: DEFAULT_PROJECT_ID,
    projectName: DEFAULT_PROJECT_NAME,
    projectPath: DEFAULT_PROJECT_PATH,
    runs: [],
    runHighWaterMark: 0,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStoredArtifactContent(
  value: unknown,
): value is StoredArtifactContent {
  return (
    isRecord(value) &&
    typeof value["mimeType"] === "string" &&
    typeof value["kind"] === "string" &&
    (value["encoding"] === "json" ||
      value["encoding"] === "text" ||
      value["encoding"] === "binary") &&
    "value" in value
  );
}

/**
 * A run is only as usable as its artifacts: `getArtifactURL` serialises
 * whatever content it finds, so an artifact without `encoding` or `value`
 * would mint a blob reading "undefined" rather than fail. One bad artifact
 * drops the whole run — the run's own artifact list would otherwise point
 * at content that is not there.
 */
function isStoredRun(value: unknown): value is StoredRun {
  if (!isRecord(value)) return false;
  const { run, log, artifacts } = value;
  return (
    isRecord(run) &&
    typeof run["id"] === "string" &&
    typeof run["started_at"] === "string" &&
    Array.isArray(run["artifacts"]) &&
    isRecord(log) &&
    isRecord(artifacts) &&
    Object.values(artifacts).every(isStoredArtifactContent)
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
  const runs = (Array.isArray(value["runs"]) ? value["runs"] : []).filter(
    isStoredRun,
  );
  for (const storedRun of runs) {
    markLegacyRasterBytesMissing(storedRun);
  }
  // A document written before the mark existed carries none, and the highest
  // stored id is exactly right for it: nothing could delete a run then, so no
  // id above the surviving ones was ever handed out.
  const storedMark = value["runHighWaterMark"];
  return {
    projectId: text("projectId", defaults.projectId),
    projectName: text("projectName", defaults.projectName),
    projectPath: text("projectPath", defaults.projectPath),
    runs,
    runHighWaterMark:
      typeof storedMark === "number" && Number.isInteger(storedMark)
        ? Math.max(storedMark, highestRunIndex(runs))
        : highestRunIndex(runs),
  };
}

/**
 * Turns a pre-`binary` raster artifact into one that admits it has no bytes.
 *
 * Browser mode used to store `run.result.raster_binary` as `text` whose value
 * was the run's SHA-256 hex string — 64 characters where a float64 array
 * belongs. Those records are still in the store and `PERSISTED_STATE_VERSION`
 * deliberately did not move for the change: a bump makes an older document
 * unreadable and throws twenty runs away, where the cost here is one raster
 * that was never really stored.
 *
 * What the missing bump would otherwise cost is the reason this exists.
 * `getArtifactContent` reads bytes only for `encoding === "binary"`, so such an
 * artifact would hand a caller asking for an `ArrayBuffer` a digest string that
 * decodes as a raster of nothing. Relabelling it puts it on the path that
 * already covers a document and a byte store that disagree, and it says so:
 * "declares binary content, but its bytes are not stored". The run's other
 * artifacts — receiver table, sidecar, summary — are untouched and still open.
 */
function markLegacyRasterBytesMissing(storedRun: StoredRun): void {
  for (const content of Object.values(storedRun.artifacts)) {
    if (content.kind !== "run.result.raster_binary") continue;
    if (content.encoding === "binary") continue;
    content.encoding = "binary";
    content.mimeType = "application/octet-stream";
    content.value = null;
  }
}

/** The largest `run-NNNN` index among the stored runs, or 0. */
function highestRunIndex(runs: StoredRun[]): number {
  let highest = 0;
  for (const entry of runs) {
    const match = /^run-(\d+)$/.exec(entry.run.id);
    if (match?.[1] !== undefined) {
      highest = Math.max(highest, Number.parseInt(match[1], 10));
    }
  }
  return highest;
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
 * a later build could still have read. An unavailable IndexedDB also means
 * the legacy localStorage document is not consulted: the session runs in
 * memory, and the migration waits for a browser that can hold its result.
 *
 * `whenUnavailable` is what an unavailable store yields: a fresh state on the
 * first load, the copy already in memory on a reload.
 */
async function loadState(
  whenUnavailable: () => BrowserBackendState = initialState,
): Promise<BrowserBackendState> {
  let stored: unknown;
  try {
    stored = await loadPersistedState();
  } catch (error) {
    console.warn(
      "Browser-mode runs cannot be persisted in this browser; they will be kept in memory for this session only",
      error,
    );
    return whenUnavailable();
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
 * In-memory copy of the persisted state. Reads serve this cache: every async
 * read awaits `ensureLoaded()`, and `persist()` updates the copy before the
 * store, so a read is never behind what the UI was just told. Writes do not
 * trust it: the store is shared by every tab of the origin, so a write first
 * calls `reloadState()` and merges into what the store holds now — a run
 * another tab completed since this tab loaded is kept, not overwritten.
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

/**
 * Re-reads the store and replaces the cache with it. An unavailable store
 * keeps the in-memory copy — the session is memory-only, as `loadState`
 * documents — and an unreadable document yields a fresh state, as on first
 * load: the write that follows replaces it. A run that could not be stored
 * earlier (quota) lives only in the cache, so it stays viewable until the next
 * write reloads; the error the user saw already says it is not kept.
 */
async function reloadState(): Promise<BrowserBackendState> {
  const cached = await ensureLoaded();
  const fresh = await loadState(() => cached);
  state = fresh;
  return fresh;
}

const STORE_LOCK_NAME = "aconiq-browser-backend";

/**
 * Serialises writers across tabs. IndexedDB is shared by every tab of the
 * origin, but the document is written whole, so two tabs writing at once
 * would still race: both reload, both merge their own run, the second write
 * wins and the first run is gone. The Web Locks API is origin-wide, so a
 * request under one name queues behind every other tab's writer.
 *
 * Where `navigator.locks` is missing (jsdom, older Safari) the callback runs
 * unlocked. `reloadState()` before every write still guarantees that
 * *sequential* cross-tab writes never drop each other's runs; only writes
 * that truly overlap can collide.
 */
async function withStoreLock<T>(fn: () => Promise<T>): Promise<T> {
  // lib.dom declares `locks` as always present; the browsers above disagree.
  const locks = navigator.locks as LockManager | undefined;
  if (locks === undefined) return fn();
  return locks.request(STORE_LOCK_NAME, fn);
}

/**
 * Writes the document, and in the same transaction removes the byte records
 * `dropped` took with it.
 *
 * One transaction, not two, because the pair is one change. Splitting them
 * forces a choice with no good answer: delete first and a failed write leaves a
 * retained run whose raster is gone, delete second and the eviction that exists
 * to make room retries while the room is still occupied. IndexedDB commits both
 * or neither, so neither half can be observed alone.
 */
async function persist(pending: PendingState): Promise<void> {
  state = pending.state;
  pruneURLCache(pending.state);
  await savePersistedStateForgetting(
    { version: PERSISTED_STATE_VERSION, state: pending.state },
    binaryArtifactIDs(pending.dropped),
  );
}

/**
 * Stores a new or updated run, merged into what the store holds *now* — never
 * into this tab's cache, which may predate another tab's runs. On a quota
 * failure the oldest *other* run is evicted and the write retried once; if
 * that still fails the run stays in memory — its results remain viewable for
 * this session — and the caller gets an error whose message says so, because
 * the computation has already succeeded and the dialogs render
 * `error.message`.
 */
async function persistRun(
  storedRun: StoredRun,
  what: "run" | "export",
): Promise<void> {
  const next = setRun(await reloadState(), storedRun);
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
      // The eviction never reached the store, so it must not reach the list
      // either — whatever the retry failed with: the user would see a run
      // vanish alongside an error about a different one.
      state = next.state;
      if (!isBrowserStorageError(error, "quota")) {
        throw storeFailure(what, error);
      }
    }
  }
  throw new BrowserStorageError(
    "quota",
    `The ${what} completed but could not be stored: the browser's storage quota is exhausted. Its results stay available only until the next run or export, or until this page is reloaded; older runs may need deleting to free space.`,
  );
}

function storeFailure(what: "run" | "export", error: unknown): Error {
  if (!isBrowserStorageError(error)) {
    return error instanceof Error ? error : new Error(String(error));
  }
  return new BrowserStorageError(
    error.reason,
    `The ${what} completed but could not be stored (${error.message}). Its results stay available only until the next run or export, or until this page is reloaded.`,
    { cause: error },
  );
}

/**
 * A state the caller has yet to commit, and the runs committing it drops.
 *
 * The two travel together because the byte records are keyed outside the
 * document: deleting them is only safe once the document that stopped naming
 * those runs is on disk. A write that fails leaves the old document in place,
 * still naming them.
 */
type PendingState = {
  state: BrowserBackendState;
  dropped: StoredRun[];
};

/**
 * Drops the oldest run other than `keepId`; `null` when there is none.
 *
 * Carries the victim forward rather than deleting its bytes: eviction exists to
 * free quota, and the raster is the largest thing a run owns, so `persist`
 * removes the two together. Dropping the document entry alone would evict a run
 * and reclaim almost nothing.
 */
function evictOldestRun(
  current: PendingState,
  keepId: string,
): PendingState | null {
  // `setRun` keeps the list newest first, so the victim is the last entry
  // that is not the run being written.
  const runs = [...current.state.runs];
  for (let index = runs.length - 1; index >= 0; index -= 1) {
    const victim = runs[index];
    if (victim === undefined || victim.run.id === keepId) continue;
    runs.splice(index, 1);
    return {
      state: { ...current.state, runs },
      // The retry writes the whole change again, so it carries what the first
      // attempt would have dropped as well: that attempt committed nothing.
      dropped: [...current.dropped, victim],
    };
  }
  return null;
}

/** The ids of every artifact these runs keep outside the document. */
function binaryArtifactIDs(storedRuns: readonly StoredRun[]): string[] {
  return storedRuns.flatMap((storedRun) =>
    Object.entries(storedRun.artifacts)
      .filter(([, content]) => content.encoding === "binary")
      .map(([artifactId]) => artifactId),
  );
}

/**
 * Deletes byte records no document will ever name.
 *
 * Only for bytes that are *already* unreachable — a run written before a
 * persist that then failed. Bytes a stored document is giving up travel with
 * that document's own write instead, through `persist`, so the two commit
 * together.
 *
 * Deliberately not awaited: nothing depends on the outcome, and a failure
 * leaves an orphan record rather than a broken run.
 */
function forgetArtifactBytes(storedRuns: readonly StoredRun[]): void {
  const binaryIds = binaryArtifactIDs(storedRuns);
  if (binaryIds.length === 0) return;
  void deleteArtifactBytes(binaryIds).catch(() => {
    // An orphaned byte record costs quota, not correctness.
  });
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
 * The one reader of the byte store, shared by `getArtifactBytes` and the
 * binary branch of `getArtifactContent` so the two cannot disagree about a
 * record that is not there.
 *
 * The bytes live beside the document, not in it. A missing record means the
 * document and the byte store disagree — a partial quota eviction, or a
 * document restored without them — and an empty buffer read as a raster is a
 * grid of zeroes, so it says so instead.
 */
async function readArtifactBytes(artifactId: string): Promise<ArrayBuffer> {
  const bytes = await loadArtifactBytes(artifactId);
  if (bytes === null) {
    throw new Error(
      `Artifact ${artifactId} declares binary content, but its bytes are not stored`,
    );
  }
  return bytes;
}

/**
 * Replaces or inserts a run and keeps the list newest first, capped at
 * `MAX_STORED_RUNS`; whatever falls off the end is the oldest.
 */
function setRun(
  current: BrowserBackendState,
  storedRun: StoredRun,
): PendingState {
  const nextRuns = current.runs.filter(
    (entry) => entry.run.id !== storedRun.run.id,
  );
  nextRuns.push(storedRun);
  nextRuns.sort((a, b) => b.run.started_at.localeCompare(a.run.started_at));
  const kept = nextRuns.slice(0, MAX_STORED_RUNS);
  return {
    state: {
      ...current,
      runs: kept,
      // Raised here rather than at mint time: this is the moment an id becomes
      // real, and an eviction or a delete afterwards must not lower it.
      runHighWaterMark: Math.max(
        current.runHighWaterMark,
        highestRunIndex([storedRun]),
      ),
    },
    // Whatever the cap drops takes its byte records with it — once this state
    // is stored. They are keyed separately from the document, so trimming the
    // list alone would leave the largest part of every run past the cap in the
    // store forever.
    dropped: nextRuns.slice(MAX_STORED_RUNS),
  };
}

/**
 * Run ids are minted from a persisted high-water mark, not from the stored
 * runs.
 *
 * The list is not a record of what has been handed out: the cap evicts the
 * oldest, and deleting removes whichever the user picked — including the
 * newest, which is what makes reading the maximum back off the list unsafe.
 * A reused id would make `setRun` silently replace an older run, and every
 * artifact id derived from the run id would name two payloads.
 *
 * The mark is raised in `setRun`, so a mint that never reaches storage — a
 * quota failure, a tab closed mid-run — leaves it alone.
 */
function nextRunID(current: BrowserBackendState): string {
  return `run-${formatRunIndex(current.runHighWaterMark)}`;
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

/**
 * The grid layout a run computed on, mirroring `results.GridLayout` on the Go
 * side: the shape, and where it sits. `georeference` is absent for explicit
 * receivers, which are points rather than a grid.
 */
type BrowserGridLayout = {
  width: number;
  height: number;
  georeference?: RasterGeoreference;
};

function buildReceiverGrid(
  bbox: { minX: number; minY: number; maxX: number; maxY: number },
  params: Record<string, string>,
): { receivers: PointReceiver[] } & BrowserGridLayout {
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

  return {
    receivers,
    width,
    height,
    // The padded south-west corner is where the loop above starts, so it is
    // the centre of cell (0,0) — the same convention `buildReceiversFromPoints`
    // records on the CLI side, and the reason `receivers[0]` and `origin` are
    // the same coordinate on both targets.
    georeference: {
      origin_x: minX,
      origin_y: minY,
      pixel_size_m: resolution,
      row_order: ROW_ORDER_SOUTH_UP,
    },
  };
}

/**
 * `results.RowOrderSouthUp`. The loop above walks Y ascending, as
 * `geo.GridReceiverSet.Generate` does, so row 0 is the southernmost.
 */
const ROW_ORDER_SOUTH_UP = "south-up";

async function sha256Hex(value: string): Promise<string> {
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((part) => part.toString(16).padStart(2, "0"))
    .join("");
}

/**
 * The two indicators this build computes, and their unit.
 *
 * Named once because the receiver table and the raster sidecar have to agree
 * on both: the sidecar's bands are the table's indicators, and a unit filed
 * under a name neither carries is refused by `results` on the CLI side.
 *
 * `"dB(A)"` rather than the CLI's `"dB"` is a pre-existing difference between
 * the two targets, left as it was — the map's decibel gate accepts either, and
 * changing it here would move browser runs' stored tables for no reason.
 */
const BROWSER_INDICATORS = ["LrDay", "LrNight"];

function browserUnits(): Record<string, string> {
  return Object.fromEntries(
    BROWSER_INDICATORS.map((indicator) => [indicator, "dB(A)"]),
  );
}

function buildReceiverTable(outputs: ReceiverOutput[]): ReceiverTable {
  return {
    indicator_order: BROWSER_INDICATORS,
    units: browserUnits(),
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
// how buildReceiverTableCSV already handles missing indicators.
function formatIndicator(values: Record<string, number>, key: string): string {
  return values[key]?.toFixed(1) ?? "";
}

/**
 * The CRS a stored run's receiver coordinates are in, read back off its own
 * run-summary artifact.
 *
 * A run made before browser mode projected anything carries no `compute_crs`
 * and its x/y are degrees, so it is labelled {@link CRS_NOT_RECORDED} rather
 * than given a CRS it was never computed in.
 */
function storedComputeCRS(storedRun: StoredRun): string {
  const ref = storedRun.run.artifacts.find(
    (artifact) => artifact.kind === "run.result.summary",
  );
  const summary = ref === undefined ? undefined : storedRun.artifacts[ref.id];
  const value = isRecord(summary?.value)
    ? summary.value["compute_crs"]
    : undefined;
  return typeof value === "string" && value !== "" ? value : CRS_NOT_RECORDED;
}

function browserExportHTML(
  run: RunSummary,
  table: ReceiverTable,
  computeCRS: string,
): string {
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
    <p class="meta">Receiver coordinates are in ${computeCRS}</p>
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

function browserExportMarkdown(
  run: RunSummary,
  table: ReceiverTable,
  computeCRS: string,
): string {
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

Receiver coordinates are in \`${computeCRS}\`.

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

/**
 * One browser-mode RLS-19 road run, from the model store to a persisted run.
 *
 * It is a free function rather than part of `startRun` because turning the
 * model into a scene is per standard, and still lives on this side of the WASM
 * boundary: the kernel takes a `ComputeRequest`, not a model. `startRun`
 * decides which extraction to call; this is the only one this build carries.
 */
async function runRLS19Road(
  kernel: AconiqKernel,
  spec: RunSpec,
): Promise<RunSummary> {
  // Project the whole workspace once, before anything reads a coordinate off
  // it, mirroring `cli.resolveComputeModel`. Never per builder:
  // `buildParkingSources` computes a shoelace area in m² and a centroid, and
  // `getFeatureBBox`/`getPolygonBBox` feed a receiver grid whose padding and
  // resolution are metres by contract. Every one of them needs the model
  // already in metres.
  const store = useModelStore.getState();
  const computeModel = await resolveComputeModel(kernel, {
    features: store.features,
    receivers: store.receivers,
    calcArea: store.calcArea,
    crs: store.crs,
  });
  const projection = computeModel.projection;

  const features = computeModel.features;
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
  let layout: BrowserGridLayout;

  if (spec.receiverMode === "custom") {
    const storeReceivers = computeModel.receivers;
    if (storeReceivers.length === 0) {
      throw new Error(
        "Custom receiver mode requires at least one receiver placed in the map workspace",
      );
    }
    // Model order, not id order: `extractExplicitReceivers`
    // (backend/internal/app/cli/run_input.go) walks `model.Features` and keeps
    // whatever order the model gives it. The receiver table's row order is
    // what `output_hash` is computed over, so sorting here made the same model
    // hash differently in the two targets.
    gridReceivers = storeReceivers.map((r) => ({
      id: r.id,
      point: { x: r.geometry.coordinates[0], y: r.geometry.coordinates[1] },
      height_m: r.heightM,
    }));
    // Not a grid, and the layout says so: explicit receivers are points the
    // user placed, and no cell size describes where they sit. The shape still
    // describes the raster truthfully — one column, one row per receiver.
    layout = { width: 1, height: storeReceivers.length };
  } else {
    const calcArea = computeModel.calcArea;
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
      throw new Error("Could not derive source extent from the current model");
    }
    const receiverGrid = buildReceiverGrid(bbox, spec.params);
    gridReceivers = receiverGrid.receivers;
    layout = {
      width: receiverGrid.width,
      height: receiverGrid.height,
      ...(receiverGrid.georeference
        ? { georeference: receiverGrid.georeference }
        : undefined),
    };
  }

  // The lock spans the compute, not just the persist: the id is minted
  // from the store before the kernel runs, and a second tab that allocated
  // in the meantime would mint the same id and have its run replaced by
  // `setRun` when both persist. Holding the lock until the persist keeps
  // the id unique while this tab is still computing.
  return withStoreLock(async () => {
    const startedAt = nowISO();
    const runId = nextRunID(await reloadState());
    const basePath = `${DEFAULT_PROJECT_PATH}/runs/${runId}`;

    const request: ComputeRequest = {
      receivers: gridReceivers,
      sources,
      barriers,
      // The kernel is told which CRS this scene is in; it cannot read one off
      // the coordinates, which are bare numbers by the time they cross. It
      // needs it for the terrain — a DTM has to be queried in the CRS its
      // raster was written in — and sending it always keeps that from
      // depending on whether a terrain happens to be loaded.
      projection: {
        project_crs: projection.projectCRS,
        compute_crs: projection.computeCRS,
        applied: projection.applied,
      },
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
    const receiverCSV = buildReceiverTableCSV(receiverTable);
    const rasterMetadata: RasterMetadata = {
      width: layout.width,
      height: layout.height,
      bands: 2,
      nodata: -9999,
      units: browserUnits(),
      band_names: BROWSER_INDICATORS,
      // Both mirror what `results.RasterMetadata` carries on the CLI side.
      // The CRS is the compute CRS, as it is there: results are expressed in
      // it, and the sidecar is the only place a consumer can ask.
      crs: projection.computeCRS,
      ...(layout.georeference
        ? { georeference: layout.georeference }
        : undefined),
    };
    // The real thing, not a digest of it. This slot held the run's SHA-256 hex
    // string — 64 characters where a float64 array belongs — because
    // `StoredArtifactContent` could not represent bytes at all, so nothing
    // could read back the raster a browser run had just computed.
    const rasterBinary = buildRasterBinary(rasterMetadata, [
      outputs.map((output) => output.Indicators.lr_day),
      outputs.map((output) => output.Indicators.lr_night),
    ]);
    const summary = {
      run_id: runId,
      status: "completed",
      grid_width: layout.width,
      grid_height: layout.height,
      source_count: sources.length,
      parking_source_count: parking.sources.length,
      receiver_count: outputs.length,
      reporting_precision_db: 0.1,
      // The CLI's own key names (`cli.provenanceProjectCRSKey` and
      // `provenanceComputeCRSKey`), because results are expressed in the
      // compute CRS on both targets and a consumer that only ever sees a
      // receiver table has to be able to learn which CRS it is in. Not on
      // `ReceiverTable`: the Go container has no CRS field, and adding one
      // browser-side would fork the format.
      project_crs: projection.projectCRS,
      compute_crs: projection.computeCRS,
      // Read off the kernel's own descriptor rather than named here. AGENTS.md
      // requires the tier to travel with the result — "a consumer that never
      // reads the docs still sees it" — and browser-mode summaries carried no
      // tier at all, so the map's evidence badge was blank in one of the two
      // shipped modes. Declaring it a second time is the exact duplication
      // `framework.StandardDescriptor.EvidenceTier` exists to prevent, which
      // is why this asks the kernel instead.
      evidence_tier: kernel
        .standards()
        .find((standard) => standard.id === spec.standardId)?.evidence_tier,
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
    // `results/<standard-id>.json` / `.bin`, the CLI's own naming: the raster
    // files are named after the standard that produced them
    // (`cli.endPersistSpec.export`), so a run bundle stays self-describing. It
    // read `rls19-road` literally, which was the same string only by accident.
    // Nothing selects these by path — every reader goes by artifact kind — but
    // a second standard would have written its raster under RLS-19's name.
    const rasterMetaArtifact = makeArtifact(
      runId,
      "raster-meta",
      "run.result.raster_metadata",
      `${basePath}/results/${spec.standardId}.json`,
      createdAt,
    );
    const rasterBinArtifact = makeArtifact(
      runId,
      "raster-bin",
      "run.result.raster_binary",
      `${basePath}/results/${spec.standardId}.bin`,
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
        projection.applied
          ? `${startedAt} compute_crs=${projection.computeCRS} (projected from ${projection.projectCRS})`
          : `${startedAt} compute_crs=${projection.computeCRS}`,
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
        encoding: "binary",
        value: null,
      },
      [summaryArtifact.id]: {
        kind: summaryArtifact.kind,
        mimeType: "application/json",
        encoding: "json",
        value: { ...summary, output_hash: outputHash },
      },
    };

    const storedRun: StoredRun = { run, log, artifacts: artifactMap };

    // The bytes go in first: a document referencing an artifact whose record
    // is missing reads as a corrupted run, while a byte record no document
    // names is merely orphaned.
    try {
      await saveArtifactBytes(rasterBinArtifact.id, rasterBinary);
    } catch (error) {
      // No eviction-and-retry here, unlike `persistRun`. Eviction is a
      // *document* write — it frees space by storing a smaller document, with
      // the dropped run's bytes removed in the same transaction — and this
      // write happens before there is any document change to pair it with.
      // Giving the raster its own evict-and-retry means writing the smaller
      // document first and the bytes after; PLAN.md carries it as open rather
      // than pretending the case is covered. What is in reach is the wording:
      // this is a storage failure like any other, and the dialogs render
      // `error.message`.
      throw storeFailure("run", error);
    }

    try {
      await persistRun(storedRun, "run");
    } catch (error) {
      // Nothing else will ever find these. `forgetArtifactBytes` walks the
      // runs the stored document holds, and a run whose persist failed is not
      // one of them — the next `reloadState` drops it from memory too. Left
      // behind, they would cost the origin a raster's worth of quota per
      // failed run, permanently, and on the quota path that is the very
      // resource that failed.
      forgetArtifactBytes([storedRun]);
      throw error;
    }

    return run;
  });
}

export const browserBackend = {
  capabilities: {
    kind: "browser",
    canExport: true,
    runsAgainstSavedModel: false,
    runsChangeExternally: false,
    // Export artifacts are stored inside the run record, so deleting the run
    // deletes the bundle with it. The confirmation has to say so.
    exportsOutliveRunDelete: false,
    // The kernel is already in memory by the time the user reaches the map —
    // `getHealth()` awaits `getKernel()` — so refusing to project here would
    // be refusing with the projector loaded.
    canReprojectForDisplay: true,
  },

  async transformCoordinates(
    req: TransformRequest,
  ): Promise<TransformResponse> {
    const kernel = await getKernel();
    return kernel.transform(req);
  },

  async getHealth(): Promise<HealthResponse> {
    await getKernel();
    return {
      status: "ok",
      version: "wasm-browser",
      time: nowISO(),
    };
  },

  /**
   * In browser mode the model store *is* the project, so the CRS it reports is
   * the store's. It used to be the constant `"WGS84 / web map"` — a display
   * label nothing parses, which said nothing about what the coordinates were in
   * and could not have, since the store held whatever the last import brought.
   */
  async getProjectStatus(): Promise<ProjectStatusResponse> {
    const current = await ensureLoaded();
    const { features, crs } = useModelStore.getState();
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
      crs,
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

  /**
   * The standards the kernel can run, as the kernel declares them.
   *
   * This used to be a hardcoded descriptor maintained by hand next to the Go
   * one, and it had drifted twice: it offered 9 of the 17 road surfaces, and
   * claimed line sources only, long after the module started accepting
   * Parkplatz areas. It also declared the evidence tier a second time, which is
   * the exact duplication `framework.StandardDescriptor.EvidenceTier` exists to
   * prevent.
   */
  async getStandards(): Promise<StandardDescriptor[]> {
    return (await getKernel()).standards();
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
    const content = findArtifact(await ensureLoaded(), artifactId);
    // Browser mode can answer for bytes here, and a caller written against
    // this mode alone does ask — so the branch stays. It delegates rather
    // than repeating the read: two readers of the same byte store would drift
    // on the missing-record case, and only one of them would say so.
    if (content.encoding === "binary") {
      return (await readArtifactBytes(artifactId)) as T;
    }
    return content.value as T;
  },

  async getArtifactBytes(artifactId: string): Promise<ArrayBuffer> {
    const content = findArtifact(await ensureLoaded(), artifactId);
    if (content.encoding !== "binary") {
      // Not a fallback to serialising the document value: a caller asking for
      // bytes is about to read them as float64, and JSON text reinterpreted
      // that way is noise rather than an error.
      throw new Error(
        `Artifact ${artifactId} holds ${content.encoding} content, not bytes; read it with getArtifactContent`,
      );
    }
    return readArtifactBytes(artifactId);
  },

  async getRunContours(
    runId: string,
    options: ContourOptions,
  ): Promise<RunContours> {
    const state = await ensureLoaded();
    const run = findRunByID(state, runId).run;

    const metadataArtifact = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.raster_metadata",
    );
    const binaryArtifact = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.raster_binary",
    );

    if (metadataArtifact === undefined || binaryArtifact === undefined) {
      // The same distinction the API route draws with `run_has_no_raster`: the
      // run is real, it simply placed receivers individually, and no interval
      // or CRS would change that.
      throw new Error(
        `Run ${runId} wrote no result raster, so it has no contours: ` +
          "only an auto-grid receiver mode produces one",
      );
    }

    const metadata = await browserBackend.getArtifactContent<RasterMetadata>(
      metadataArtifact.id,
    );
    const payload = await browserBackend.getArtifactBytes(binaryArtifact.id);

    // Every refusal past this point is the kernel's, in the contour package's
    // own words — the same words `aconiq export` and the API route use.
    const kernel = await getKernel();
    return kernel.contours(new Uint8Array(payload), {
      raster: metadata,
      target_crs: options.crs,
      ...(options.interval === undefined ? {} : { interval: options.interval }),
    });
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
    if (content.encoding === "binary") {
      // Binary content is not in the document, and this is synchronous
      // because pages put the result straight into `<iframe src>` and
      // `<a href>`. Nothing asks for a URL to a raster today; when something
      // does it needs an async path, not a blob minted from `null`.
      throw new Error(
        `Artifact ${artifactId} holds binary content; read it with getArtifactContent`,
      );
    }
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

    // Two refusals, because two different things can be missing, and saying
    // "not available" for both hid the second one.
    //
    // First: does the kernel publish this standard at all? Asked of
    // `kernel.standards()` rather than compared against a literal, so this gate
    // cannot disagree with the list `getStandards` serves — both come from the
    // Go descriptor, declared once. The literal was only ever right because the
    // published list happened to have one entry.
    const published = kernel
      .standards()
      .some((standard) => standard.id === spec.standardId);
    if (!published) {
      throw new Error(
        `Standard ${spec.standardId} is not available in browser mode`,
      );
    }

    // Second: does *this* build know how to extract a model for it? Publishing
    // a standard and being able to run it here are separate facts while the
    // extraction lives in TypeScript, so a kernel that grows a second entry must
    // be refused in words that cannot be mistaken for "the kernel cannot do it".
    // The TypeScript-side twin of Go's `TestEveryStandardHasItsEntryPoint`.
    switch (spec.standardId) {
      case "rls19-road":
        return runRLS19Road(kernel, spec);
      default:
        throw new Error(
          `The kernel publishes ${spec.standardId}, but this build has no model extraction for it`,
        );
    }
  },

  async createExport(runId: string): Promise<RunSummary> {
    // Held from the reload to the persist so the run read here is the one
    // the export is merged into.
    return withStoreLock(async () => {
      const storedRun = findRunByID(await reloadState(), runId);
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

      const computeCRS = storedComputeCRS(storedRun);
      const context = {
        exported_at: exportedAt,
        project_id: DEFAULT_PROJECT_ID,
        run: storedRun.run,
        compute_crs: computeCRS,
        receiver_table: receiverTable,
      };
      const html = browserExportHTML(storedRun.run, receiverTable, computeCRS);
      const markdown = browserExportMarkdown(
        storedRun.run,
        receiverTable,
        computeCRS,
      );
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

      await persistRun(nextStoredRun, "export");
      return nextStoredRun.run;
    });
  },

  async deleteRun(runId: string): Promise<DeleteRunResult> {
    // The lock spans the read and the write, like every other writer: the
    // store is shared by every tab of the origin, so deleting out of this
    // tab's cache would drop whatever another tab has stored since.
    //
    // Written through `persist`, which is what prunes the URL cache and
    // revokes the deleted run's artifact object URLs. Bypassing it leaks them.
    //
    // `runHighWaterMark` is untouched on purpose: it is the record of what has
    // been handed out, and deleting the newest run must not free its id.
    return withStoreLock(async () => {
      const current = await reloadState();
      const storedRun = findRunByID(current, runId);
      if (
        storedRun.run.status === "pending" ||
        storedRun.run.status === "running"
      ) {
        // The same refusal the API gives, for the same reason: the run is
        // still writing. Browser runs complete inside `startRun`, so this is a
        // guard rather than a case anyone reaches.
        throw new Error(`Run ${runId} is still running`);
      }
      await persist({
        state: {
          ...current,
          runs: current.runs.filter((entry) => entry.run.id !== runId),
        },
        // The raster bytes are keyed outside the document, so removing the run
        // from it reclaims nothing on its own: run, delete, repeat would fill
        // the origin's quota with rasters no run names. In the same
        // transaction, so a delete that does not commit still has a run with a
        // readable raster.
        dropped: [storedRun],
      });
      // No paths and no surviving bundle: the export artifacts lived inside
      // the record just removed.
      return { runId, retainedPaths: [] };
    });
  },

  /**
   * In browser mode the model store is the project, so there is nothing to
   * read back — and a `null` here would be indistinguishable from "the
   * project has no model yet" to a caller that forgot the capability gate,
   * which would then leave the map empty over a populated store. Rejecting
   * follows `httpBackend.createExport`: the method exists so the interface
   * has no mode-specific hole, and says why it cannot answer.
   */
  getModel(): Promise<ModelResponse | null> {
    return Promise.reject(
      new Error(
        "Reading the project model is not available in browser mode; the model store is the project",
      ),
    );
  },

  /**
   * In browser mode the model store is the project: a run reads it directly,
   * so there is nothing to write.
   *
   * `hash: null` rather than a computed digest. The hash is a receipt for a
   * file the server wrote; there is no file here, and inventing one would let
   * a draft claim to match a project that does not exist.
   */
  async saveModel(req: ModelSaveRequest): Promise<ModelSaveResult> {
    return {
      featureCount: req.model.features.length,
      warnings: [],
      hash: null,
    };
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
