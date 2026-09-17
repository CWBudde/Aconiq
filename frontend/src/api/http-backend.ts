import {
  ERROR_CODE_MODEL_NOT_FOUND,
  ERROR_CODE_NOT_FOUND,
  errorFromResponse,
  parseErrorEnvelope,
} from "./api-error";
import type { Backend, OsmImportRequest, RunSpec } from "./backend";
import { apiHeaders } from "./client";
import type {
  CreateRunRequest,
  DeleteRunResponse,
  HealthResponse,
  ModelResponse,
  ModelSaveResponse,
  ProjectStatusResponse,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import { apiURL } from "./mode";
import type { GeoJSONFeatureCollection } from "@/model/types";

interface RequestOptions {
  method?: "GET" | "POST" | "DELETE";
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
    runsChangeExternally: true,
    // The API deletes the run directory but keeps export bundles on disk and
    // names them in `retained_paths` — a bundle may already have been
    // delivered.
    exportsOutliveRunDelete: true,
    // No transform endpoint exists, and the WASM kernel is not loaded in this
    // mode. See `transformCoordinates` below.
    canReprojectForDisplay: false,
  },

  getHealth() {
    return request<HealthResponse>("/api/v1/health");
  },

  async getProjectStatus() {
    const response = await send("/api/v1/project/status");
    if (response.ok) {
      return (await response.json()) as ProjectStatusResponse;
    }
    // A `not_found` envelope means no project is initialised: a state the
    // welcome page renders, not a failure. Any other 404 — an HTML page from
    // a wrong base URL, an older server without the route — is one.
    if (response.status === 404) {
      const envelope = parseErrorEnvelope(
        await response
          .clone()
          .json()
          .catch(() => null),
      );
      if (envelope?.code === ERROR_CODE_NOT_FOUND) {
        return null;
      }
    }
    throw await errorFromResponse(response);
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

  async deleteRun(runId) {
    // 200 with a body rather than 204, so the UI can say the bundle was kept —
    // and because `request` always parses JSON.
    const response = await request<DeleteRunResponse>(
      `/api/v1/runs/${encodeURIComponent(runId)}`,
      { method: "DELETE" },
    );
    return { runId: response.run_id, retainedPaths: response.retained_paths };
  },

  async getModel(crs) {
    const response = await send(`/api/v1/model?crs=${encodeURIComponent(crs)}`);
    if (response.ok) {
      return (await response.json()) as ModelResponse;
    }
    // Only `model_not_found` is an answer rather than a failure: the project
    // loaded and holds no model yet. A `not_found` means the project itself
    // is gone, which is a different thing and is thrown — as is any other
    // 404, such as an older server without the route.
    if (response.status === 404) {
      const envelope = parseErrorEnvelope(
        await response
          .clone()
          .json()
          .catch(() => null),
      );
      if (envelope?.code === ERROR_CODE_MODEL_NOT_FOUND) {
        return null;
      }
    }
    throw await errorFromResponse(response);
  },

  transformCoordinates() {
    // The map never calls it (`canReprojectForDisplay` is false); the method
    // still exists so the interface has no mode-specific hole. `POST
    // /api/v1/transform` is a design decision of its own, and pulling the 4 MB
    // kernel into a mode that never otherwise loads it, to draw a map, is not
    // the way to avoid making it.
    return Promise.reject(
      new Error(
        "Coordinate projection is not available in API mode; save the model and reload, which refetches it in WGS84",
      ),
    );
  },

  async saveModel(req) {
    const saved = await postJSON<ModelSaveResponse>("/api/v1/model", req);
    // The hash is the server's receipt for the bytes it just wrote. Kept, not
    // discarded: it is what lets the next startup ask "is my draft still the
    // project's model?" without fetching anything.
    return {
      featureCount: saved.feature_count,
      warnings: saved.warnings,
      hash: saved.hash,
    };
  },
};
