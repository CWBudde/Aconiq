import { errorFromResponse } from "./api-error";
import type { Backend, OsmImportRequest, RunSpec } from "./backend";
import { apiHeaders } from "./client";
import type {
  CreateRunRequest,
  HealthResponse,
  ModelSaveResponse,
  ProjectStatusResponse,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import { apiURL } from "./mode";
import type { GeoJSONFeatureCollection } from "@/model/types";

interface RequestOptions {
  method?: "GET" | "POST";
  headers?: Readonly<Record<string, string>>;
  body?: string;
}

/** Send one request to the local API with the headers every request carries. */
function send(path: string, options: RequestOptions = {}): Promise<Response> {
  return fetch(apiURL(path), {
    ...(options.method === undefined ? {} : { method: options.method }),
    ...(options.body === undefined ? {} : { body: options.body }),
    headers: apiHeaders(options.headers),
  });
}

/**
 * Send a request and parse the JSON body. Every non-OK response is thrown
 * whole through `errorFromResponse`: an envelope surfaces as
 * `APIRequestError` with its `code`, `hint` and `details`, and anything else
 * as a plain status error — never a fabricated code.
 */
async function request<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const response = await send(path, options);
  if (!response.ok) {
    throw await errorFromResponse(response);
  }
  return (await response.json()) as T;
}

function postJSON<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/**
 * Translate a run spec into the API request body.
 *
 * `experimental` is omitted unless the spec sets it, so a run against a
 * standard that needs no acknowledgement never carries one — matching the
 * `omitempty` on the Go struct rather than sending an explicit `false`.
 */
export function buildCreateRunRequest(spec: RunSpec): CreateRunRequest {
  return {
    standard_id: spec.standardId,
    standard_version: spec.version,
    standard_profile: spec.profile,
    receiver_mode: spec.receiverMode,
    params: spec.params,
    ...(spec.experimental === true ? { experimental: true } : {}),
  };
}

export const httpBackend: Backend = {
  capabilities: {
    kind: "http",
    canExport: false,
    runsAgainstSavedModel: true,
  },

  getHealth() {
    return request<HealthResponse>("/api/v1/health");
  },

  async getProjectStatus() {
    const response = await send("/api/v1/project/status");
    // 404, and only 404, means no project is initialised: that is a state the
    // welcome page renders, not a failure.
    if (response.status === 404) {
      return null;
    }
    if (!response.ok) {
      throw await errorFromResponse(response);
    }
    return (await response.json()) as ProjectStatusResponse;
  },

  getStandards() {
    return request<StandardDescriptor[]>("/api/v1/standards");
  },

  getRuns() {
    return request<RunSummary[]>("/api/v1/runs");
  },

  getRunLog(runId) {
    return request<RunLog>(`/api/v1/runs/${runId}/log`);
  },

  getArtifactContent<T>(artifactId: string) {
    return request<T>(`/api/v1/artifacts/${artifactId}/content`);
  },

  getArtifactURL(artifactId) {
    return apiURL(`/api/v1/artifacts/${artifactId}/content`);
  },

  importFromOSM(req: OsmImportRequest) {
    return postJSON<GeoJSONFeatureCollection>("/api/v1/import/osm", req);
  },

  startRun(spec) {
    return postJSON<RunSummary>("/api/v1/runs", buildCreateRunRequest(spec));
  },

  createExport() {
    // The page never offers the button (`canExport` is false); the method
    // still exists so the interface has no mode-specific hole.
    return Promise.reject(
      new Error(
        "Export generation from the UI is not available in API mode; run `aconiq export` instead",
      ),
    );
  },

  saveModel(req) {
    return postJSON<ModelSaveResponse>("/api/v1/model", req);
  },
};
