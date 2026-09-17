import { beforeEach, describe, expect, it } from "vitest";
import { useMapStore } from "./map-store";

/**
 * The store holds what the map looks like, not what it contains — basemap
 * choice and layer-group visibility. Its one rule with any depth is that an
 * unrecorded group counts as visible, which is what lets `LAYER_IDS` grow
 * without seeding an entry for every new group.
 */

beforeEach(() => {
  useMapStore.setState({ basemap: "light", layerVisibility: {} });
});

describe("useMapStore basemap", () => {
  it("starts on the light basemap", () => {
    expect(useMapStore.getState().basemap).toBe("light");
  });

  it("switches basemap", () => {
    useMapStore.getState().setBasemap("dark");
    expect(useMapStore.getState().basemap).toBe("dark");
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
