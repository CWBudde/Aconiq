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
