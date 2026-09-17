import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { LayerControl } from "./layer-control";
import {
  MODEL_LAYER_GROUPS,
  RESULT_LAYER_GROUPS,
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
      throw new Error(`The layer '${layerId}' does not exist in the map's style`);
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
    // The result layers only exist once a run is displayed, so toggling them
    // on a fresh project throws out of MapLibre. Unguarded, the first throw
    // would abort the loop and leave the group half-hidden, and the store
    // update — which already happened — would disagree with the map.
    const map = new FakeMap();
    const contours = group("contours");
    const [first, second] = contours.layerIds;
    if (first === undefined || second === undefined) {
      throw new Error("contours group lost a layer");
    }
    map.missing.add(first);
    renderControl(map);

    expect(() => {
      fireEvent.click(toggleFor(contours.label()));
    }).not.toThrow();

    expect(visibilityOf(map, second)).toEqual(["none"]);
    expect(useMapStore.getState().layerVisibility[contours.id]).toBe(false);
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
