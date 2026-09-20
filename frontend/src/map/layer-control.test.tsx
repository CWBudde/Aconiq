import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { LayerControl } from "./layer-control";
import {
  BASEMAP_IDS,
  basemapLabel,
  readStoredBasemap,
  type BasemapId,
} from "./basemap";
import {
  MODEL_LAYER_GROUPS,
  RESULT_CONTOUR_LAYERS,
  RESULT_LAYER_GROUPS,
  RESULT_RASTER_LAYERS,
  RESULT_RECEIVER_LAYERS,
  type LayerGroup,
} from "./layers";
import { useMapStore } from "./map-store";
import { MapContext } from "./use-map";
import { m } from "@/i18n/messages";

/**
 * The control is plain DOM over the map, so nothing here needs WebGL — but it
 * is the one place the store's visibility state and MapLibre's layout property
 * have to agree, and either half can drift without the other noticing. Both
 * halves are asserted on every toggle.
 *
 * `pages/map.test.tsx` mocks this component out entirely, which is why it has
 * its own file rather than more page tests.
 */

/** The `setLayoutProperty` surface, and only that. */
class FakeMap {
  readonly calls: [string, string, unknown][] = [];
  /** Layer ids that are not on the style yet, the way MapLibre reports them. */
  missing = new Set<string>();

  setLayoutProperty(layerId: string, name: string, value: unknown) {
    if (this.missing.has(layerId)) {
      throw new Error(
        `The layer '${layerId}' does not exist in the map's style`,
      );
    }
    this.calls.push([layerId, name, value]);
  }
}

function renderControl(map: FakeMap | null) {
  render(
    <MapContext value={map as unknown as MapLibreMap | null}>
      <LayerControl />
    </MapContext>,
  );
}

function toggleFor(label: string): HTMLElement {
  return screen.getByRole("button", { name: m.action_hide_layer({ label }) });
}

/** A group by id, so a test names the group it means rather than an index. */
function group(id: string): LayerGroup {
  const found = [...MODEL_LAYER_GROUPS, ...RESULT_LAYER_GROUPS].find(
    (candidate) => candidate.id === id,
  );
  if (!found) throw new Error(`no layer group ${id}`);
  return found;
}

function visibilityOf(map: FakeMap, layerId: string): unknown[] {
  return map.calls
    .filter(([id, name]) => id === layerId && name === "visibility")
    .map(([, , value]) => value);
}

beforeEach(() => {
  useMapStore.setState({ basemap: "light", layerVisibility: {} });
});

describe("LayerControl", () => {
  it("offers a toggle for every model and result group", () => {
    // A group added to `layers.ts` with no control is invisible to the user;
    // this is the only thing that would notice.
    renderControl(new FakeMap());

    for (const candidate of [...MODEL_LAYER_GROUPS, ...RESULT_LAYER_GROUPS]) {
      expect(toggleFor(candidate.label())).toBeInTheDocument();
    }
  });

  it("names the action, not the state, so the label says what a click does", () => {
    // "Buildings" alone leaves a screen-reader user guessing whether pressing
    // it shows or hides. The label flips with the state.
    const map = new FakeMap();
    renderControl(map);
    const label = group("buildings").label();

    fireEvent.click(toggleFor(label));

    expect(
      screen.getByRole("button", { name: m.action_show_layer({ label }) }),
    ).toBeInTheDocument();
  });

  it("hides every layer id in the group, not just the first", () => {
    // Buildings draw as a fill plus an outline. Hiding only one leaves the
    // outlines floating over the basemap with nothing to explain them.
    const map = new FakeMap();
    renderControl(map);
    const buildings = group("buildings");

    fireEvent.click(toggleFor(buildings.label()));

    expect(buildings.layerIds.length).toBeGreaterThan(1);
    for (const layerId of buildings.layerIds) {
      expect(visibilityOf(map, layerId)).toEqual(["none"]);
    }
  });

  it("shows the layers again on a second click", () => {
    const map = new FakeMap();
    renderControl(map);
    const barriers = group("barriers");
    const label = barriers.label();

    fireEvent.click(toggleFor(label));
    fireEvent.click(
      screen.getByRole("button", { name: m.action_show_layer({ label }) }),
    );

    for (const layerId of barriers.layerIds) {
      expect(visibilityOf(map, layerId)).toEqual(["none", "visible"]);
    }
  });

  it("keeps toggling when the layer is not on the style yet", () => {
    // The receiver-level layer only exists once a completed run has been
    // projected, so toggling it on a fresh project throws out of MapLibre.
    // Unguarded the throw escapes the handler, and the store update — which
    // already happened — would disagree with the map for good.
    //
    // The group carries exactly one layer, so this also pins that the guard is
    // per layer id and not a try around the whole loop: the model groups below
    // are what prove the loop runs on.
    const map = new FakeMap();
    const levels = group("receiver-levels");
    const [only] = levels.layerIds;
    if (only === undefined) {
      throw new Error("receiver-levels group lost its layer");
    }
    map.missing.add(only);
    renderControl(map);

    expect(() => {
      fireEvent.click(toggleFor(levels.label()));
    }).not.toThrow();

    expect(visibilityOf(map, only)).toEqual([]);
    expect(useMapStore.getState().layerVisibility[levels.id]).toBe(false);
  });

  it("offers no toggle for a layer nothing draws", () => {
    // The invariant is that every id in a result group is added by some
    // component, so the set is compared against the layer specifications
    // `ResultLayers` actually adds. A dead control is worse than a missing one:
    // it reports a state that is not there.
    //
    // `contours` is why this test exists. The group sat in the control for a
    // layer nothing added, because `aconiq export --format contour-geojson`
    // was the only producer and browser mode wrote none at all. It is in the
    // list now because the generation moved behind the kernel boundary and
    // `ResultLayers` adds the layers in both modes — so the group and the
    // specifications have to be added or removed together, which is exactly
    // what this compares.
    const offered = new Set(RESULT_LAYER_GROUPS.flatMap((g) => g.layerIds));
    const added = [
      ...RESULT_RASTER_LAYERS,
      ...RESULT_CONTOUR_LAYERS,
      ...RESULT_RECEIVER_LAYERS,
    ].map((layer) => layer.id);

    expect([...offered].sort()).toEqual([...added].sort());
  });

  it("still records the choice when there is no map", () => {
    // `MapView` renders its children while the map is still null, so the
    // control is clickable before MapLibre exists. The store is updated above
    // the map guard, so the choice survives and the next sync applies it.
    renderControl(null);
    const sources = group("sources");

    fireEvent.click(toggleFor(sources.label()));

    expect(useMapStore.getState().layerVisibility[sources.id]).toBe(false);
  });

  it("groups itself under a named region with model and result sections", () => {
    // The control sits beside MapLibre's own navigation buttons; without the
    // group name the two sets of unlabelled icon buttons run together.
    const map = new FakeMap();
    renderControl(map);

    const region = screen.getByRole("group", { name: m.label_layers() });
    expect(region).toHaveTextContent(m.section_model());
    expect(region).toHaveTextContent(m.section_results());
  });

  it("reflects a visibility already in the store when it mounts", () => {
    // Navigating away from the map and back remounts the control against a
    // store that outlives it. Reading `defaultVisible` unconditionally would
    // show every layer as visible while the map still had them hidden.
    const receivers = group("receivers");
    useMapStore.setState({ layerVisibility: { [receivers.id]: false } });

    renderControl(new FakeMap());

    expect(
      screen.getByRole("button", {
        name: m.action_show_layer({ label: receivers.label() }),
      }),
    ).toBeInTheDocument();
  });
});

