import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router";
import type { Map } from "maplibre-gl";
import MapPage from "./map";
import { MapContext } from "@/map/use-map";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The real `MapView` needs WebGL, but it is also what provides `MapContext` —
 * and a stub that dropped its children was half of why the dead draw wiring
 * went unnoticed. This one keeps both: it renders the children and provides
 * the context, so everything laid over the map is exercised.
 */
vi.mock("@/map/map-view", () => ({
  MapView: ({ children }: { children?: React.ReactNode }) => (
    <MapContext value={{} as Map}>
      <div data-testid="map-view">{children}</div>
    </MapContext>
  ),
}));
vi.mock("@/map/model-layers", () => ({
  ModelLayers: () => null,
}));
vi.mock("@/map/layer-control", () => ({
  LayerControl: () => null,
}));
vi.mock("@/map/coordinate-display", () => ({
  CoordinateDisplay: () => null,
}));
vi.mock("@/map/feature-popup", () => ({
  FeaturePopup: () => null,
}));
vi.mock("@/map/draw-toolbar", () => ({
  DrawToolbar: ({
    activeMode,
    disabled,
    disabledReason,
  }: {
    activeMode: string;
    disabled?: boolean;
    disabledReason?: string;
  }) => (
    <div
      data-testid="draw-toolbar"
      data-mode={activeMode}
      data-disabled={String(disabled === true)}
      data-disabled-reason={disabledReason ?? ""}
    />
  ),
}));
// Renders the id it was handed, which is what `?select=` has to reach.
vi.mock("@/map/feature-editor", () => ({
  FeatureEditor: ({ featureId }: { featureId: string | null }) =>
    featureId === null ? null : (
      <div data-testid="feature-editor">{featureId}</div>
    ),
}));
// Renders only when it is open, which is the signal that a drawn shape
// reached the page at all.
vi.mock("@/map/new-feature-dialog", () => ({
  NewFeatureDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="new-feature-dialog" /> : null,
}));
vi.mock("@/map/validation-panel", () => ({
  ValidationPanel: () => null,
}));
vi.mock("@/map/undo-redo-bar", () => ({
  UndoRedoBar: () => null,
}));
// terra-draw is stubbed rather than `useDraw`, so the path from a control to
// the draw instance actually runs here. Mocking the hook is what hid the
// `MapContext` defect; `map/draw-provider.test.tsx` covers the wiring in
// detail.
vi.mock("terra-draw", () => {
  const mode = vi.fn();
  return {
    TerraDraw: class {
      setMode = vi.fn();
      start = vi.fn();
      stop = vi.fn();
      // Captured so a test can play terra-draw finishing a shape. The adapter
      // hands the page WGS84 coordinates whatever the model is stored in,
      // which is the whole reason the page has to refuse.
      on = vi.fn((event: string, handler: (id: string) => void) => {
        if (event === "finish") draw.finish = handler;
      });
      getSnapshot = vi.fn(() => [
        {
          id: "drawn-1",
          type: "Feature",
          properties: {},
          geometry: { type: "Polygon", coordinates: [DRAWN_RING] },
        },
      ]);
      removeFeatures = vi.fn();
    },
    TerraDrawPointMode: mode,
    TerraDrawLineStringMode: mode,
    TerraDrawPolygonMode: mode,
    TerraDrawSelectMode: mode,
    TerraDrawRenderMode: mode,
  };
});
vi.mock("terra-draw-maplibre-gl-adapter", () => ({
  TerraDrawMapLibreGLAdapter: vi.fn(),
}));

/** The handler `useDraw` registers on terra-draw's "finish" event. */
const draw: { finish: ((id: string) => void) | null } = { finish: null };

/** A ring in WGS84, which is all terra-draw ever emits. */
const DRAWN_RING = [
  [10, 51],
  [10.1, 51],
  [10.1, 51.1],
  [10, 51],
];

