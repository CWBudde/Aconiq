import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  hasDraft,
  loadDraft,
  discardDraft,
  useAutosave,
  writeDraft,
  DRAFT_KEY,
  DRAFT_VERSION,
} from "./use-autosave";
import { useModelStore } from "./model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "./types";

// Whether the draft write may mark the model clean depends on what the
// project is, so each autosave test states the mode it runs in instead of
// inheriting whatever the env selects.
const backendState = vi.hoisted(() => ({ runsAgainstSavedModel: false }));

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: backendState.runsAgainstSavedModel ? "http" : "browser",
        canExport: false,
        runsAgainstSavedModel: backendState.runsAgainstSavedModel,
        runsChangeExternally: backendState.runsAgainstSavedModel,
      };
    },
  },
}));

const sampleFeature: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const sampleReceiver: ModelReceiver = {
  id: "r1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [11, 52] },
};

const sampleCalcArea: CalcArea = {
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [11, 51],
        [11, 52],
        [10, 52],
        [10, 51],
      ],
    ],
  },
};

beforeEach(() => {
  localStorage.clear();
});

describe("draft utilities", () => {
  it("hasDraft returns false when nothing is stored", () => {
    expect(hasDraft()).toBe(false);
  });

  it("hasDraft returns true after saving a draft", () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({
        features: [sampleFeature],
        receivers: [sampleReceiver],
      }),
    );
    expect(hasDraft()).toBe(true);
  });

  it("loadDraft returns null when nothing is stored", () => {
    expect(loadDraft()).toBeNull();
  });

  it("loadDraft returns parsed features", () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({
        features: [sampleFeature],
        receivers: [sampleReceiver],
      }),
    );
    const result = loadDraft();
    expect(result?.features).toHaveLength(1);
    expect(result?.features[0]?.id).toBe("s1");
    expect(result?.receivers).toHaveLength(1);
    expect(result?.receivers[0]?.id).toBe("r1");
  });

  it("loadDraft supports legacy feature-only drafts", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    const result = loadDraft();
    expect(result?.features).toHaveLength(1);
    expect(result?.receivers).toEqual([]);
  });

  it("loadDraft returns null on corrupt data", () => {
    localStorage.setItem(DRAFT_KEY, "not-valid-json{{{");
    expect(loadDraft()).toBeNull();
  });

  describe("draft versions", () => {
    let warn: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    });

    afterEach(() => {
      warn.mockRestore();
    });

    it("writeDraft stamps the current version", () => {
      writeDraft({ features: [], receivers: [], calcArea: null });
      const stored = JSON.parse(localStorage.getItem(DRAFT_KEY) ?? "") as {
        version: number;
      };
      expect(stored.version).toBe(DRAFT_VERSION);
    });

    it("loadDraft accepts the current version", () => {
      localStorage.setItem(
        DRAFT_KEY,
        JSON.stringify({
          version: DRAFT_VERSION,
          features: [sampleFeature],
          receivers: [sampleReceiver],
          calcArea: sampleCalcArea,
        }),
      );
      expect(loadDraft()).toEqual({
        features: [sampleFeature],
        receivers: [sampleReceiver],
        calcArea: sampleCalcArea,
      });
      expect(warn).not.toHaveBeenCalled();
    });

    it("loadDraft still accepts an unversioned object", () => {
      localStorage.setItem(
        DRAFT_KEY,
        JSON.stringify({ features: [sampleFeature], receivers: [] }),
      );
      expect(loadDraft()?.features).toHaveLength(1);
      expect(warn).not.toHaveBeenCalled();
    });

    it.each([
      ["a newer version", { version: 2, features: [sampleFeature] }],
      ["a version of the wrong type", { version: "1", features: [] }],
      ["a non-object shape", "a string"],
    ])("loadDraft refuses %s and warns", (_name, draft) => {
      localStorage.setItem(DRAFT_KEY, JSON.stringify(draft));
      expect(loadDraft()).toBeNull();
      expect(warn).toHaveBeenCalledOnce();
    });

    it("hasDraft is false for a draft loadDraft refuses", () => {
      // The banner offers Restore on hasDraft; a key that is present but
      // unreadable would make that offer and then do nothing.
      localStorage.setItem(
        DRAFT_KEY,
        JSON.stringify({ version: 2, features: [sampleFeature] }),
      );
      expect(hasDraft()).toBe(false);
    });
  });

  it("discardDraft removes the entry", () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({
        features: [sampleFeature],
        receivers: [sampleReceiver],
      }),
    );
    discardDraft();
    expect(hasDraft()).toBe(false);
  });

  it("writeDraft stores what loadDraft reads back", () => {
    expect(
      writeDraft({
        features: [sampleFeature],
        receivers: [sampleReceiver],
        calcArea: sampleCalcArea,
      }),
    ).toBe(true);
    expect(loadDraft()).toEqual({
      features: [sampleFeature],
      receivers: [sampleReceiver],
      calcArea: sampleCalcArea,
    });
  });

  it("writeDraft reports a failed write instead of throwing", () => {
    const spy = vi
      .spyOn(Storage.prototype, "setItem")
      .mockImplementation(() => {
        throw new Error("storage full");
      });
    expect(writeDraft({ features: [], receivers: [], calcArea: null })).toBe(
      false,
    );
    spy.mockRestore();
  });

  it("hasDraft/loadDraft/discardDraft handle localStorage unavailability gracefully", () => {
    const spy = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(() => {
        throw new Error("storage unavailable");
      });
    expect(hasDraft()).toBe(false);
    expect(loadDraft()).toBeNull();
    spy.mockRestore();

    const setSpy = vi
      .spyOn(Storage.prototype, "removeItem")
      .mockImplementation(() => {
        throw new Error("storage unavailable");
      });
    expect(() => {
      discardDraft();
    }).not.toThrow();
    setSpy.mockRestore();
  });
});

