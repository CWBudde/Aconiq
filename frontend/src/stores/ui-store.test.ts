import { beforeEach, describe, expect, it } from "vitest";
import { useUIStore } from "./ui-store";

beforeEach(() => {
  // Reset to initial state.
  useUIStore.setState({ activeNav: "map", runInProgress: false });
});

describe("useUIStore", () => {
  it("has correct initial state", () => {
    const state = useUIStore.getState();
    expect(state.activeNav).toBe("map");
    expect(state.runInProgress).toBe(false);
  });

  it("setActiveNav updates active nav", () => {
    useUIStore.getState().setActiveNav("run");
    expect(useUIStore.getState().activeNav).toBe("run");
  });

  it("setRunInProgress toggles the flag", () => {
    useUIStore.getState().setRunInProgress(true);
    expect(useUIStore.getState().runInProgress).toBe(true);
    useUIStore.getState().setRunInProgress(false);
    expect(useUIStore.getState().runInProgress).toBe(false);
  });
});
