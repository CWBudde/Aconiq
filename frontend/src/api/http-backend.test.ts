import { afterEach, describe, expect, it, vi } from "vitest";
import {
  APIRequestError,
  ERROR_CODE_NOT_FOUND,
  asAPIRequestError,
} from "./api-error";
import type { ModelSaveRequest, ModelSaveResponse } from "./client";
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
    });
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
    // The receipt the server returns for what it just wrote. `saveModel` does
    // not surface it yet; the draft stores it once hydration lands, so that a
    // restored draft can prove it equals the project by comparing strings.
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

describe("httpBackend.createExport", () => {
  it("rejects: the API has no export endpoint", async () => {
    const fetchMock = stubFetch(jsonResponse({}, 200));

    await expect(httpBackend.createExport("run-0001")).rejects.toThrow(
      /not available in API mode/,
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
