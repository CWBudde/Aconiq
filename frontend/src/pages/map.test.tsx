import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import MapPage from "./map";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

vi.mock("@/api", () => ({
  useProjectStatus: () => ({
    isLoading: false,
    isError: false,
    error: null,
    data: {
      name: "Demo Project",
      crs: "EPSG:4326",
      scenario_count: 2,
      run_count: 1,
    },
  }),
}));

vi.mock("@/map/map-view", () => ({
  MapView: () => <div data-testid="map-view" />,
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
  DrawToolbar: () => null,
}));
vi.mock("@/map/feature-editor", () => ({
  FeatureEditor: () => null,
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
vi.mock("@/map/use-draw", () => ({
  useDraw: () => ({
    activeMode: null,
    setMode: vi.fn(),
    cancel: vi.fn(),
  }),
}));

describe("MapPage", () => {
  beforeEach(() => {
    useModelStore.getState().reset();
  });

  it("shows the workspace start screen before geometry is loaded", () => {
    render(
      <MemoryRouter>
        <MapPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("heading", { name: m.heading_map_workspace() }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: m.nav_import() })).toBeVisible();
    expect(screen.getByRole("link", { name: m.nav_status() })).toBeVisible();
    expect(screen.queryByTestId("map-view")).not.toBeInTheDocument();
    expect(screen.getByText("Demo Project")).toBeInTheDocument();
  });
});
