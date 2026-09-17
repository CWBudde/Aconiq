import { beforeEach, describe, expect, it } from "vitest";
import { useMapStore } from "./map-store";
import { DEFAULT_BASEMAP, readStoredBasemap } from "./basemap";

/**
 * The store holds what the map looks like, not what it contains — basemap
 * choice, whether its tiles are reachable, and layer-group visibility. Its one
 * rule with any depth is that an unrecorded group counts as visible, which is
 * what lets `LAYER_IDS` grow without seeding an entry for every new group.
 */

beforeEach(() => {
  localStorage.clear();
  useMapStore.setState({
    basemap: DEFAULT_BASEMAP,
    tilesFailed: false,
    layerVisibility: {},
  });
});

describe("useMapStore basemap", () => {
  it("starts on the default basemap", () => {
    expect(useMapStore.getState().basemap).toBe(DEFAULT_BASEMAP);
  });

  it("switches basemap", () => {
    useMapStore.getState().setBasemap("dark");
    expect(useMapStore.getState().basemap).toBe("dark");
  });

  it("persists the choice, so a reload comes back on it", () => {
    // The map is rebuilt from the store on every mount; without this the
    // picker's choice lasted exactly as long as the tab.
    useMapStore.getState().setBasemap("bright");
    expect(readStoredBasemap()).toBe("bright");
  });
});

describe("useMapStore tile failures", () => {
  /**
   * The flag the basemap style is selected from. It lives here rather than in
   * `MapView` because flipping it *rebuilds* `MapView` — a new `Map` instance
   * is how the model layers get re-added, since `ModelLayers` syncs on data
   * and not on `styledata`.
   */
  it("starts with the tiles assumed to work", () => {
    expect(useMapStore.getState().tilesFailed).toBe(false);
  });

  it("records a failure and clears it on retry", () => {
    useMapStore.getState().reportTilesFailed();
    expect(useMapStore.getState().tilesFailed).toBe(true);

    useMapStore.getState().clearTilesFailed();
    expect(useMapStore.getState().tilesFailed).toBe(false);
  });

  it("stays raised while the failures keep arriving", () => {
    // Every tile that cannot be fetched fires its own `error`, and the map is
    // rebuilt on the first one; repeating the report must not flap the flag.
    useMapStore.getState().reportTilesFailed();
    useMapStore.getState().reportTilesFailed();
    expect(useMapStore.getState().tilesFailed).toBe(true);
  });

  it("is not tangled up with the basemap choice", () => {
    // The two are independent: a fallback must not forget which basemap to
    // come back to, and picking one must not clear a live failure.
    useMapStore.getState().reportTilesFailed();
    useMapStore.getState().setBasemap("dark");

    expect(useMapStore.getState().basemap).toBe("dark");
    expect(useMapStore.getState().tilesFailed).toBe(true);
  });
});

describe("useMapStore layer visibility", () => {
  it("records nothing until a group is touched", () => {
    // Absence is the signal the default is still in force; pre-seeding every
    // group would make "never touched" and "explicitly shown" indistinguishable.
    expect(useMapStore.getState().layerVisibility).toEqual({});
  });

  it("treats an unrecorded group as visible, so the first toggle hides it", () => {
    // The rule that matters. Read as `?? false`, the first click on a layer
    // that is plainly visible on screen would set it to "visible" and do
    // nothing — the user clicks twice to hide anything.
    useMapStore.getState().toggleLayer("buildings");
    expect(useMapStore.getState().layerVisibility["buildings"]).toBe(false);
  });

  it("toggles back to visible", () => {
    useMapStore.getState().toggleLayer("buildings");
    useMapStore.getState().toggleLayer("buildings");
    expect(useMapStore.getState().layerVisibility["buildings"]).toBe(true);
  });

  it("leaves the other groups alone when one is toggled", () => {
    // Each setter rebuilds the map, so a missing spread would silently drop
    // every other group back to its default.
    useMapStore.getState().setLayerVisible("sources", false);
    useMapStore.getState().toggleLayer("buildings");

    expect(useMapStore.getState().layerVisibility).toEqual({
      sources: false,
      buildings: false,
    });
  });

  it("sets a group's visibility outright, regardless of what it was", () => {
    // `setLayerVisible` is how a caller that knows the wanted end state says
    // so; unlike `toggleLayer`, repeating it is a no-op.
    useMapStore.getState().setLayerVisible("contours", true);
    useMapStore.getState().setLayerVisible("contours", true);
    expect(useMapStore.getState().layerVisibility["contours"]).toBe(true);

    useMapStore.getState().setLayerVisible("contours", false);
    expect(useMapStore.getState().layerVisibility["contours"]).toBe(false);
  });
});
