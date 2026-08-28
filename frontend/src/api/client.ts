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

export interface ParameterDefinition {
  name: string;
  kind: "string" | "bool" | "int" | "float";
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
