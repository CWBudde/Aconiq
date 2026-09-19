import { createElement, type ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ERROR_CODE_MODEL_NOT_FOUND, ERROR_CODE_NOT_FOUND } from "./api-error";
import { CLIENT_HEADER_NAME } from "./client";
import type { RasterMetadata, ReceiverTable, RunSummary } from "./client";
import {
  getArtifactContentURL,
  useArtifactBytes,
  useArtifactContent,
  useCreateExport,
  useCreateRun,
  useHealth,
  useImportFromOSM,
  useProjectStatus,
  useRasterMetadata,
  useReceiverTable,
  useRunContours,
  useStandards,
} from "./hooks";
import { apiURL } from "./mode";
import { queryClient } from "./query-client";
import { queryKeys } from "./query-keys";

/**
 * The hooks the sibling `hooks.test.ts` cannot reach, driven end to end
 * against a stubbed `fetch`.
 *
 * Two files, because `vi.mock` is hoisted over the whole module. `hooks.test.ts`
 * mocks `./backend`, which is what its polling and browser-mode cases need — a
 * `fetch` stub cannot say `capabilities.kind === "browser"`, and a run that
 * completes inside `startRun` has no request to intercept. Everything here
 * wants the opposite: the real `httpBackend` underneath, so the assertions
 * cover the URL that was built, the headers it carried and the envelope that
 * came back, not a hand-written stand-in for them.
 *
 * `backend` resolves to `httpBackend` here because `IS_WASM_MODE` reads
 * `import.meta.env.VITE_WASM_MODE`, which vitest does not set (`mode.ts`).
 */

type FetchMock = ReturnType<typeof vi.fn<typeof fetch>>;

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function envelope(code: string, message: string): unknown {
  return { error: { code, message } };
}

function stubFetch(...responses: Response[]): FetchMock {
  const mock = vi.fn<typeof fetch>();
  for (const response of responses) {
    mock.mockResolvedValueOnce(response);
  }
  vi.stubGlobal("fetch", mock);
  return mock;
}

