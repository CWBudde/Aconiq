import type { GeoJSONFeatureCollection } from "@/model/types";

/**
 * The custom request header the local API requires on every state-changing
 * method. A CORS simple request cannot carry a custom header, so sending one
 * forces a preflight, which is what makes a cross-site write visible to the
 * server's origin allowlist. The server never reads the value — only its
 * presence counts.
 */
export const CLIENT_HEADER_NAME = "X-Aconiq-Client";

const CLIENT_HEADER_VALUE = "aconiq-web";

/**
 * Optional bearer token, matching `aconiq serve --api-token`. Empty unless the
 * server was started with one, in which case every request must present it.
 */
const API_TOKEN = import.meta.env.VITE_API_TOKEN ?? "";

/**
 * Headers every request to the local API carries. `extra` wins over the
 * defaults, so a caller can set its own Content-Type without restating the
 * rest.
 */
export function apiHeaders(
  extra?: Readonly<Record<string, string>>,
): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    [CLIENT_HEADER_NAME]: CLIENT_HEADER_VALUE,
    ...extra,
  };

  if (API_TOKEN !== "") {
    headers.Authorization = `Bearer ${API_TOKEN}`;
  }

  return headers;
}

export interface APIError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  hint?: string;
}

export interface ErrorEnvelope {
  error: APIError;
}

export interface HealthResponse {
  status: string;
  version: string;
  time: string;
}

export interface LastRunStatus {
  id: string;
  status: string;
  /**
   * Which assessment question the standard answers: `planning` for an
   * individual project's approval case, `mapping` for area-wide strategic
   * noise mapping. Typed as a plain string because older backends omit it.
   */
  context?: string;
  standard_id: string;
  version: string;
  profile?: string;
  started_at: string;
  finished_at: string;
}

export interface ProjectStatusResponse {
  project_id: string;
  name: string;
  project_path: string;
  manifest_version: number;
  crs: string;
  scenario_count: number;
  run_count: number;
  last_run?: LastRunStatus;
  /** Absent until a model has been saved. */
  model?: ProjectModelStatus;
}

/**
 * The saved model's receipt (schema `ProjectModelStatus`). A client that kept
 * the `hash` from its last `POST /api/v1/model` compares the two as strings to
 * learn whether its local draft is still the project's model, without fetching
 * anything. Never recompute the hash locally: a re-serialised model is not the
 * same bytes.
 */
export interface ProjectModelStatus {
  hash: string;
  updated_at: string;
}

export interface ArtifactRef {
  id: string;
  kind: string;
  path: string;
  created_at: string;
}

export interface RunSummary {
  id: string;
  scenario_id: string;
  /**
   * Which assessment question the standard answers: `planning` for an
   * individual project's approval case, `mapping` for area-wide strategic
   * noise mapping. Typed as a plain string because older backends omit it.
   */
  context?: string;
  standard_id: string;
  version: string;
  profile?: string;
  receiver_mode?: string;
  receiver_set_id?: string;
  status: "pending" | "running" | "completed" | "failed";
  started_at: string;
  finished_at: string;
  log_path: string;
  artifacts: ArtifactRef[];
}

export interface CreateRunRequest {
  scenario_id?: string;
  standard_id?: string;
  standard_version?: string;
  standard_profile?: string;
  model_path?: string;
  receiver_mode?: "auto-grid" | "custom";
  params?: Record<string, string>;
  input_paths?: string[];
  /**
   * Acknowledges that the selected standard is scaffold tier: it carries no
   * normative coefficients, its base levels are invented and it has no octave
   * bands. The API refuses a run against such a standard without it, with
   * error code `experimental_opt_in_required`.
   *
   * Omitted rather than sent as `false`, matching the Go struct's
   * `json:"experimental,omitempty"`.
   */
  experimental?: boolean;
}

export interface RunLog {
  run_id: string;
  lines: string[];
}

/**
 * Response of `DELETE /api/v1/runs/{id}` (schema `DeleteRunResponse`). Both
 * arrays are always present, empty rather than null.
 *
 * A run that is still `pending` or `running` is refused with 409 and error code
 * `run_not_finished` — its directory is still being written.
 */
export interface DeleteRunResponse {
  run_id: string;
  /** Project-relative paths that were deleted. */
  removed_paths: string[];
  /**
   * Files whose manifest refs were dropped but whose bytes were deliberately
   * left in place — export bundles. A bundle may already have been delivered,
   * so deleting a run never deletes one.
   */
  retained_paths: string[];
}