describe("the basemap picker", () => {
  function pickerButton(id: BasemapId): HTMLElement {
    return within(
      screen.getByRole("group", { name: m.section_basemap() }),
    ).getByRole("button", { name: basemapLabel(id) });
  }

  it("offers every basemap and marks the current one", () => {
    // `aria-pressed`, not a disabled current option: the picker refuses
    // nothing, and a disabled button would drop out of the tab order.
    renderControl(new FakeMap());

    for (const id of BASEMAP_IDS) {
      expect(pickerButton(id)).toHaveAttribute(
        "aria-pressed",
        String(id === "light"),
      );
      expect(pickerButton(id)).not.toBeDisabled();
      expect(pickerButton(id)).not.toHaveAttribute("aria-disabled");
    }
  });

  it("marks the current basemap visibly, not only to a screen reader", () => {
    // `aria-pressed` is the state; this is the pixel that carries it. The
    // picker used to mark the current basemap with the `secondary` variant, a
    // few per cent of grey on a translucent panel — present in the DOM,
    // invisible on screen. The filled variant is what a sighted reader
    // actually sees, so it is asserted rather than left to a theme.
    renderControl(new FakeMap());

    expect(pickerButton("light").className).toContain("bg-primary");
    for (const id of BASEMAP_IDS.filter((other) => other !== "light")) {
      expect(pickerButton(id).className).not.toContain("bg-primary");
    }
  });

  it("moves the visible mark with the choice", () => {
    renderControl(new FakeMap());
    fireEvent.click(pickerButton("dark"));

    expect(pickerButton("dark").className).toContain("bg-primary");
    expect(pickerButton("light").className).not.toContain("bg-primary");
  });

  it("marks the choice without widening the button", () => {
    // The three buttons are equal columns of a grid that sizes to its widest
    // cell, inside a panel that sizes to its content — so anything the mark
    // adds to the current button widens the whole panel, by a different amount
    // per label, and pushes it into the pill beside it. The mark has to be
    // paint, not content.
    renderControl(new FakeMap());

    for (const id of BASEMAP_IDS) {
      expect(pickerButton(id).querySelector("svg")).toBeNull();
      expect(pickerButton(id)).toHaveAccessibleName(basemapLabel(id));
    }
  });

  it("writes the choice to the store, which is what rebuilds the map", () => {
    // The store is the only channel: `MapView` keys its init effect on it, so
    // nothing here touches MapLibre. A picker that called `setStyle` instead
    // would drop every model layer until the next model edit.
    const map = new FakeMap();
    renderControl(map);

    fireEvent.click(pickerButton("dark"));

    expect(useMapStore.getState().basemap).toBe("dark");
    expect(map.calls).toEqual([]);
    expect(pickerButton("dark")).toHaveAttribute("aria-pressed", "true");
    expect(pickerButton("light")).toHaveAttribute("aria-pressed", "false");
  });

  it("persists the choice across a remount", () => {
    renderControl(new FakeMap());
    fireEvent.click(pickerButton("bright"));

    expect(readStoredBasemap()).toBe("bright");
  });

  it("reflects a basemap already in the store when it mounts", () => {
    useMapStore.setState({ basemap: "dark" });
    renderControl(new FakeMap());

    expect(pickerButton("dark")).toHaveAttribute("aria-pressed", "true");
  });
});