describe("useAutosave", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    useModelStore.getState().reset();
    backendState.runsAgainstSavedModel = false;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function flushAutosave() {
    act(() => {
      vi.advanceTimersByTime(5000);
    });
  }

  it("persists the calculation area, not just features and receivers", () => {
    renderHook(() => {
      useAutosave();
    });

    act(() => {
      useModelStore.getState().addFeature(sampleFeature);
      useModelStore.getState().addReceiver(sampleReceiver);
      useModelStore.getState().setCalcArea(sampleCalcArea);
    });
    expect(useModelStore.getState().dirty).toBe(true);

    flushAutosave();

    // The regression: `setCalcArea` set `dirty`, so the timer fired and called
    // `markClean()`, but the payload omitted `calcArea` — the area was gone on
    // the next reload with nothing left marked unsaved.
    expect(useModelStore.getState().dirty).toBe(false);
    const draft = loadDraft();
    expect(draft?.calcArea).toEqual(sampleCalcArea);
    expect(draft?.features).toHaveLength(1);
    expect(draft?.receivers).toHaveLength(1);
  });

  it("saves a calculation area set on its own", () => {
    renderHook(() => {
      useAutosave();
    });

    act(() => {
      useModelStore.getState().setCalcArea(sampleCalcArea);
    });
    flushAutosave();

    expect(loadDraft()?.calcArea).toEqual(sampleCalcArea);
  });

  it("re-saves when only the calculation area changes", () => {
    renderHook(() => {
      useAutosave();
    });

    act(() => {
      useModelStore.getState().addFeature(sampleFeature);
    });
    flushAutosave();
    expect(loadDraft()?.calcArea).toBeNull();

    // `calcArea` was missing from the effect's dependency list, so a pending
    // timer kept the stale closure and wrote the previous model.
    act(() => {
      useModelStore.getState().setCalcArea(sampleCalcArea);
    });
    flushAutosave();
    expect(loadDraft()?.calcArea).toEqual(sampleCalcArea);
  });

  it("round-trips through the store via loadModel", () => {
    renderHook(() => {
      useAutosave();
    });
    act(() => {
      useModelStore.getState().addFeature(sampleFeature);
      useModelStore.getState().setCalcArea(sampleCalcArea);
    });
    flushAutosave();

    act(() => {
      useModelStore.getState().reset();
    });
    expect(useModelStore.getState().calcArea).toBeNull();

    const draft = loadDraft();
    expect(draft).not.toBeNull();
    act(() => {
      if (draft) useModelStore.getState().loadModel(draft);
    });

    expect(useModelStore.getState().calcArea).toEqual(sampleCalcArea);
    expect(useModelStore.getState().features).toHaveLength(1);
    // A bulk load is not an undoable edit.
    expect(useModelStore.getState().canUndo).toBe(false);
    // ...but it is content the project has not seen.
    expect(useModelStore.getState().dirty).toBe(true);
  });

  describe("in browser mode, where the draft is the project", () => {
    it("marks the model clean after writing the draft", () => {
      renderHook(() => {
        useAutosave();
      });
      act(() => {
        useModelStore.getState().addFeature(sampleFeature);
      });
      expect(useModelStore.getState().dirty).toBe(true);

      flushAutosave();

      expect(loadDraft()?.features).toHaveLength(1);
      expect(useModelStore.getState().dirty).toBe(false);
    });

    it("writes a restored draft back so it survives the next reload", () => {
      renderHook(() => {
        useAutosave();
      });
      act(() => {
        useModelStore.getState().loadModel({
          features: [sampleFeature],
          receivers: [],
          calcArea: null,
        });
      });
      flushAutosave();

      expect(loadDraft()?.features).toHaveLength(1);
      expect(useModelStore.getState().dirty).toBe(false);
    });
  });

  describe("in HTTP mode, where the project is the server's copy", () => {
    beforeEach(() => {
      backendState.runsAgainstSavedModel = true;
    });

    it("still writes the draft for crash recovery", () => {
      renderHook(() => {
        useAutosave();
      });
      act(() => {
        useModelStore.getState().addFeature(sampleFeature);
        useModelStore.getState().setCalcArea(sampleCalcArea);
      });
      flushAutosave();

      const draft = loadDraft();
      expect(draft?.features).toHaveLength(1);
      expect(draft?.calcArea).toEqual(sampleCalcArea);
    });

    it("leaves the model dirty after the draft write", () => {
      renderHook(() => {
        useAutosave();
      });
      act(() => {
        useModelStore.getState().addFeature(sampleFeature);
      });
      flushAutosave();

      // The regression: the draft write called `markClean()`, so the header
      // could report "saved" for a model the server had never received.
      expect(useModelStore.getState().dirty).toBe(true);
    });

    it("keeps the beforeunload guard while the project is behind", () => {
      renderHook(() => {
        useAutosave();
      });
      act(() => {
        useModelStore.getState().addFeature(sampleFeature);
      });
      flushAutosave();

      const event = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(event);
      expect(event.defaultPrevented).toBe(true);

      act(() => {
        useModelStore.getState().markClean();
      });
      const after = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(after);
      expect(after.defaultPrevented).toBe(false);
    });
  });
});