export interface ParameterDefinition {
  name: string;
  kind: "string" | "bool" | "int" | "float";
  /**
   * Physical unit of the value as a short symbol — "m", "km/h", "dB",
   * "dB/km", "1/h", "1/km", "%", "°C", "°". Absent when the parameter is
   * dimensionless (a share, a factor, a count) or not numeric at all.
   *
   * The symbol is the conventional SI-style spelling, not the suffix the
   * parameter name happens to use: `speed_pkw_kph` carries "km/h". Declared by
   * `framework.ParameterDefinition.Unit` in the Go modules and published on
   * `GET /api/v1/standards`; optional because older backends omit it.
   */
  unit?: string;
  required: boolean;
  default_value?: string;
  description?: string;
  enum?: string[];
  min?: number;
  max?: number;
}

export interface ProfileInfo {
  name: string;
  supported_source_types: string[];
  supported_indicators: string[];
  parameters: ParameterDefinition[];
}

export interface VersionInfo {
  name: string;
  default_profile: string;
  profiles: ProfileInfo[];
}

export interface StandardDescriptor {
  /**
   * Which assessment question the standard answers: `planning` for an
   * individual project's approval case, `mapping` for area-wide strategic
   * noise mapping. Typed as a plain string because older backends omit it.
   */
  context?: string;
  id: string;
  description: string;
  default_version: string;
  versions: VersionInfo[];
  /**
   * How much a module's output can be trusted: `normative`, `preview`,
   * `scaffold` or `test-fixture`. Optional and deliberately typed as a plain
   * string — older backends omit the field, and newer ones may report a tier
   * this build does not know yet. Narrow it with `parseEvidenceTier` rather
   * than comparing raw strings.
   */
  evidence_tier?: string;
}

export interface ReceiverRecord {
  id: string;
  x: number;
  y: number;
  height_m: number;
  values: Record<string, number>;
}

export interface ReceiverTable {
  indicator_order: string[];
  unit: string;
  records: ReceiverRecord[];
}

export interface RasterMetadata {
  width: number;
  height: number;
  bands: number;
  nodata: number;
  unit: string;
  band_names?: string[];
}

/**
 * Body of `POST /api/v1/model` (schema `ModelSaveRequest`). The model is a
 * GeoJSON FeatureCollection in the v1 input schema, handed over unparsed.
 */
export interface ModelSaveRequest {
  /**
   * CRS of the model's coordinates, e.g. `EPSG:4326` for coordinates drawn on
   * a web map. When omitted the coordinates are taken to be in the project CRS
   * already.
   */
  crs?: string;
  model: GeoJSONFeatureCollection;
}

/**
 * One validation finding as the API reports it (schema `ValidationIssue`).
 * Named apart from the model store's own `ValidationIssue`, which carries a
 * level and camel-cased fields; this one is the wire shape.
 */
export interface APIValidationIssue {
  code: string;
  message: string;
  /** The feature the finding is about; absent for model-wide findings. */
  feature_id?: string;
}

/**
 * Response of a successful model save (schema `ModelSaveResponse`). A model
 * with validation errors is refused instead, with error code `model_invalid`
 * and the findings under `details.errors` as `APIValidationIssue[]`.
 */
export interface ModelSaveResponse {
  normalized_path: string;
  dump_path: string;
  validation_report_path: string;
  feature_count: number;
  /**
   * Receipt for what was just written — the SHA-256 of the normalized file, as
   * bare lowercase hex. Store it beside the local draft and compare it against
   * `ProjectStatusResponse.model.hash` later.
   */
  hash: string;
  /** Validation warnings. The model was written despite them. */
  warnings: APIValidationIssue[];
}

/**
 * Response of `GET /api/v1/model` (schema `ModelResponse`). A project that
 * loaded but has no model yet is refused with error code `model_not_found`,
 * which is deliberately not the `not_found` a missing project produces.
 */
export interface ModelResponse {
  /**
   * The CRS the returned coordinates are actually in: the one asked for via
   * `?crs=`, or the project CRS when none was asked for.
   */
  crs: string;
  project_crs: string;
  /** Receipt for the stored file. Unaffected by a reprojection. */
  hash: string;
  feature_count: number;
  model: GeoJSONFeatureCollection;
}
