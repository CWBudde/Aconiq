/**
 * The one seam between the UI and whatever computes for it.
 *
 * Two implementations exist: `httpBackend` talks to `aconiq serve`, and
 * `browserBackend` runs the Go kernel as WebAssembly against the in-memory
 * model store. Pages and hooks see only `Backend`, and where the two modes
 * genuinely differ they ask `backend.capabilities` rather than testing which
 * mode they are in — a capability says what the UI may do, which is the
 * question a page actually has.
 */

import type {
  APIValidationIssue,
  HealthResponse,
  LglnImportRequest,
  LglnImportResponse,
  ModelResponse,
  ModelSaveRequest,
  ProjectStatusResponse,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import { browserBackend } from "./browser-backend";
import { httpBackend } from "./http-backend";
import { IS_WASM_MODE } from "./mode";
import type { GeoJSONFeatureCollection } from "@/model/types";
import { CancelledErrorName } from "@/wasm/kernel-client";
import type { TransformRequest, TransformResponse } from "@/wasm/types";

export interface BackendCapabilities {
  /** Which implementation is behind the interface. Read it for labels, not for branching. */
  readonly kind: "http" | "browser";
  /**
   * The UI can generate an export bundle itself. When set, the export page
   * offers a Generate button and describes the bundle it will produce; when
   * not set, the page shows the `aconiq export` command to run instead. The
   * browser writes bundles into local storage; the API has no export
   * endpoint yet.
   */
  readonly canExport: boolean;
  /**
   * A run reads the model saved in the project rather than the in-memory
   * store. When set, the run dialog notes that explicit receivers come from
   * the saved model; when not set, the dialog warns about a custom receiver
   * mode with no receivers placed, and refuses to start such a run.
   */
  readonly runsAgainstSavedModel: boolean;
  /**
   * Runs change while the page is open without the UI having done anything:
   * the server executes them and their status moves on its own. When set,
   * the runs list polls; when not set, a run completes inside `startRun` and
   * the list is invalidated there, so polling would only re-read what the UI
   * itself wrote.
   */
  readonly runsChangeExternally: boolean;
  /**
   * An export bundle survives the deletion of the run it belongs to. The API
   * keeps the bytes on disk and reports them in `retainedPaths`; the browser
   * stores export artifacts inside the run record, so deleting the run takes
   * them with it.
   *
   * Read before the deletion, not after: the sentence this answers belongs in
   * the confirmation, and `retainedPaths` only arrives once the user has
   * already agreed.
   */
  readonly exportsOutliveRunDelete: boolean;
  /**
   * A projection is reachable, so a model stored in a projected CRS can be
   * moved into WGS84 for the map to draw it.
   *
   * Both shipped modes have one, and it is the same projection in both: browser
   * mode has the Go kernel in memory before the user reaches the map —
   * `getHealth()` awaits `getKernel()` — and API mode reaches
   * `POST /api/v1/transform`, which serves it over the wire rather than making
   * a mode that never otherwise loads the 4 MB kernel fetch it to draw a map.
   *
   * The flag stays all the same: it is a property of this interface, not of the
   * two implementations that happen to exist. A backend without a projector —
   * an offline or read-only one — is what it is here for, and the map still has
   * to say so rather than draw a model in the wrong place.
   */
  readonly canReprojectForDisplay: boolean;
  /**
   * A run in flight can be stopped, and stopping it really stops the compute.
   * When set, the run dialog turns its Cancel button into a run-cancel while a
   * run is pending; when not set, that button keeps closing the dialog.
   *
   * Browser mode can, because the kernel lives in a worker this tab owns and
   * terminating it ends the computation. The API cannot: it has no cancel
   * endpoint, and a button that only closed the dialog while the server kept
   * computing would be a lie — the run would still finish and still appear in
   * the list, having ignored the one instruction the user gave it.
   */
  readonly runsAreCancellable: boolean;
  /**
   * The official LGLN LoD2 buildings can be loaded for a box. When set, the
   * import page's LGLN tab is live; when not set, the tab is shown disabled
   * and says it needs `aconiq serve`.
   *
   * Only the API can: a tile is a ~50 MB CityGML download that `aconiq serve`
   * fetches, parses with the Go importer and caches for every later request.
   * The WASM kernel carries no CityGML reader and the tab no tile cache.
   */
  readonly canImportLGLN: boolean;
}

/**
 * What a caller may attach to a run beyond the run's own inputs.
 *
 * A second argument rather than fields on {@link RunSpec}: a spec is the value
 * `buildCreateRunRequest` maps onto the API body, and neither a callback nor a
 * signal is data a run can be described by.
 */
export interface RunHooks {
  /**
   * Called with the receivers computed so far, out of how many there are.
   * Never called by `httpBackend` — the API reports no progress — so a UI that
   * wants a bar has to survive never hearing from this at all.
   */
  onProgress?: (done: number, total: number) => void;
  /**
   * Aborting terminates the kernel, which is the only way to stop a WASM
   * computation: the module has one stack and no cancellation point, so a run
   * ends when its thread does.
   *
   * Only honoured when `capabilities.runsAreCancellable`. A backend that
   * ignores it is not misbehaving — it is saying it cannot stop — which is why
   * the capability, and not the presence of a signal, is what the UI reads
   * before it offers a Cancel button.
   */
  signal?: AbortSignal;
}

/**
 * Whether a rejected run was cancelled rather than failed.
 *
 * Here rather than at the call sites so the UI never reaches into `@/wasm/`:
 * which `Error.name` a terminated kernel produces is the worker protocol's
 * business, and a page that imported it would be a page to edit when the
 * protocol changes. The distinction earns its keep — a cancellation is not a
 * failure to report, so it is the difference between a red callout and a
 * neutral one.
 */
export function isRunCancelled(error: unknown): boolean {
  return error instanceof Error && error.name === CancelledErrorName;
}

/** What the UI needs from a run deletion, mode-independent. */
/** What a caller may vary about a contour request. */
export interface ContourOptions {
  /** The dB step between levels. Omitted, the backend uses 5 dB (EU END). */
  interval?: number;
  /** The CRS to return the vertices in. */
  crs: string;
}

/**
 * A run's contours, and the CRS they are in.
 *
 * `crs` is always populated. A consumer handed bare lines guesses WGS84, which
 * for a metric run is the site off the coast of Africa. `interval` is echoed
 * because the caller may have omitted it, and a legend naming the step has to
 * know which step it got.
 */
export interface RunContours {
  crs: string;
  interval: number;
  lines: ContourLine[];
}

/** One contour at one dB level, in {@link RunContours.crs}. */
export interface ContourLine {
  level: number;
  band_name: string;
  points: [number, number][];
}

export interface DeleteRunResult {
  runId: string;
  /**
   * Files kept although the run is gone — export bundles, which may already
   * have been delivered. Always empty in browser mode, which has no paths and
   * keeps no bundle; `removed_paths` is deliberately not carried across,
   * because it would have to be invented there.
   */
  retainedPaths: string[];
}

/** What the UI needs from a model save, mode-independent. */
export interface ModelSaveResult {
  featureCount: number;
  /** Validation warnings; the model was saved despite them. */
  warnings: APIValidationIssue[];
  /**
   * The receipt the API answers with: the SHA-256 of the file just written.
   * `null` where there is no project file to be a receipt for (browser mode).
   *
   * `string | null` rather than `""` on purpose — "this draft has a receipt"
   * is then a null check, not a truthiness test on a hex string. The client
   * never computes one itself: a re-serialised model is not the same bytes.
   */
  hash: string | null;
}

/** What the run dialog collects; `buildCreateRunRequest` maps it onto the API body. */
export interface RunSpec {
  standardId: string;
  version: string;
  profile: string;
  params: Record<string, string>;
  receiverMode: "auto-grid" | "custom";
  /**
   * The caller acknowledges that the selected standard is scaffold tier. It is
   * derived from the tier of the standard being run, never carried over from a
   * previous choice, and reaches the API as `experimental`. Browser mode runs
   * only `rls19-road`, which is normative, so it is never set there.
   */
  experimental?: boolean;
}

/** Bounding box for an Overpass import, in WGS84 degrees. */
export interface OsmImportRequest {
  south: number;
  west: number;
  north: number;
  east: number;
  overpass_endpoint?: string;
}

export interface Backend {
  readonly capabilities: BackendCapabilities;
  getHealth(): Promise<HealthResponse>;
  /** `null` when no project is initialised (the API answers 404). */
  getProjectStatus(): Promise<ProjectStatusResponse | null>;
  getStandards(): Promise<StandardDescriptor[]>;
  getRuns(): Promise<RunSummary[]>;
  getRunLog(runId: string): Promise<RunLog>;
  /**
   * The artifact's content decoded as JSON, or — in browser mode, where the
   * value is stored rather than fetched — as the stored text. It is the
   * JSON/text reader: bytes are `getArtifactBytes`'s business, whatever
   * browser mode happens to tolerate here.
   */
  getArtifactContent<T>(artifactId: string): Promise<T>;
  /**
   * The raw bytes of a binary artifact — today only `run.result.raster_binary`.
   * Rejects for an artifact whose content is not binary.
   *
   * A separate method rather than a widening of `getArtifactContent<T>`,
   * because the two modes disagree about what that method can return:
   * `httpBackend` parses every response as JSON, and a headerless float64
   * array is not JSON. A `getArtifactContent<ArrayBuffer>` that answers in
   * browser mode and throws a SyntaxError in API mode is worse than two
   * honest methods.
   *
   * No capability flag guards it: both modes can do it, and a capability says
   * what the UI may do — there is no page here that would offer one mode a
   * button the other cannot serve.
   */
  getArtifactBytes(artifactId: string): Promise<ArrayBuffer>;
  /**
   * A run's noise contours, traced from its result raster.
   *
   * Both modes answer from the same Go marching squares — API mode over
   * `GET /api/v1/runs/{id}/contours`, browser mode through the WASM kernel —
   * so the two cannot put a 55 dB line in different places. The frontend
   * deliberately has no tracer of its own, for the reason it has no projection
   * of its own.
   *
   * The two modes ask differently and that is correct rather than tolerated:
   * the server is holding the raster on disk and the browser is holding it in
   * IndexedDB, so each asks with what it has. What must not differ is the
   * answer, which is why both return the same shape.
   *
   * No capability flag guards it: both modes can do it, and a capability says
   * what the UI may offer. That was not true before the kernel gained the
   * export — which is exactly why the layer control had no contours toggle.
   *
   * Rejects when the run wrote no raster, when its receivers were not a grid,
   * or when the contours cannot be moved into `crs`. Each refusal carries the
   * contour package's own words.
   */
  getRunContours(runId: string, options: ContourOptions): Promise<RunContours>;
  /** A URL the browser can open or download the artifact from. */
  getArtifactURL(artifactId: string): string;
  importFromOSM(req: OsmImportRequest): Promise<GeoJSONFeatureCollection>;
  /**
   * The LGLN LoD2 buildings whose footprint centroid lies inside the box.
   * Rejects unless `capabilities.canImportLGLN`. Can take tens of seconds the
   * first time a tile is touched: the server downloads it before it answers.
   */
  importFromLGLN(req: LglnImportRequest): Promise<LglnImportResponse>;
  /**
   * Start a run and resolve with it once it has finished.
   *
   * `hooks` is optional in both directions: a caller need not pass one, and an
   * implementation need not honour what it holds. What it must not do is
   * pretend — see `capabilities.runsAreCancellable`.
   */
  startRun(spec: RunSpec, hooks?: RunHooks): Promise<RunSummary>;
  /** Rejects unless `capabilities.canExport`. */
  createExport(runId: string): Promise<RunSummary>;
  /**
   * Removes a run and everything it wrote. Refused while the run is still
   * `pending` or `running` — the API answers 409 `run_not_finished`.
   */
  deleteRun(runId: string): Promise<DeleteRunResult>;
  /**
   * The model saved in the project, in `crs`. `null` when the project holds
   * no model yet (`model_not_found`) — which is not the same refusal as a
   * missing project (`not_found`), and that one is rethrown.
   *
   * `crs` is required, not optional. Omitted, the API answers in the project
   * CRS — metres in EPSG:25832 for a typical German project — and the map,
   * which draws in WGS84, would place the model a few hundred metres off the
   * coast of Africa. Making the caller name the CRS is what keeps that from
   * being the default.
   *
   * Rejects unless `capabilities.runsAgainstSavedModel`: where the store is
   * the project there is nothing to read back, and a `null` would be absorbed
   * silently by a caller that forgot the capability gate.
   */
  getModel(crs: string): Promise<ModelResponse | null>;
  /** Replace the project model; refused with `model_invalid` when validation fails. */
  saveModel(req: ModelSaveRequest): Promise<ModelSaveResult>;
  /**
   * Project a flat, interleaved batch of coordinates between two CRS.
   *
   * Rejects unless `capabilities.canReprojectForDisplay`. The one projection
   * the frontend has — `internal/geo/crstransform`, in process in browser mode
   * and over the wire in API mode — so neither can place a model where
   * `aconiq run` would not compute it.
   */
  transformCoordinates(req: TransformRequest): Promise<TransformResponse>;
}

/**
 * Selected here rather than in `mode.ts`: `http-backend.ts` imports the URL
 * helpers from `mode.ts`, so selecting there would close an import cycle.
 */
export const backend: Backend = IS_WASM_MODE ? browserBackend : httpBackend;
