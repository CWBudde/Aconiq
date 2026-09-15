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
  DrawToolbar: ({ activeMode }: { activeMode: string }) => (
    <div data-testid="draw-toolbar" data-mode={activeMode} />
  ),
}));
// Renders the id it was handed, which is what `?select=` has to reach.
vi.mock("@/map/feature-editor", () => ({
  FeatureEditor: ({ featureId }: { featureId: string | null }) =>
    featureId === null ? null : (
      <div data-testid="feature-editor">{featureId}</div>
    ),
}));
vi.mock("@/map/new-feature-dialog", () => ({
  NewFeatureDialog: () => null,
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
      on = vi.fn();
      getSnapshot = vi.fn(() => []);
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

describe("MapPage", () => {
  beforeEach(() => {
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
    useModelStore.getState().loadFeatures([source]);
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("feature-editor")).toHaveTextContent("src-1");
  });

  it("strips the select parameter once it has been honoured", () => {
    // Otherwise a Back re-opens the editor on a feature the reader has moved
    // on from.
    useModelStore.getState().loadFeatures([source]);
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("location-search").textContent).toBe("");
  });

  it("does not show the panel when the model already has content", () => {
    useModelStore.getState().loadFeatures([source]);
    renderPage();

    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
    expect(screen.getByTestId("map-view")).toBeInTheDocument();
  });
});
