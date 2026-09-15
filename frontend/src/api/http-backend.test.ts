import { afterEach, describe, expect, it, vi } from "vitest";
import {
  APIRequestError,
  ERROR_CODE_EXPORT_INSIDE_RUN,
  ERROR_CODE_NOT_FOUND,
  asAPIRequestError,
} from "./api-error";
import type {
  ModelResponse,
  ModelSaveRequest,
  ModelSaveResponse,
} from "./client";
import { CLIENT_HEADER_NAME } from "./client";
import { httpBackend } from "./http-backend";
import { apiURL } from "./mode";

/**
 * Every refusal the local API sends is an envelope, and the envelope is what a
 * page needs to explain the refusal. These pin that the HTTP backend never
 * collapses one into a bare message — for the reads and the writes alike —
 * and that a non-envelope failure still surfaces as a status error rather
 * than a fabricated code.
 */

type FetchMock = ReturnType<typeof vi.fn<typeof fetch>>;

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(code: string, message: string, hint?: string): unknown {
  return { error: { code, message, ...(hint === undefined ? {} : { hint }) } };
}

function stubFetch(response: Response): FetchMock {
  const mock = vi.fn<typeof fetch>().mockResolvedValue(response);
  vi.stubGlobal("fetch", mock);
  return mock;
}

/** The request `fetch` was called with, as `[url, init]`. */
function requestOf(mock: FetchMock): [string, RequestInit] {
  const call = mock.mock.calls[0];
  if (call === undefined) throw new Error("fetch was not called");
  const [input, init] = call;
  const url =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return [url, init ?? {}];
}

function headersOf(init: RequestInit): Record<string, string> {
  const headers = init.headers;
  if (headers === undefined || Array.isArray(headers)) {
    throw new Error("expected a headers record");
  }
  if (headers instanceof Headers) {
    return Object.fromEntries(headers.entries());
  }
  return headers;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("httpBackend capabilities", () => {
  it("cannot export from the UI and runs against the saved model", () => {
    expect(httpBackend.capabilities).toEqual({
      kind: "http",
      canExport: false,
      runsAgainstSavedModel: true,
      runsChangeExternally: true,
      exportsOutliveRunDelete: true,
    });
  });
});

describe("httpBackend.deleteRun", () => {
  it("sends DELETE to the run resource and reports what was kept", async () => {
    const mock = stubFetch(
      jsonResponse({
        run_id: "run-0007",
        removed_paths: [".noise/runs/run-0007"],
        retained_paths: [".noise/exports/run-0007-bundle.json"],
      }),
    );

    const result = await httpBackend.deleteRun("run-0007");

    const [url, init] = requestOf(mock);
    expect(url).toBe(apiURL("/api/v1/runs/run-0007"));
    expect(init.method).toBe("DELETE");
    // The guard on state-changing methods
    // (backend/internal/api/httpv1/security.go) rejects a request without it.
    expect(headersOf(init)).toHaveProperty(CLIENT_HEADER_NAME);
    // `removed_paths` is deliberately not carried into the UI: it has no
    // meaning in browser mode, and a `[]` there would be a small lie.
    expect(result).toEqual({
      runId: "run-0007",
      retainedPaths: [".noise/exports/run-0007-bundle.json"],
    });
  });

  it("escapes an id that would otherwise change the path", async () => {
    const mock = stubFetch(
      jsonResponse({ run_id: "a/b", removed_paths: [], retained_paths: [] }),
    );

    await httpBackend.deleteRun("a/b");

    expect(requestOf(mock)[0]).toBe(apiURL("/api/v1/runs/a%2Fb"));
  });

  it("keeps the envelope of a refusal", async () => {
    stubFetch(
      jsonResponse(
        envelope(
          ERROR_CODE_EXPORT_INSIDE_RUN,
          "export bundle lives inside the run directory",
          "Move the export bundle out of .noise/runs/, then delete the run.",
        ),
        409,
      ),
    );

    const error = await httpBackend
      .deleteRun("run-0007")
      .catch((e: unknown) => e);
    const apiError = asAPIRequestError(error);

    expect(apiError?.code).toBe(ERROR_CODE_EXPORT_INSIDE_RUN);
    expect(apiError?.hint).toContain("Move the export bundle out");
  });
});

describe("httpBackend error handling", () => {
  it("keeps the envelope of a refused read", async () => {
    stubFetch(
      jsonResponse(
        envelope(ERROR_CODE_NOT_FOUND, "no project here", "Run aconiq init."),
        404,
      ),
    );

    const error = await httpBackend.getRuns().catch((e: unknown) => e);
    const apiError = asAPIRequestError(error);

    expect(apiError).toBeInstanceOf(APIRequestError);
    expect(apiError?.code).toBe(ERROR_CODE_NOT_FOUND);
    expect(apiError?.hint).toBe("Run aconiq init.");
    expect(apiError?.message).toBe("no project here");
  });

  it("keeps the envelope of a refused OSM import", async () => {
    stubFetch(
      jsonResponse(
        envelope("bbox_too_large", "bounding box exceeds the limit"),
        400,
      ),
    );

    const error = await httpBackend
      .importFromOSM({ south: 1, west: 2, north: 3, east: 4 })
      .catch((e: unknown) => e);
    const apiError = asAPIRequestError(error);

    expect(apiError).toBeInstanceOf(APIRequestError);
    expect(apiError?.code).toBe("bbox_too_large");
    expect(apiError?.message).toBe("bounding box exceeds the limit");
  });

  it("reports the status for a failure that is not an envelope", async () => {
    stubFetch(new Response("<html>gateway</html>", { status: 500 }));

    const error = await httpBackend.getRuns().catch((e: unknown) => e);

    expect(error).toBeInstanceOf(Error);
    expect(asAPIRequestError(error)).toBeNull();
    expect((error as Error).message).toBe("Request failed: 500");
  });
});

describe("httpBackend.getProjectStatus", () => {
  it("reads a not_found envelope as no project", async () => {
    stubFetch(
      jsonResponse(envelope(ERROR_CODE_NOT_FOUND, "no project here"), 404),
    );

    await expect(httpBackend.getProjectStatus()).resolves.toBeNull();
  });

  it("throws for a 404 that is not the API's envelope", async () => {
    // A wrong base-URL override or an older server without the route answers
    // 404 too; neither means "no project yet".
    stubFetch(new Response("<html>not found</html>", { status: 404 }));

    await expect(httpBackend.getProjectStatus()).rejects.toThrow(
      "Request failed: 404",
    );
  });

  it("throws for a 404 envelope with another code", async () => {
    stubFetch(jsonResponse(envelope("route_missing", "no such route"), 404));

    const error = await httpBackend.getProjectStatus().catch((e: unknown) => e);

    expect(asAPIRequestError(error)?.code).toBe("route_missing");
  });

  it("throws for any other failure", async () => {
    stubFetch(new Response("boom", { status: 500 }));

    await expect(httpBackend.getProjectStatus()).rejects.toThrow(
      "Request failed: 500",
    );
  });
});

describe("httpBackend.saveModel", () => {
  const request: ModelSaveRequest = {
    crs: "EPSG:4326",
    model: { type: "FeatureCollection", features: [] },
  };
  const saved: ModelSaveResponse = {
    normalized_path: ".noise/model/normalized.geojson",
    dump_path: ".noise/model/dump.json",
    validation_report_path: ".noise/model/validation-report.json",
    feature_count: 3,
    // The receipt the server returns for what it just wrote. It is stored
    // beside the draft, so a restored draft can prove it equals the project
    // by comparing strings rather than fetching the model.
    hash: "78179c43f885b7df906afef9613af46caa688bc66f4af97633f62ea4299a688e",
    warnings: [{ code: "short_segment", message: "segment under 1 m" }],
  };

  it("posts the model as JSON with the client header and maps the response", async () => {
    const fetchMock = stubFetch(jsonResponse(saved, 201));

    const response = await httpBackend.saveModel(request);

    const [url, init] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/model"));
    expect(init.method).toBe("POST");
    expect(init.body).toBe(JSON.stringify(request));
    const headers = headersOf(init);
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers[CLIENT_HEADER_NAME]).toBe("aconiq-web");
    expect(response).toEqual({
      featureCount: 3,
      warnings: [{ code: "short_segment", message: "segment under 1 m" }],
      hash: saved.hash,
    });
  });

  it("keeps the validation findings of a refused model", async () => {
    stubFetch(
      jsonResponse(
        {
          error: {
            code: "model_invalid",
            message: "model failed validation",
            details: {
              errors: [
                {
                  code: "missing_height",
                  message: "height_m is required",
                  feature_id: "b1",
                },
              ],
            },
          },
        },
        400,
      ),
    );

    const error = await httpBackend.saveModel(request).catch((e: unknown) => e);
    const apiError = asAPIRequestError(error);

    expect(apiError?.code).toBe("model_invalid");
    expect(apiError?.details).toEqual({
      errors: [
        {
          code: "missing_height",
          message: "height_m is required",
          feature_id: "b1",
        },
      ],
    });
  });
});

