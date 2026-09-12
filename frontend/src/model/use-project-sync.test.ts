import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { APIRequestError, ERROR_CODE_MODEL_INVALID } from "@/api/api-error";
import type { ModelSaveRequest } from "@/api/client";
import { useModelStore } from "./model-store";
import type { ModelFeature, ModelReceiver } from "./types";
import { useProjectSync } from "./use-project-sync";

const state = vi.hoisted(() => {
  const value: {
    runsAgainstSavedModel: boolean;
    projectLoaded: boolean;
    isPending: boolean;
    requests: unknown[];
    respond: () => Promise<{ featureCount: number; warnings: never[] }>;
  } = {
    runsAgainstSavedModel: true,
    projectLoaded: true,
    isPending: false,
    requests: [],
    respond: () => Promise.resolve({ featureCount: 0, warnings: [] }),
  };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.runsAgainstSavedModel ? "http" : "browser",
        canExport: false,
        runsAgainstSavedModel: state.runsAgainstSavedModel,
        runsChangeExternally: state.runsAgainstSavedModel,
      };
    },
  },
}));

vi.mock("@/api/hooks", () => ({
  useProjectStatus: () => ({
    data: state.projectLoaded
      ? { name: "Demo", crs: "EPSG:25832", scenario_count: 1, run_count: 0 }
      : null,
    isLoading: false,
  }),
  useSaveModel: () => ({
    mutateAsync: (req: ModelSaveRequest) => {
      state.requests.push(req);
      return state.respond();
    },
    isPending: state.isPending,
  }),
}));

const feature: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const receiver: ModelReceiver = {
  id: "r1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [11, 52] },
};

beforeEach(() => {
  state.runsAgainstSavedModel = true;
  state.projectLoaded = true;
  state.isPending = false;
  state.requests = [];
  state.respond = () => Promise.resolve({ featureCount: 0, warnings: [] });
  useModelStore.getState().reset();
});

describe("useProjectSync", () => {
  it("is enabled only where a run reads the saved model of a loaded project", () => {
    expect(renderHook(() => useProjectSync()).result.current.enabled).toBe(
      true,
    );

    state.projectLoaded = false;
    expect(renderHook(() => useProjectSync()).result.current.enabled).toBe(
      false,
    );

    state.projectLoaded = true;
    state.runsAgainstSavedModel = false;
    expect(renderHook(() => useProjectSync()).result.current.enabled).toBe(
      false,
    );
  });

  it("reports clean, then dirty after an edit", () => {
    const { result } = renderHook(() => useProjectSync());
    expect(result.current.status).toBe("clean");
    expect(result.current.dirty).toBe(false);

    act(() => {
      useModelStore.getState().addFeature(feature);
    });
    expect(result.current.status).toBe("dirty");
    expect(result.current.dirty).toBe(true);
  });

  it("reports saving while the request is in flight", () => {
    state.isPending = true;
    useModelStore.getState().addFeature(feature);
    const { result } = renderHook(() => useProjectSync());
    expect(result.current.status).toBe("saving");
  });

  it("sends the whole model in WGS84 and marks it clean on success", async () => {
    useModelStore.getState().addFeature(feature);
    useModelStore.getState().addReceiver(receiver);
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    expect(state.requests).toHaveLength(1);
    const req = state.requests[0] as ModelSaveRequest;
    expect(req.crs).toBe("EPSG:4326");
    expect(req.model.features.map((f) => f.id)).toEqual(["s1", "r1"]);
    expect(useModelStore.getState().dirty).toBe(false);
    expect(result.current.status).toBe("clean");
    expect(result.current.error).toBeNull();
  });

  it("keeps the model dirty when an edit lands while the save is in flight", async () => {
    useModelStore.getState().addFeature(feature);
    let resolve: (() => void) | undefined;
    state.respond = () =>
      new Promise((r) => {
        resolve = () => {
          r({ featureCount: 0, warnings: [] });
        };
      });
    const { result } = renderHook(() => useProjectSync());

    let saved: Promise<void> | undefined;
    act(() => {
      saved = result.current.save();
    });
    act(() => {
      useModelStore.getState().addReceiver(receiver);
    });
    await act(async () => {
      resolve?.();
      await saved;
    });

    // The receiver is not in what was sent, so declaring the workspace
    // synced would be a lie the next run acted on.
    expect(useModelStore.getState().dirty).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it("keeps the failure and the dirty flag when the save is refused", async () => {
    useModelStore.getState().addFeature(feature);
    const refused = new APIRequestError({
      code: ERROR_CODE_MODEL_INVALID,
      message: "model validation failed",
      details: { errors: [{ code: "x", message: "y", feature_id: "s1" }] },
    });
    state.respond = () => Promise.reject(refused);
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    expect(useModelStore.getState().dirty).toBe(true);
    expect(result.current.status).toBe("error");
    expect(result.current.error).toBe(refused);
  });

  it("wraps a non-Error rejection so the header has something to show", async () => {
    useModelStore.getState().addFeature(feature);
    state.respond = () => Promise.reject("boom" as unknown as Error);
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("boom");
  });

  it("clears the failure once a later save succeeds", async () => {
    useModelStore.getState().addFeature(feature);
    state.respond = () => Promise.reject(new Error("Request failed: 500"));
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });
    expect(result.current.status).toBe("error");

    state.respond = () => Promise.resolve({ featureCount: 1, warnings: [] });
    await act(async () => {
      await result.current.save();
    });
    await waitFor(() => {
      expect(result.current.status).toBe("clean");
    });
    expect(result.current.error).toBeNull();
  });

  it("sends one request for overlapping save calls", async () => {
    useModelStore.getState().addFeature(feature);
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await Promise.all([result.current.save(), result.current.save()]);
    });

    expect(state.requests).toHaveLength(1);
  });
});
