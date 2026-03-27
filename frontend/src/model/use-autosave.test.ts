import { beforeEach, describe, expect, it, vi } from "vitest";
import { hasDraft, loadDraft, discardDraft, DRAFT_KEY } from "./use-autosave";
import type { ModelFeature, ModelReceiver } from "./types";

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
