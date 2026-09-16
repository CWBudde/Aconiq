import { beforeEach, describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import { useModelValidation } from "./use-model-validation";
import { useModelStore } from "./model-store";
import type { ModelFeature, ModelReceiver } from "./types";

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10, 51] },
};

/** A receiver the feature-only validator cannot see anything wrong with. */
const badReceiver: ModelReceiver = {
  id: "rcv-1",
  heightM: -1,
  geometry: { type: "Point", coordinates: [10, 51] },
};

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("useModelValidation", () => {
  it("reports an empty model as empty, not as one error", () => {
    // `validateProjectModel` pushes a synthetic `model.empty` *error*, whose
    // message is hardcoded English. Reported as a finding, a fresh install
    // would read "1 error" — in English, in the German UI.
    const { result } = renderHook(() => useModelValidation());

    expect(result.current.state).toBe("empty");
    expect(result.current.errorCount).toBe(0);
    expect(result.current.report).toBeNull();
  });

  it("reports a well-formed model as valid", () => {
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    const { result } = renderHook(() => useModelValidation());

    expect(result.current.state).toBe("valid");
    expect(result.current.errorCount).toBe(0);
  });

  it("sees a defect that only a receiver carries", () => {
    // The whole reason this hook exists: the former `validateModel` passed
    // [] for
    // receivers, so every call site that used it read this model as valid.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    useModelStore.getState().addReceiver(badReceiver);
    const { result } = renderHook(() => useModelValidation());

    expect(result.current.state).toBe("issues");
    expect(result.current.errorCount).toBeGreaterThan(0);
  });

  it("does not call a receiver-only model empty", () => {
    useModelStore.getState().addReceiver(receiver);
    const { result } = renderHook(() => useModelValidation());

    expect(result.current.state).not.toBe("empty");
  });
});
