import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { APIRequestError, ERROR_CODE_MODEL_INVALID } from "@/api/api-error";
import type { ModelSaveRequest } from "@/api/client";
import { useModelStore } from "./model-store";
import type { ModelFeature, ModelReceiver } from "./types";
import { loadDraft, writeDraft } from "./use-autosave";
import { resetProjectSyncStore, useProjectSync } from "./use-project-sync";

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
  }),
  useIsSavingModel: () => state.isPending,
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
  resetProjectSyncStore();
  localStorage.clear();
  // Failures that are not API refusals are logged; keep the run quiet and
  // let the tests that care assert on it.
  vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
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
    // What the autosave wrote once the request outlasted its debounce: the
    // newer state, including the in-flight edit.
    const newerDraft = {
      features: [feature],
      receivers: [receiver],
      calcArea: null,
    };
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
    writeDraft(newerDraft);
    await act(async () => {
      resolve?.();
      await saved;
    });

    // The receiver is not in what was sent, so declaring the workspace
    // synced would be a lie the next run acted on.
    expect(useModelStore.getState().dirty).toBe(true);
    expect(result.current.error).toBeNull();
    // ...and the save must not roll the draft back to its stale snapshot:
    // nothing would re-trigger the autosave, and a reload would lose the
    // receiver.
    expect(loadDraft()).toEqual(newerDraft);
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
    // The server's answer is shown in full by the header; nothing to log.
    expect(console.error).not.toHaveBeenCalled();
  });

  it("logs a failure that is not an API refusal", async () => {
    useModelStore.getState().addFeature(feature);
    const failure = new TypeError("Failed to fetch");
    state.respond = () => Promise.reject(failure);
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    // The header renders this as "saving failed"; a programming error
    // behind it must not vanish into that.
    expect(console.error).toHaveBeenCalledWith("model save", failure);
    expect(result.current.error).toBe(failure);
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

  it("rewrites the draft with the saved snapshot so draft and project agree", async () => {
    useModelStore.getState().addFeature(feature);
    // A draft from before the save, as the autosave would have left it.
    writeDraft({ features: [], receivers: [], calcArea: null });
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    // Without this, "Restore draft" after a reload offered the older model.
    expect(loadDraft()?.features.map((f) => f.id)).toEqual(["s1"]);
  });

  it("leaves the draft alone when the save is refused", async () => {
    useModelStore.getState().addFeature(feature);
    writeDraft({ features: [], receivers: [], calcArea: null });
    state.respond = () => Promise.reject(new Error("Request failed: 500"));
    const { result } = renderHook(() => useProjectSync());

    await act(async () => {
      await result.current.save();
    });

    expect(loadDraft()?.features).toEqual([]);
  });

  describe("shared across surfaces", () => {
    it("shows one instance's failure on every instance", async () => {
      useModelStore.getState().addFeature(feature);
      const refused = new Error("Request failed: 500");
      state.respond = () => Promise.reject(refused);
      const header = renderHook(() => useProjectSync());
      const dialog = renderHook(() => useProjectSync());

      await act(async () => {
        await header.result.current.save();
      });

      expect(header.result.current.status).toBe("error");
      expect(dialog.result.current.status).toBe("error");
      expect(dialog.result.current.error).toBe(refused);

      state.respond = () => Promise.resolve({ featureCount: 1, warnings: [] });
      await act(async () => {
        await dialog.result.current.save();
      });
      expect(header.result.current.status).toBe("clean");
      expect(header.result.current.error).toBeNull();
    });

    it("holds the in-flight guard across instances", async () => {
      useModelStore.getState().addFeature(feature);
      let resolve: (() => void) | undefined;
      state.respond = () =>
        new Promise((r) => {
          resolve = () => {
            r({ featureCount: 0, warnings: [] });
          };
        });
      const header = renderHook(() => useProjectSync());
      const dialog = renderHook(() => useProjectSync());

      let first: Promise<void> | undefined;
      act(() => {
        first = header.result.current.save();
      });
      // The dialog's inline button, pressed while the header's save runs.
      await act(async () => {
        await dialog.result.current.save();
      });
      expect(state.requests).toHaveLength(1);

      await act(async () => {
        resolve?.();
        await first;
      });
      expect(useModelStore.getState().dirty).toBe(false);
    });
  });
});