describe("MapPage", () => {
  beforeEach(() => {
    draw.finish = null;
    useModelStore.getState().reset();
  });

  function renderPage() {
    render(
      <MemoryRouter>
        <MapPage />
      </MemoryRouter>,
    );
  }

  function LocationProbe() {
    return <div data-testid="location-search">{useLocation().search}</div>;
  }

  function renderPageAt(entry: string) {
    render(
      <MemoryRouter initialEntries={[entry]}>
        <MapPage />
        <LocationProbe />
      </MemoryRouter>,
    );
  }

  const source: ModelFeature = {
    id: "src-1",
    kind: "source",
    sourceType: "point",
    geometry: { type: "Point", coordinates: [10, 51] },
  };

  it("mounts the map even with nothing in the model", () => {
    // Was: "shows the workspace start screen before geometry is loaded", which
    // asserted `map-view` was *absent*. The start panel replaced the canvas,
    // the toolbar and the validation panel, so the screen telling the user to
    // start a workspace was the one screen with no way to draw one.
    renderPage();

    expect(screen.getByTestId("map-view")).toBeInTheDocument();
    expect(screen.getByTestId("draw-toolbar")).toBeInTheDocument();
  });

  it("lays the start panel over the map as a labelled region", () => {
    renderPage();

    const panel = screen.getByRole("region", {
      name: m.heading_map_workspace(),
    });
    expect(panel).toBeVisible();
    expect(
      screen.getByRole("link", { name: m.action_import_data() }),
    ).toHaveAttribute("href", "/import");
  });

  it("arms point mode and clears the panel on Start drawing", () => {
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    );

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "point",
    );
    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
  });

  it("dismisses the panel without arming a tool on Close", () => {
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: m.action_close() }));

    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
  });

  it("brings the panel back when the model empties again", () => {
    // The dismissal covers one empty-model episode, not the whole visit: a
    // user who draws a feature and then deletes the last one is back on an
    // empty map, which is the one state the hint exists for.
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: m.action_close() }));
    act(() => {
      useModelStore.getState().addFeature(source);
    });
    act(() => {
      useModelStore.getState().removeFeature(source.id);
    });

    expect(
      screen.getByRole("region", { name: m.heading_map_workspace() }),
    ).toBeVisible();
  });

  it("opens the editor on the feature the select parameter names", () => {
    // The link the import page's done step builds for a finding.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("feature-editor")).toHaveTextContent("src-1");
  });

  it("strips the select parameter once it has been honoured", () => {
    // Otherwise a Back re-opens the editor on a feature the reader has moved
    // on from.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("location-search").textContent).toBe("");
  });

  it("does not show the panel when the model already has content", () => {
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPage();

    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
    expect(screen.getByTestId("map-view")).toBeInTheDocument();
  });

  it("takes a drawn shape into the page for a model the map draws in", () => {
    // The positive control for the refusal below: with the store in WGS84 the
    // finished shape reaches the page and opens the new-feature dialog.
    renderPageAt("/model?draw=1");

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-disabled",
      "false",
    );
    act(() => {
      draw.finish?.("drawn-1");
    });

    expect(screen.getByTestId("new-feature-dialog")).toBeInTheDocument();
  });

  it("disables drawing and refuses a finished shape in a metric model", () => {
    // terra-draw emits WGS84 whatever the store is in, and the finish handler
    // writes straight into the model. Accepting the shape would mix degrees
    // into a model held in metres — a round trip through the map quietly
    // becoming the model.
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const fixtureFeatures = useModelStore.getState().features;
    renderPageAt("/model?draw=1");

    const toolbar = screen.getByTestId("draw-toolbar");
    expect(toolbar).toHaveAttribute("data-disabled", "true");
    expect(toolbar.getAttribute("data-disabled-reason")).toContain(
      "EPSG:25832",
    );

    act(() => {
      draw.finish?.("drawn-1");
    });

    expect(screen.queryByTestId("new-feature-dialog")).toBeNull();
    expect(useModelStore.getState().calcArea).toBeNull();
    // Reference identity: nothing on the map side may write a coordinate back.
    expect(useModelStore.getState().features).toBe(fixtureFeatures);
    expect(useModelStore.getState().features[0]).toBe(fixtureFeatures[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");
  });

  it("does not arm a tool from the draw parameter in a metric model", () => {
    // `?draw=1` was a way past the disabled toolbar: it armed point mode, the
    // user drew a shape, and `handleDrawFinish` dropped it without a word.
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const fixtureFeatures = useModelStore.getState().features;
    renderPageAt("/model?draw=1");

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
    // Stripped even when refused, or a reload would re-ask.
    expect(screen.getByTestId("location-search").textContent).toBe("");

    act(() => {
      draw.finish?.("drawn-1");
    });

    expect(screen.queryByTestId("new-feature-dialog")).toBeNull();
    // Reference identity: nothing on the map side may write a coordinate back.
    expect(useModelStore.getState().features).toBe(fixtureFeatures);
    expect(useModelStore.getState().features[0]).toBe(fixtureFeatures[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");
  });

  it("disables Start drawing in an empty metric workspace and says why", () => {
    // The other entry point the finish-time refusal did not cover. The panel
    // stays up on a refused `?draw=1` precisely because it is what carries the
    // reason — the toolbar can only say it in a tooltip.
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPageAt("/model?draw=1");

    expect(
      screen.getByRole("region", { name: m.heading_map_workspace() }),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    ).toBeDisabled();
    // The toolbar's wording, not a second one invented for this surface.
    expect(
      screen.getByText(m.msg_draw_disabled_crs({ crs: "EPSG:25832" })),
    ).toBeVisible();
    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
  });
});
