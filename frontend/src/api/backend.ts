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
}

/** What the UI needs from a model save, mode-independent. */
export interface ModelSaveResult {
  featureCount: number;
  /** Validation warnings; the model was saved despite them. */
  warnings: APIValidationIssue[];
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
  getArtifactContent<T>(artifactId: string): Promise<T>;
  /** A URL the browser can open or download the artifact from. */
  getArtifactURL(artifactId: string): string;
  importFromOSM(req: OsmImportRequest): Promise<GeoJSONFeatureCollection>;
  startRun(spec: RunSpec): Promise<RunSummary>;
  /** Rejects unless `capabilities.canExport`. */
  createExport(runId: string): Promise<RunSummary>;
  /** Replace the project model; refused with `model_invalid` when validation fails. */
  saveModel(req: ModelSaveRequest): Promise<ModelSaveResult>;
}

/**
 * Selected here rather than in `mode.ts`: `http-backend.ts` imports the URL
 * helpers from `mode.ts`, so selecting there would close an import cycle.
 */
export const backend: Backend = IS_WASM_MODE ? browserBackend : httpBackend;
