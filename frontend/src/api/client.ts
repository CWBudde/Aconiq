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

export interface APIClientOptions {
  baseURL: string;
  fetchImpl?: typeof fetch;
}

export class APIClient {
  private readonly baseURL: string;
  private readonly fetchImpl: typeof fetch;

  constructor(options: APIClientOptions) {
    this.baseURL = options.baseURL.replace(/\/$/, "");
    this.fetchImpl = options.fetchImpl ?? fetch;
  }

  async getHealth(): Promise<HealthResponse> {
    return this.requestJSON<HealthResponse>("/api/v1/health");
  }

  async getProjectStatus(): Promise<ProjectStatusResponse> {
    return this.requestJSON<ProjectStatusResponse>("/api/v1/project/status");
  }

  eventsURL(): string {
    return this.baseURL + "/api/v1/events";
  }

  private async requestJSON<T>(path: string): Promise<T> {
    const response = await this.fetchImpl(this.baseURL + path, {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
    });

    if (!response.ok) {
      const payload = (await response
        .json()
        .catch(() => null)) as ErrorEnvelope | null;
      if (payload?.error.message) {
        throw new Error(`${payload.error.code}: ${payload.error.message}`);
      }
      throw new Error(`Request failed: ${String(response.status)}`);
    }

    return (await response.json()) as T;
  }
}
