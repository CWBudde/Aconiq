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
interface DrawStub {
  setMode: ReturnType<typeof vi.fn>;
  stop: ReturnType<typeof vi.fn>;
  addFeatures: ReturnType<typeof vi.fn>;
  selectFeature: ReturnType<typeof vi.fn>;
  removeFeatures: ReturnType<typeof vi.fn>;
  features: GeoJSON.Feature[];
  /** Plays a terra-draw event, which is how a test stands in for the user. */
  emit: (event: string, ...args: unknown[]) => void;
}

const instances: DrawStub[] = [];

vi.mock("terra-draw", () => {
  type Handler = (...args: never[]) => void;

  class TerraDraw {
    features: GeoJSON.Feature[] = [];
    private handlers: Record<string, Handler[]> = {};
    setMode = vi.fn();
    start = vi.fn();
    stop = vi.fn();
    on = vi.fn((event: string, handler: Handler) => {
      (this.handlers[event] ??= []).push(handler);
    });
    // A real store, not an empty snapshot: the editing path adds a feature,
    // drags it and reads it back, so a stub that forgets what it was given
    // could not tell a committed reshape from a dropped one.
    getSnapshot = vi.fn(() => this.features);
    addFeatures = vi.fn((features: GeoJSON.Feature[]) => {
      this.features.push(...features);
      return features.map((feature) => ({ id: feature.id, valid: true }));
    });
    selectFeature = vi.fn();
    removeFeatures = vi.fn((ids: (string | number)[]) => {
      this.features = this.features.filter(
        (feature) => !ids.includes(feature.id as string),
      );
    });
    emit(event: string, ...args: unknown[]) {
      for (const handler of this.handlers[event] ?? []) {
        (handler as (...a: unknown[]) => void)(...args);
      }
    }
    constructor() {
      instances.push(this as unknown as DrawStub);
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
const finished: string[] = [];

function Consumer() {
  api = useDrawContext();
  return <span data-testid="mode">{api.activeMode}</span>;
}

/** A stand-in for the MapLibre map; the adapter is stubbed, so it is opaque. */
const stubMap = {} as Map;

function tree(map: Map | null) {
  return (
    <MapContext value={map}>
      <DrawProvider
        onFinish={(mode) => {
          finished.push(mode);
        }}
      >
        <Consumer />
      </DrawProvider>
    </MapContext>
  );
}

function renderWithin(map: Map | null) {
  return render(tree(map));
}

beforeEach(() => {
  instances.length = 0;
  finished.length = 0;
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

  it("arms the mode that was requested before the map existed", () => {
    // `MapView` renders its children while `map` is still null, so
    // `/model?draw=1` reaches `setMode` before terra-draw exists. The request
    // is consumed once and the query flag stripped, so nothing retries it:
    // without a replay on initialization the toolbar read Point while
    // terra-draw sat in "static" and clicks did nothing.
    const view = renderWithin(null);
    act(() => {
      api?.setMode("point");
    });
    expect(instances).toHaveLength(0);

    view.rerender(tree(stubMap));

    expect(instances).toHaveLength(1);
    expect(instances[0]?.setMode).toHaveBeenCalledWith("point");
    expect(instances[0]?.setMode).not.toHaveBeenCalledWith("static");
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

describe("DrawProvider editing surface", () => {
  const point: GeoJSON.Feature = {
    type: "Feature",
    id: "src-1",
    properties: { mode: "point" },
    geometry: { type: "Point", coordinates: [10, 51] },
  };

  it("feeds a feature into terra-draw and takes it back out", () => {
    // Select mode was built with draggable features and draggable, deletable
    // midpoints from the start — it was simply never given anything, because
    // nothing in the app called `addFeatures`. This is that path.
    renderWithin(stubMap);

    act(() => {
      expect(api?.addFeatures([point])).toBe(true);
      api?.selectFeature("src-1");
    });

    expect(instances[0]?.addFeatures).toHaveBeenCalledWith([point]);
    expect(instances[0]?.selectFeature).toHaveBeenCalledWith("src-1");
    expect(api?.getFeature("src-1")).toEqual(point);

    act(() => {
      api?.removeFeatures(["src-1"]);
    });
    expect(api?.getFeature("src-1")).toBeUndefined();
  });

  it("swallows a removal of a feature terra-draw no longer holds", () => {
    // Every caller is a teardown path that cannot know whether a mode change
    // or a map rebuild already cleared the store underneath it.
    renderWithin(stubMap);
    instances[0]?.removeFeatures.mockImplementation(() => {
      throw new Error("No feature with this id");
    });

    expect(() => {
      api?.removeFeatures(["gone"]);
    }).not.toThrow();
  });

  it("fans terra-draw's selection events out to its subscribers", () => {
    renderWithin(stubMap);
    const onChange = vi.fn();
    const onSelect = vi.fn();
    const onDeselect = vi.fn();

    let unsubscribe: (() => void) | undefined;
    act(() => {
      unsubscribe = api?.subscribeSelection({ onChange, onSelect, onDeselect });
    });

    act(() => {
      instances[0]?.emit("select", "src-1");
      instances[0]?.emit("change", ["src-1", 7], "update");
      instances[0]?.emit("deselect", "src-1");
    });

    expect(onSelect).toHaveBeenCalledWith("src-1");
    // Ids arrive as strings whatever terra-draw's id strategy mints, so a
    // subscriber can compare them against a model id without converting.
    expect(onChange).toHaveBeenCalledWith(["src-1", "7"], "update");
    expect(onDeselect).toHaveBeenCalledWith("src-1");

    act(() => {
      unsubscribe?.();
      instances[0]?.emit("select", "src-2");
    });
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("hands a finish to the editing subscriber while select mode is armed", () => {
    // The regression guard for the editing path, in both directions. Terra-draw
    // fires "finish" at the end of a *drag* too, and the finish handler is
    // written for a newly drawn shape: it resets the mode to "static" and
    // removes the feature, which in select mode disarms the tool mid-edit and
    // deletes the feature being reshaped. But the event is still the end of the
    // gesture, and the only one that says so — terra-draw leaves a dropped
    // feature selected, so "deselect" comes once for a whole run of drags — so
    // swallowing it outright left the reshape's owner with no drag boundary.
    vi.useFakeTimers();
    try {
      renderWithin(stubMap);
      const onFinish = vi.fn();
      act(() => {
        api?.subscribeSelection({ onFinish });
        api?.addFeatures([point]);
        api?.setMode("select");
      });
      instances[0]?.setMode.mockClear();

      act(() => {
        instances[0]?.emit("finish", "src-1");
        vi.runAllTimers();
      });

      expect(onFinish).toHaveBeenCalledWith("src-1");
      expect(finished).toEqual([]);
      expect(instances[0]?.setMode).not.toHaveBeenCalled();
      expect(instances[0]?.removeFeatures).not.toHaveBeenCalled();
      expect(screen.getByTestId("mode")).toHaveTextContent("select");
      expect(api?.getFeature("src-1")).toEqual(point);
    } finally {
      vi.useRealTimers();
    }
  });

  it("reports the instance being rebuilt, which a basemap switch does", () => {
    // A basemap switch replaces the MapLibre map, and terra-draw with it: the
    // new instance starts with an empty feature store. Nothing else about the
    // API moves — `activeMode` is React state and survives — so a consumer that
    // has put a feature in there has no other way to learn its copy is gone.
    const view = renderWithin(stubMap);
    const before = api?.instanceEpoch;
    act(() => {
      api?.addFeatures([point]);
    });
    expect(api?.getFeature("src-1")).toEqual(point);

    view.rerender(tree({} as Map));

    expect(instances).toHaveLength(2);
    expect(api?.instanceEpoch).not.toBe(before);
    expect(api?.getFeature("src-1")).toBeUndefined();
  });

  it("still finishes a drawn shape in every other mode", () => {
    vi.useFakeTimers();
    try {
      renderWithin(stubMap);
      act(() => {
        api?.addFeatures([point]);
        api?.setMode("point");
      });

      act(() => {
        instances[0]?.emit("finish", "src-1");
        vi.runAllTimers();
      });

      expect(finished).toEqual(["point"]);
      expect(instances[0]?.removeFeatures).toHaveBeenCalledWith(["src-1"]);
      expect(screen.getByTestId("mode")).toHaveTextContent("static");
    } finally {
      vi.useRealTimers();
    }
  });
});