/** The request `fetch` was called with, as `[url, init]`. */
function requestOf(mock: FetchMock, index = 0): [string, RequestInit] {
  const call = mock.mock.calls[index];
  if (call === undefined)
    throw new Error(`fetch call ${String(index)} missing`);
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

/** The JSON body a write carried. Everything here posts a string. */
function bodyOf(init: RequestInit): unknown {
  const { body } = init;
  if (typeof body !== "string") {
    throw new Error("expected a JSON string body");
  }
  return JSON.parse(body);
}

function wrapper({ children }: { children: ReactNode }) {
  return createElement(QueryClientProvider, { client: queryClient }, children);
}

beforeEach(() => {
  // Every query here is cached by key, and several tests read the same key
  // with different fixtures. Without this the second one is served the first
  // one's value and never calls `fetch` at all.
  queryClient.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("useHealth", () => {
  it("reads /api/v1/health with the client header that forces the preflight", async () => {
    const fetchMock = stubFetch(
      jsonResponse({
        status: "ok",
        version: "0.1.0",
        time: "2026-01-01T00:00:00Z",
      }),
    );

    const { result } = renderHook(() => useHealth(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.version).toBe("0.1.0");

    const [url, init] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/health"));
    expect(headersOf(init)[CLIENT_HEADER_NAME]).toBe("aconiq-web");
  });
});

describe("useProjectStatus", () => {
  it("resolves to the status document", async () => {
    stubFetch(
      jsonResponse({
        project_id: "p1",
        name: "Testprojekt",
        project_path: "/tmp/p1",
        manifest_version: 1,
        crs: "EPSG:25832",
        scenario_count: 1,
        run_count: 2,
      }),
    );

    const { result } = renderHook(() => useProjectStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.run_count).toBe(2);
  });

  /**
   * "No project here yet" is the welcome page's state, not a failure, and the
   * backend distinguishes it by the envelope's code rather than by the status.
   * The hook has to surface that as data — a query that rejected would render
   * an error page over a perfectly healthy empty install.
   */
  it("resolves to null when no project is initialised", async () => {
    stubFetch(jsonResponse(envelope(ERROR_CODE_NOT_FOUND, "no project"), 404));

    const { result } = renderHook(() => useProjectStatus(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toBeNull();
  });

  /** A 404 that is not that envelope is a real failure — a wrong base URL, say. */
  it("fails on a 404 that carries a different code", async () => {
    stubFetch(
      jsonResponse(envelope(ERROR_CODE_MODEL_NOT_FOUND, "no model"), 404),
      jsonResponse(envelope(ERROR_CODE_MODEL_NOT_FOUND, "no model"), 404),
    );

    const { result } = renderHook(() => useProjectStatus(), { wrapper });

    await waitFor(
      () => {
        expect(result.current.isError).toBe(true);
      },
      { timeout: 5000 },
    );
  });
});

describe("useStandards", () => {
  it("reads the descriptor list", async () => {
    const fetchMock = stubFetch(
      jsonResponse([{ id: "rls19-road", evidence_tier: "normative" }]),
    );

    const { result } = renderHook(() => useStandards(), { wrapper });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toHaveLength(1);
    expect(requestOf(fetchMock)[0]).toBe(apiURL("/api/v1/standards"));
  });
});

describe("useArtifactContent", () => {
  it("escapes the artifact id in the path", async () => {
    const fetchMock = stubFetch(jsonResponse({ hello: "world" }));

    const { result } = renderHook(
      () => useArtifactContent<{ hello: string }>("art/1"),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(requestOf(fetchMock)[0]).toBe(
      apiURL("/api/v1/artifacts/art%2F1/content"),
    );
  });

  /**
   * `null` is how a caller says "there is no artifact to read yet" — a results
   * page rendering before its run resolved. The query must stay idle rather
   * than fetch an empty id, which the API would answer with a 404 the page
   * would then have to explain away.
   */
  it("does not fetch while the id is null", () => {
    const fetchMock = stubFetch(jsonResponse({}));

    const { result } = renderHook(() => useArtifactContent(null), { wrapper });

    expect(result.current.fetchStatus).toBe("idle");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("useArtifactBytes", () => {
  it("asks for octet-stream and hands back the raw buffer", async () => {
    const bytes = new Float64Array([1.5, 2.5]);
    const fetchMock = stubFetch(new Response(bytes.buffer));

    const { result } = renderHook(() => useArtifactBytes("raster-1"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(new Float64Array(result.current.data as ArrayBuffer)).toEqual(bytes);

    const [url, init] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/artifacts/raster-1/content"));
    // The default Accept is application/json, which is the one thing a
    // headerless float64 raster is not.
    expect(headersOf(init).Accept).toBe("application/octet-stream");
  });

  it("does not fetch while the id is null", () => {
    const fetchMock = stubFetch(new Response(new ArrayBuffer(0)));

    const { result } = renderHook(() => useArtifactBytes(null), { wrapper });

    expect(result.current.fetchStatus).toBe("idle");
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("useRunContours", () => {
  it("sends the CRS and the interval as query parameters", async () => {
    const fetchMock = stubFetch(jsonResponse({ crs: "EPSG:4326", lines: [] }));

    const { result } = renderHook(
      () => useRunContours("run-1", { crs: "EPSG:4326", interval: 5 }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(requestOf(fetchMock)[0]).toBe(
      apiURL("/api/v1/runs/run-1/contours?crs=EPSG%3A4326&interval=5"),
    );
  });

  /**
   * An omitted interval means "whatever the server's default is". Sending an
   * empty one would read the same to the route and mean something else here:
   * that the client had an opinion it does not have.
   */
  it("omits the interval entirely when none was asked for", async () => {
    const fetchMock = stubFetch(jsonResponse({ crs: "EPSG:4326", lines: [] }));

    const { result } = renderHook(
      () => useRunContours("run-1", { crs: "EPSG:4326" }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const [url] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/runs/run-1/contours?crs=EPSG%3A4326"));
    expect(url).not.toContain("interval");
  });

  it("stays idle without a run or without options", () => {
    const fetchMock = stubFetch(jsonResponse({}));

    const { result } = renderHook(() => useRunContours(null, null), {
      wrapper,
    });

    expect(result.current.fetchStatus).toBe("idle");
    expect(fetchMock).not.toHaveBeenCalled();
  });

  /**
   * A run whose receivers were not a grid is refused in the same words every
   * time. `retry: false` is what keeps that from costing three round trips to
   * be told so three times — and the default on this client is one retry, so
   * it is a real setting rather than an inherited one.
   */
  it("does not retry a refusal", async () => {
    const fetchMock = stubFetch(
      jsonResponse(envelope("contours_unavailable", "not a grid"), 409),
      jsonResponse(envelope("contours_unavailable", "not a grid"), 409),
    );

    const { result } = renderHook(
      () => useRunContours("run-1", { crs: "EPSG:4326" }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

/**
 * Both of these read an artifact that may predate the per-channel unit. The
 * expansion happens in the hook so a new consumer cannot forget it, which
 * means the hook is the only place it can be tested.
 */
describe("useReceiverTable", () => {
  it("expands a legacy scalar unit across every indicator", async () => {
    stubFetch(
      jsonResponse({
        indicator_order: ["lr_day", "lr_night"],
        unit: "dB(A)",
        records: [],
      }),
    );

    const { result } = renderHook(() => useReceiverTable("table-1"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.units).toEqual({
      lr_day: "dB(A)",
      lr_night: "dB(A)",
    });
  });

  it("leaves a current container untouched", async () => {
    const table: ReceiverTable = {
      indicator_order: ["lr_day"],
      units: { lr_day: "dB(A)" },
      records: [],
    };
    stubFetch(jsonResponse(table));

    const { result } = renderHook(() => useReceiverTable("table-2"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(table);
  });

  /**
   * The memo is the point of the wrapper: `withLegacyUnits` builds a new
   * object for a legacy container, and react-query hands back a stable
   * reference, so without it every render would produce a fresh table and
   * re-run every memo and effect keyed on it — the raster's colourising among
   * them.
   */
  it("keeps the same object across renders", async () => {
    stubFetch(
      jsonResponse({
        indicator_order: ["lr_day"],
        unit: "dB(A)",
        records: [],
      }),
    );

    const { result, rerender } = renderHook(() => useReceiverTable("table-3"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    const first = result.current.data;
    rerender();
    expect(result.current.data).toBe(first);
  });

  it("stays idle while the id is null", () => {
    const fetchMock = stubFetch(jsonResponse({}));

    const { result } = renderHook(() => useReceiverTable(null), { wrapper });

    expect(result.current.data).toBeUndefined();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("useRasterMetadata", () => {
  it("expands a legacy scalar unit across the band names", async () => {
    stubFetch(
      jsonResponse({
        width: 2,
        height: 2,
        bands: 2,
        nodata: -9999,
        unit: "dB(A)",
        band_names: ["LrDay", "LrNight"],
      }),
    );

    const { result } = renderHook(() => useRasterMetadata("raster-meta"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.units).toEqual({
      LrDay: "dB(A)",
      LrNight: "dB(A)",
    });
  });

  /**
   * Without band names there is nothing to key the scalar by, and inventing
   * names would be worse than leaving the field absent.
   */
  it("leaves a legacy container alone when it names no bands", async () => {
    const meta = {
      width: 1,
      height: 1,
      bands: 1,
      nodata: -9999,
      unit: "dB(A)",
    } as unknown as RasterMetadata;
    stubFetch(jsonResponse(meta));

    const { result } = renderHook(() => useRasterMetadata("raster-meta-2"), {
      wrapper,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data?.units).toBeUndefined();
  });
});

describe("useImportFromOSM", () => {
  it("posts the bounding box and returns the feature collection", async () => {
    const fetchMock = stubFetch(
      jsonResponse({ type: "FeatureCollection", features: [] }),
    );

    const { result } = renderHook(() => useImportFromOSM(), { wrapper });

    const collection = await result.current.mutateAsync({
      south: 52.5,
      west: 13.3,
      north: 52.6,
      east: 13.4,
    });

    expect(collection.type).toBe("FeatureCollection");
    const [url, init] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/import/osm"));
    expect(init.method).toBe("POST");
    expect(bodyOf(init)).toEqual({
      south: 52.5,
      west: 13.3,
      north: 52.6,
      east: 13.4,
    });
  });
});

describe("useCreateRun", () => {
  const spec = {
    standardId: "rls19-road",
    version: "2019",
    profile: "default",
    params: { grid_spacing_m: "10" },
    receiverMode: "auto-grid" as const,
  };

  it("posts the run request and invalidates the runs and project caches", async () => {
    const run: RunSummary = {
      id: "run-1",
      scenario_id: "default",
      standard_id: "rls19-road",
      version: "2019",
      status: "completed",
      started_at: "2026-01-01T00:00:00Z",
      finished_at: "2026-01-01T00:00:01Z",
      log_path: "",
      artifacts: [],
    };
    const fetchMock = stubFetch(jsonResponse(run, 201));
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateRun(), { wrapper });
    const created = await result.current.mutateAsync(spec);

    expect(created.id).toBe("run-1");
    const [url, init] = requestOf(fetchMock);
    expect(url).toBe(apiURL("/api/v1/runs"));
    expect(init.method).toBe("POST");
    expect(bodyOf(init)).toEqual({
      standard_id: "rls19-road",
      standard_version: "2019",
      standard_profile: "default",
      receiver_mode: "auto-grid",
      params: { grid_spacing_m: "10" },
    });

    const invalidated = invalidate.mock.calls.map(([args]) => args?.queryKey);
    expect(invalidated).toContainEqual(queryKeys.runs.all);
    expect(invalidated).toContainEqual(queryKeys.project.all);
  });

  /**
   * The run dialog explains a refusal from the envelope's `code` and `hint`,
   * so the mutation must throw the envelope whole rather than a bare message.
   */
  it("rejects with the envelope's code", async () => {
    stubFetch(
      jsonResponse(
        envelope("experimental_opt_in_required", "scaffold tier"),
        400,
      ),
    );

    const { result } = renderHook(() => useCreateRun(), { wrapper });

    await expect(result.current.mutateAsync(spec)).rejects.toMatchObject({
      code: "experimental_opt_in_required",
    });
  });
});

describe("useCreateExport", () => {
  /**
   * The page never offers the button — `canExport` is false in API mode — but
   * the method exists so the `Backend` interface has no mode-specific hole,
   * and it must refuse rather than silently do nothing.
   */
  it("refuses in API mode without reaching the network", async () => {
    const fetchMock = stubFetch(jsonResponse({}));

    const { result } = renderHook(() => useCreateExport(), { wrapper });

    await expect(result.current.mutateAsync("run-1")).rejects.toThrow(
      /aconiq export/,
    );
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("getArtifactContentURL", () => {
  /**
   * A URL, not a fetch: the export page hands it to a download link, so it has
   * to carry the base URL and escape the id the same way every other call
   * does.
   */
  it("builds an absolute content URL with the id escaped", () => {
    expect(getArtifactContentURL("art/1")).toBe(
      apiURL("/api/v1/artifacts/art%2F1/content"),
    );
  });
});