describe("httpBackend.getModel", () => {
  const stored: ModelResponse = {
    crs: "EPSG:4326",
    project_crs: "EPSG:25832",
    hash: "78179c43f885b7df906afef9613af46caa688bc66f4af97633f62ea4299a688e",
    feature_count: 1,
    model: { type: "FeatureCollection", features: [] },
  };

  it("asks for the CRS it was given", async () => {
    const fetchMock = stubFetch(jsonResponse(stored, 200));

    const response = await httpBackend.getModel("EPSG:4326");

    // Without the query the API answers in the project CRS — metres, for a
    // typical German project — and the map would draw the model in the
    // Atlantic. The parameter is required for exactly that reason.
    const [url] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/model?crs=EPSG%3A4326"));
    expect(response).toEqual(stored);
  });

  it("answers null for a project that holds no model yet", async () => {
    stubFetch(
      jsonResponse(
        envelope("model_not_found", "the project has no model yet"),
        404,
      ),
    );

    await expect(httpBackend.getModel("EPSG:4326")).resolves.toBeNull();
  });

  it("throws for a missing project, which is a different refusal", async () => {
    // `not_found` means the project itself is gone. Folding it into the same
    // `null` would show an empty map where the welcome page belongs.
    stubFetch(
      jsonResponse(envelope(ERROR_CODE_NOT_FOUND, "no project here"), 404),
    );

    const error = await httpBackend
      .getModel("EPSG:4326")
      .catch((e: unknown) => e);

    expect(asAPIRequestError(error)?.code).toBe(ERROR_CODE_NOT_FOUND);
  });

  it("throws for any other failure", async () => {
    stubFetch(new Response("boom", { status: 500 }));

    await expect(httpBackend.getModel("EPSG:4326")).rejects.toThrow(
      "Request failed: 500",
    );
  });
});

describe("httpBackend.createExport", () => {
  it("rejects: the API has no export endpoint", async () => {
    const fetchMock = stubFetch(jsonResponse({}, 200));

    await expect(httpBackend.createExport("run-0001")).rejects.toThrow(
      /not available in API mode/,
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
