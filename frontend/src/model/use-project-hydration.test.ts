import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ModelResponse, ProjectStatusResponse } from "@/api/client";
import { useModelStore } from "./model-store";
import { CALC_AREA_FEATURE_ID } from "./to-geojson";
import type { CalcArea, ModelFeature } from "./types";
import { DRAFT_KEY, writeDraft } from "./use-autosave";
import {
  projectHydrationStore,
  resetProjectHydration,
  retryProjectHydration,
  useProjectHydration,
} from "./use-project-hydration";

/**
 * The whole point of this hook is that it never destroys work. Every case
 * below is a way it could: overwriting edits, restoring a draft that only
 * looks like the project, fetching twice under StrictMode, or leaving an empty
 * map over a populated project without saying so.
 */

const PROJECT_HASH = "78179c43f885b7df906afef9613af46caa688bc66f4af97633f6";

const state = vi.hoisted(() => {
  const value: {
    runsAgainstSavedModel: boolean;
    project: unknown;
    isLoading: boolean;
    isError: boolean;
    error: unknown;
    refetch: () => Promise<unknown>;
    crsRequests: string[];
    respond: () => Promise<unknown>;
  } = {
    runsAgainstSavedModel: true,
    project: null,
    isLoading: false,
    isError: false,
    error: null,
    refetch: () => Promise.resolve(null),
    crsRequests: [],
    respond: () => Promise.resolve(null),
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
    getModel: (crs: string) => {
      state.crsRequests.push(crs);
      return state.respond();
    },
  },
}));

vi.mock("@/api/hooks", () => ({
  useProjectStatus: () => ({
    data: state.project,
    isLoading: state.isLoading,
    isError: state.isError,
    error: state.error,
    refetch: () => state.refetch(),
  }),
}));

function projectWithModel(hash: string | null): ProjectStatusResponse {
  return {
    project_id: "p1",
    name: "Demo",
    project_path: "/tmp/demo",
    manifest_version: 1,
    crs: "EPSG:25832",
    scenario_count: 1,
    run_count: 0,
    ...(hash === null
      ? {}
      : { model: { hash, updated_at: "2026-01-01T00:00:00Z" } }),
  };
}

const draftFeature: ModelFeature = {
  id: "s2",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [12, 53] },
};

const area: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [10.1, 51],
        [10.1, 51.1],
        [10, 51],
      ],
    ],
  },
};

/** A project model as the API serves it: ids in `properties.id`, no `id`. */
function storedModel(): ModelResponse {
  return {
    crs: "EPSG:4326",
    project_crs: "EPSG:25832",
    hash: PROJECT_HASH,
    feature_count: 3,
    model: {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { id: "s1", kind: "source", source_type: "point" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
        {
          type: "Feature",
          properties: { id: "r1", kind: "receiver", height_m: 4 },
          geometry: { type: "Point", coordinates: [10.5, 51.5] },
        },
        {
          type: "Feature",
          properties: { id: CALC_AREA_FEATURE_ID, kind: "calc-area" },
          geometry: area.geometry,
        },
      ],
    },
  };
}

beforeEach(() => {
  state.runsAgainstSavedModel = true;
  state.project = null;
  state.isLoading = false;
  state.isError = false;
  state.error = null;
  state.refetch = () => Promise.resolve(null);
  state.crsRequests = [];
  state.respond = () => Promise.resolve(storedModel());
  localStorage.clear();
  useModelStore.getState().reset();
  resetProjectHydration();
});

afterEach(() => {
  vi.restoreAllMocks();
});

async function settle() {
  await waitFor(() => {
    expect(projectHydrationStore.getState().status).not.toBe("fetching");
  });
}

