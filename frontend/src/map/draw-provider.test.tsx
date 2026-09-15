import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import type { Map } from "maplibre-gl";
import { DrawProvider } from "./draw-provider";
import { useDrawContext } from "./use-draw-context";
import { MapContext } from "./use-map";

/**
 * terra-draw needs a real MapLibre canvas, so the library is stubbed and what
 * is asserted is the wiring: that a mode change reaches the draw instance.
 *
 * Deliberately *not* mocking `useDraw`, which is what let the defect below
 * survive — `pages/map.test.tsx` stubbed the hook out, so nothing exercised
 * the path from the toolbar to terra-draw.
 */
const instances: {
  setMode: ReturnType<typeof vi.fn>;
  stop: ReturnType<typeof vi.fn>;
}[] = [];

vi.mock("terra-draw", () => {
  class TerraDraw {
    setMode = vi.fn();
    start = vi.fn();
    stop = vi.fn();
    on = vi.fn();
    getSnapshot = vi.fn(() => []);
    removeFeatures = vi.fn();
    constructor() {
      instances.push(this as unknown as (typeof instances)[number]);
    }
  }
  // `vi.fn()` rather than an empty class: the modes are only ever constructed
  // and handed to the stub above, and an empty class is a lint finding.
  const mode = vi.fn();
  return {
    TerraDraw,
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

let api: ReturnType<typeof useDrawContext> | null = null;

function Consumer() {
  api = useDrawContext();
  return <span data-testid="mode">{api.activeMode}</span>;
}

/** A stand-in for the MapLibre map; the adapter is stubbed, so it is opaque. */
const stubMap = {} as Map;

function renderWithin(map: Map | null) {
  return render(
    <MapContext value={map}>
      <DrawProvider onFinish={() => undefined}>
        <Consumer />
      </DrawProvider>
    </MapContext>,
  );
}

beforeEach(() => {
  instances.length = 0;
  api = null;
});

describe("DrawProvider", () => {
  it("attaches terra-draw to the map it is rendered inside", () => {
    renderWithin(stubMap);
    expect(instances).toHaveLength(1);
    expect(instances[0]?.setMode).toHaveBeenCalledWith("static");
  });

  it("passes a mode change through to the draw instance", () => {
    // The regression guard. Before the provider existed, `useDraw` ran above
    // `MapView` and so outside `MapContext`: `setMode` moved React state, the
    // toolbar button lit up, and terra-draw was never told anything.
    renderWithin(stubMap);
    act(() => {
      api?.setMode("point");
    });

    expect(instances[0]?.setMode).toHaveBeenCalledWith("point");
    expect(screen.getByTestId("mode")).toHaveTextContent("point");
  });

  it("maps the calculation area onto terra-draw's polygon mode", () => {
    renderWithin(stubMap);
    act(() => {
      api?.setMode("calc-area");
    });

    expect(instances[0]?.setMode).toHaveBeenCalledWith("polygon");
    expect(screen.getByTestId("mode")).toHaveTextContent("calc-area");
  });

  it("builds nothing without a map, which is the shape of the old defect", () => {
    renderWithin(null);
    act(() => {
      api?.setMode("point");
    });

    // No instance, so no adapter, so no drawing — while the React state moves
    // and the toolbar shows the mode as active. Exactly what shipped.
    expect(instances).toHaveLength(0);
    expect(screen.getByTestId("mode")).toHaveTextContent("point");
  });

  it("survives a teardown against a map that is already gone", () => {
    // React destroys a deleted subtree's effects parent-first, so `MapView`
    // has already called `map.remove()` when this cleanup runs and terra-draw
    // tears down against a dead map. Unguarded, that threw `getSource` of
    // undefined — which React surfaced as a crashed page on whatever route the
    // user had just navigated to.
    const view = renderWithin(stubMap);
    instances[0]?.stop.mockImplementation(() => {
      throw new TypeError(
        "Cannot read properties of undefined (reading 'getSource')",
      );
    });

    expect(() => {
      view.unmount();
    }).not.toThrow();
  });
});
