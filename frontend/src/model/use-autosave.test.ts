import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  hasDraft,
  loadDraft,
  discardDraft,
  useAutosave,
  DRAFT_KEY,
} from "./use-autosave";
import { useModelStore } from "./model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "./types";

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
    expect(useModelStore.getState().dirty).toBe(false);
  });
});