describe("useProjectHydration", () => {
  it("never fetches in browser mode, where the store is the project", async () => {
    state.runsAgainstSavedModel = false;
    state.project = projectWithModel(PROJECT_HASH);

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual([]);
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("does not fetch while the project status is still loading", () => {
    state.isLoading = true;

    renderHook(() => {
      useProjectHydration();
    });

    // Deciding on `data === undefined` would be indistinguishable from
    // deciding there is no project.
    expect(state.crsRequests).toEqual([]);
    expect(projectHydrationStore.getState().started).toBe(false);
  });

  it("does not fetch when no project is loaded", async () => {
    state.project = null;

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual([]);
    expect(projectHydrationStore.getState().status).toBe("settled");
  });

  it("does not settle a hydration the project status never answered", async () => {
    // An errored status is no answer, not an answer of "no model". Collapsing
    // the two would leave an empty map over a populated project for the rest
    // of the session, and the next save would write that emptiness back.
    state.isError = true;
    state.error = new Error("Request failed: 503");

    renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(projectHydrationStore.getState().status).toBe("error");
    });

    expect(state.crsRequests).toEqual([]);
    expect(projectHydrationStore.getState().error).toBe(state.error);
    // Still armed, so a later answer is still acted on.
    expect(projectHydrationStore.getState().started).toBe(false);
  });

  it("hydrates when the project status succeeds on a later attempt", async () => {
    state.isError = true;
    state.error = new Error("Request failed: 503");

    const { rerender } = renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(projectHydrationStore.getState().status).toBe("error");
    });

    // A reconnect, a window focus or the Retry button: React Query answers,
    // and the hook is still there to act on it.
    state.isError = false;
    state.error = null;
    state.project = projectWithModel(PROJECT_HASH);
    rerender();
    await waitFor(() => {
      expect(state.crsRequests).toEqual(["EPSG:4326"]);
    });
    await settle();

    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s1"]);
  });

  it("asks the project status again when that is the request that failed", async () => {
    // Re-arming the hook alone changes nothing while the query sits in its own
    // error state: it would serve the same rejection and the same decision.
    state.isError = true;
    state.error = new Error("Request failed: 503");
    const refetch = vi.fn(() => Promise.resolve(null));
    state.refetch = refetch;

    renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(projectHydrationStore.getState().status).toBe("error");
    });
    expect(refetch).not.toHaveBeenCalled();

    retryProjectHydration();
    await waitFor(() => {
      expect(refetch).toHaveBeenCalledTimes(1);
    });
  });

  it("does not fetch for a project that holds no model", async () => {
    state.project = projectWithModel(null);

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual([]);
  });

  it("fetches in WGS84 and hydrates the store clean", async () => {
    state.project = projectWithModel(PROJECT_HASH);

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    // WGS84 and not the project CRS: the map draws in degrees, and metres
    // would land the model off the coast of Africa.
    expect(state.crsRequests).toEqual(["EPSG:4326"]);
    const model = useModelStore.getState();
    expect(model.features.map((f) => f.id)).toEqual(["s1"]);
    expect(model.receivers.map((r) => r.id)).toEqual(["r1"]);
    expect(model.calcArea?.geometry.coordinates).toEqual(
      area.geometry.coordinates,
    );
    // Clean, not dirty: content that arrives from the project is the project.
    // Dirty here would arm the unload guard on every reload and make the
    // autosave write a hash-less draft of the project's own model.
    expect(model.dirty).toBe(false);
  });

  it("restores a draft whose hash matches, without fetching", async () => {
    // Not only an optimisation: the draft holds the coordinates the user drew,
    // not ones round-tripped through 4326 → 25832 → 4326.
    state.project = projectWithModel(PROJECT_HASH);
    writeDraft({
      features: [draftFeature],
      receivers: [],
      calcArea: null,
      hash: PROJECT_HASH,
    });
    resetProjectHydration();

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual([]);
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s2"]);
    expect(useModelStore.getState().dirty).toBe(false);
    // Nothing left to offer: the draft is the project.
    expect(projectHydrationStore.getState().draftOffered).toBe(false);
  });

  it("fetches and still offers a draft whose hash differs", async () => {
    state.project = projectWithModel(PROJECT_HASH);
    writeDraft({
      features: [draftFeature],
      receivers: [],
      calcArea: null,
      hash: "a-hash-from-an-earlier-save",
    });
    resetProjectHydration();

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual(["EPSG:4326"]);
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s1"]);
    // The divergent draft is exactly the work the user would otherwise lose.
    expect(projectHydrationStore.getState().draftOffered).toBe(true);
  });

  it("fetches for a legacy draft that carries no hash at all", async () => {
    // Every draft the autosave writes is hash-less, and so is every draft
    // written before the field existed. None of them may be trusted to be the
    // project.
    state.project = projectWithModel(PROJECT_HASH);
    localStorage.setItem(DRAFT_KEY, JSON.stringify([draftFeature]));
    resetProjectHydration();

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual(["EPSG:4326"]);
    expect(projectHydrationStore.getState().draftOffered).toBe(true);
  });

  it("never hydrates over unsaved work already in the store", async () => {
    state.project = projectWithModel(PROJECT_HASH);
    useModelStore.getState().addFeature(draftFeature);
    resetProjectHydration();

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual([]);
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s2"]);
    expect(useModelStore.getState().dirty).toBe(true);
  });

  it("abandons the hydration when the store changes mid-flight", async () => {
    // The first check is stale by a network round trip: the user can draw
    // while the request is in the air, and that drawing must win.
    state.project = projectWithModel(PROJECT_HASH);
    let release: (() => void) | undefined;
    state.respond = () =>
      new Promise((resolve) => {
        release = () => {
          resolve(storedModel());
        };
      });

    renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(state.crsRequests).toHaveLength(1);
    });
    useModelStore.getState().addFeature(draftFeature);
    release?.();
    await settle();

    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s2"]);
    expect(useModelStore.getState().dirty).toBe(true);
  });

  it("fetches exactly once when the hook runs twice", async () => {
    // StrictMode double-mounts, and a remount must not re-run the decision.
    // The guard is set before the first await and lives in the module store
    // rather than a ref for exactly this.
    state.project = projectWithModel(PROJECT_HASH);

    const first = renderHook(() => {
      useProjectHydration();
    });
    renderHook(() => {
      useProjectHydration();
    });
    first.unmount();
    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(state.crsRequests).toEqual(["EPSG:4326"]);
  });

  it("surfaces a rejection instead of leaving an empty map", async () => {
    // Silence here reproduces the failure this hook exists to fix: an empty
    // map over a populated project looks exactly like an empty project.
    state.project = projectWithModel(PROJECT_HASH);
    const refused = new Error("Request failed: 500");
    state.respond = () => Promise.reject(refused);

    renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(projectHydrationStore.getState().status).toBe("error");
    });

    expect(projectHydrationStore.getState().error).toBe(refused);
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("settles without hydrating when the model is gone by the time it is read", async () => {
    state.project = projectWithModel(PROJECT_HASH);
    state.respond = () => Promise.resolve(null);

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(projectHydrationStore.getState().status).toBe("settled");
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("retries a refused hydration without a reload", async () => {
    state.project = projectWithModel(PROJECT_HASH);
    state.respond = () => Promise.reject(new Error("Request failed: 500"));

    renderHook(() => {
      useProjectHydration();
    });
    await waitFor(() => {
      expect(projectHydrationStore.getState().status).toBe("error");
    });

    state.respond = () => Promise.resolve(storedModel());
    retryProjectHydration();
    await settle();

    expect(state.crsRequests).toHaveLength(2);
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual(["s1"]);
  });

  it("keeps the ids the project stores rather than minting new ones", async () => {
    // The model arrives with its ids in `properties.id` and no GeoJSON `id`
    // member, which is what `Model.ToFeatureCollection` writes. Reading only
    // the member would give every feature a fresh UUID, and the first save
    // afterwards would rewrite every id in the project.
    state.project = projectWithModel(PROJECT_HASH);

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(useModelStore.getState().features[0]?.id).toBe("s1");
    expect(useModelStore.getState().receivers[0]?.id).toBe("r1");
  });

  it("writes no draft of its own", async () => {
    // Writing one would overwrite the divergent draft that must still be
    // offered for Restore. Leaving the store clean is what keeps the autosave
    // idle, so the draft survives.
    state.project = projectWithModel(PROJECT_HASH);
    writeDraft({
      features: [draftFeature],
      receivers: [],
      calcArea: null,
    });
    const before = localStorage.getItem(DRAFT_KEY);
    resetProjectHydration();

    renderHook(() => {
      useProjectHydration();
    });
    await settle();

    expect(localStorage.getItem(DRAFT_KEY)).toBe(before);
  });

  it("does not re-run once the project status is invalidated by a save", async () => {
    // `useSaveModel.onSuccess` invalidates the project query, so
    // `status.model.hash` changes after every save while the app runs. A
    // second hydration would overwrite whatever was on screen.
    state.project = projectWithModel(PROJECT_HASH);
    const { rerender } = renderHook(() => {
      useProjectHydration();
    });
    await settle();

    state.project = projectWithModel("a-hash-from-a-later-save");
    rerender();
    await settle();

    expect(state.crsRequests).toEqual(["EPSG:4326"]);
  });
});
