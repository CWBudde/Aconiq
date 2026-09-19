import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { LAYER_IDS } from "./layers";
// The real catalogue, not a stub: the unavailable panel is the one part of
// MapView that renders copy, and asserting on a hardcoded English string
// would pass just as well after the string stopped going through i18n.
import { m } from "@/i18n/messages";

// maplibre-gl cannot run in jsdom (no WebGL), so the whole module is replaced
// by a fake whose `Map` records instances and can be told to throw. The fake
// lives in `vi.hoisted` so the same class survives `vi.resetModules()` below.
const fake = vi.hoisted(() => {
  type Handler = (event?: unknown) => void;

  interface MapOptions {
    container: HTMLElement;
    style: { name?: string; sources?: Record<string, unknown> };
    center: [number, number];
    zoom: number;
  }

  class FakeMap {
    private readonly handlers = new Map<string, Set<Handler>>();
    private readonly canvas = document.createElement("canvas");
    readonly container: HTMLElement;
    readonly options: MapOptions;
    /** What `getCenter`/`getZoom` report; the component reads them on teardown. */
    center: { lng: number; lat: number };
    zoom: number;
    /**
     * The layer ids this style holds, and what a query over each answers.
     *
     * Both are empty by default, which is the state every test written before
     * the result layers existed assumed: `getLayer` returns nothing, the
     * component filters every id out of its query, and no callback fires.
     * `withLayer` is how a test says a layer is on the map and what is under
     * the pointer in it.
     */
    readonly layers = new Set<string>();
    readonly rendered = new Map<string, { properties: unknown }[]>();
    readonly addControl = vi.fn();
    readonly remove = vi.fn();
    readonly getLayer = vi.fn((id: string) =>
      this.layers.has(id) ? { id } : undefined,
    );
    readonly queryRenderedFeatures = vi.fn(
      (_point: unknown, options?: { layers?: string[] }) =>
        (options?.layers ?? []).flatMap((id) => this.rendered.get(id) ?? []),
    );

    constructor(options: MapOptions) {
      state.constructCalls += 1;
      if (state.throwOnConstruct) {
        throw new Error("WebGL is not supported");
      }
      this.container = options.container;
      this.options = options;
      this.center = { lng: options.center[0], lat: options.center[1] };
      this.zoom = options.zoom;
      state.instances.push(this);
    }

    on(event: string, handler: Handler): this {
      let set = this.handlers.get(event);
      if (!set) {
        set = new Set();
        this.handlers.set(event, set);
      }
      set.add(handler);
      return this;
    }

    off(event: string, handler: Handler): this {
      this.handlers.get(event)?.delete(handler);
      return this;
    }

    getCanvas(): HTMLCanvasElement {
      return this.canvas;
    }

    getCenter(): { lng: number; lat: number } {
      return this.center;
    }

    getZoom(): number {
      return this.zoom;
    }

    /** Add a layer to the style, holding the features a query over it finds. */
    withLayer(id: string, features: { properties: unknown }[] = []): this {
      this.layers.add(id);
      this.rendered.set(id, features);
      return this;
    }

    /** Move the viewport the way a user's pan and zoom would. */
    jumpTo(center: { lng: number; lat: number }, zoom: number): void {
      this.center = center;
      this.zoom = zoom;
    }

    /** Invoke every handler registered for `event`, as maplibre would. */
    fire(event: string, payload?: unknown): void {
      for (const handler of this.handlers.get(event) ?? []) {
        handler(payload);
      }
    }
  }

  const state = {
    constructCalls: 0,
    throwOnConstruct: false,
    instances: [] as FakeMap[],
  };

  return { FakeMap, state };
});

vi.mock("maplibre-gl", () => ({
  default: {
    Map: fake.FakeMap,
    NavigationControl: vi.fn(),
    ScaleControl: vi.fn(),
  },
}));

const { state } = fake;

// `webglDisabledForSession` is module-level on purpose (it must survive
// remounts), so each test imports a fresh copy of the module instead. Fake
// timers go in only afterwards: the module transform needs the real ones.
async function loadMapModules() {
  const view = await import("./map-view");
  // The store has to come out of the same freshly reset registry as the
  // component, or the test would be driving a different instance than the one
  // MapView subscribes to.
  const store = await import("./map-store");
  vi.useFakeTimers();
  return { MapView: view.MapView, useMapStore: store.useMapStore };
}

async function loadMapView() {
  return (await loadMapModules()).MapView;
}

function latestInstance() {
  const instance = state.instances.at(-1);
  if (!instance) throw new Error("no map instance was created");
  return instance;
}

const MAP_LOAD_TIMEOUT_MS = 15000;

/** A rendered feature as `queryRenderedFeatures` hands it over. */
function hit(id: string) {
  return { properties: { id } };
}

beforeEach(() => {
  vi.resetModules();
  state.constructCalls = 0;
  state.throwOnConstruct = false;
  state.instances = [];
});

afterEach(() => {
  vi.useRealTimers();
});

describe("MapView", () => {
  it("renders the map container once the map loads", async () => {
    const MapView = await loadMapView();
    const { container } = render(<MapView />);

    expect(state.constructCalls).toBe(1);
    act(() => {
      latestInstance().fire("load");
    });

    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
    expect(container.contains(latestInstance().container)).toBe(true);
    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });
    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
  });

  it("shows the timeout panel without load and recovers on retry", async () => {
    const MapView = await loadMapView();
    render(<MapView />);
    const first = latestInstance();

    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });

    expect(screen.getByText(m.label_map_unavailable())).toBeInTheDocument();
    expect(screen.getByText(m.error_map_load_timeout())).toBeInTheDocument();
    // The error re-runs the init effect, whose cleanup removes the stale map.
    expect(first.remove).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: m.action_retry() }));

    expect(state.constructCalls).toBe(2);
    act(() => {
      latestInstance().fire("load");
    });
    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
  });

  it("shows the unavailable panel when the constructor throws and retries", async () => {
    state.throwOnConstruct = true;
    const MapView = await loadMapView();
    render(<MapView />);

    expect(state.constructCalls).toBe(1);
    expect(screen.getByText(m.label_map_unavailable())).toBeInTheDocument();
    expect(screen.getByText(/rendering is unavailable/)).toBeInTheDocument();

    // Still broken: the panel persists and the button stays available.
    fireEvent.click(screen.getByRole("button", { name: m.action_retry() }));
    expect(state.constructCalls).toBe(2);
    expect(screen.getByText(m.label_map_unavailable())).toBeInTheDocument();

    // Fixed in the meantime: the retry clears the session switch, so the map
    // is created and rendered.
    state.throwOnConstruct = false;
    fireEvent.click(screen.getByRole("button", { name: m.action_retry() }));
    expect(state.constructCalls).toBe(3);
    act(() => {
      latestInstance().fire("load");
    });
    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
  });

  it("keeps the session kill switch across remounts until retried", async () => {
    state.throwOnConstruct = true;
    const MapView = await loadMapView();
    const { unmount } = render(<MapView />);
    expect(state.constructCalls).toBe(1);
    unmount();

    // A remount must not even try: the switch is session-wide.
    state.throwOnConstruct = false;
    render(<MapView />);
    expect(state.constructCalls).toBe(1);
    expect(screen.getByText(m.label_map_unavailable())).toBeInTheDocument();
  });

  it("keeps the canvas mounted through a lost WebGL context", async () => {
    const MapView = await loadMapView();
    render(<MapView />);
    const instance = latestInstance();
    act(() => {
      instance.fire("load");
    });

    // MapLibre restores the context itself; all this component has to do is
    // not swap the canvas for the error panel.
    act(() => {
      instance
        .getCanvas()
        .dispatchEvent(new Event("webglcontextlost", { cancelable: true }));
    });

    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
    expect(instance.remove).not.toHaveBeenCalled();
  });

  it("carries the viewport across a rebuild", async () => {
    // A rebuild reads the mount-time props unless the map's own viewport is
    // remembered — and the workspace page computes those props once, so a
    // fallback or a basemap switch would silently re-centre on Germany.
    const { MapView, useMapStore } = await loadMapModules();
    render(<MapView center={[10.45, 51.16]} zoom={6} />);
    const first = latestInstance();
    act(() => {
      first.fire("load");
    });
    first.jumpTo({ lng: 13.4, lat: 52.52 }, 14);

    act(() => {
      useMapStore.getState().setBasemap("dark");
    });

    const second = latestInstance();
    expect(second).not.toBe(first);
    expect(second.options.center).toEqual([13.4, 52.52]);
    expect(second.options.zoom).toBe(14);
  });

  it("falls back to the offline style when the basemap tiles fail", async () => {
    const { MapView, useMapStore } = await loadMapModules();
    render(<MapView />);
    const first = latestInstance();
    act(() => {
      first.fire("load");
    });
    expect(first.options.style.sources).toHaveProperty("osm");
    first.jumpTo({ lng: 9.99, lat: 53.55 }, 11);

    // What maplibre emits when a tile request fails: one `error` event that
    // names the source it came from.
    act(() => {
      first.fire("error", {
        error: new Error("Failed to fetch"),
        sourceId: "osm",
      });
    });

    expect(useMapStore.getState().tilesFailed).toBe(true);
    // A new Map instance, not `setStyle`: that is what makes ModelLayers
    // re-add itself, and it is why the viewport has to be carried over.
    expect(state.constructCalls).toBe(2);
    const offline = latestInstance();
    expect(offline.options.style.name).toBe("offline-fallback");
    expect(offline.options.style.sources).toEqual({});
    expect(offline.options.center).toEqual([9.99, 53.55]);
    expect(offline.options.zoom).toBe(11);

    // A failed tile is not a failed map: the canvas stays, and the "Map
    // unavailable" panel — which would unmount it — stays away.
    expect(
      screen.queryByText(m.label_map_unavailable()),
    ).not.toBeInTheDocument();
    const notice = screen.getByRole("status");
    expect(document.body.contains(offline.container)).toBe(true);

    // Retry puts the chosen basemap back.
    act(() => {
      offline.fire("load");
    });
    fireEvent.click(within(notice).getByRole("button"));

    expect(useMapStore.getState().tilesFailed).toBe(false);
    expect(state.constructCalls).toBe(3);
    expect(latestInstance().options.style.sources).toHaveProperty("osm");
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("ignores an error that did not come from the basemap source", async () => {
    // Every source failure arrives on the same event. A model source that
    // cannot be parsed is a model problem, and swapping in a blank basemap
    // would hide it behind a notice about tiles.
    const { MapView, useMapStore } = await loadMapModules();
    render(<MapView />);
    const instance = latestInstance();
    act(() => {
      instance.fire("load");
    });

    act(() => {
      instance.fire("error", {
        error: new Error("Unimplemented type: 7"),
        sourceId: "model-buildings",
      });
      instance.fire("error", { error: new Error("style is not done loading") });
    });

    expect(useMapStore.getState().tilesFailed).toBe(false);
    expect(state.constructCalls).toBe(1);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("clears the timer and removes the map on unmount", async () => {
    const MapView = await loadMapView();
    const setTimeout = vi.spyOn(window, "setTimeout");
    const clearTimeout = vi.spyOn(window, "clearTimeout");
    const { unmount } = render(<MapView />);
    const instance = latestInstance();
    // React schedules timers of its own, so pick out ours by its delay and
    // assert that exactly that handle is cleared.
    const armed = setTimeout.mock.calls.findIndex(
      ([, delay]) => delay === MAP_LOAD_TIMEOUT_MS,
    );
    expect(armed).toBeGreaterThanOrEqual(0);
    const handle: unknown = setTimeout.mock.results[armed]?.value;

    unmount();

    expect(clearTimeout).toHaveBeenCalledWith(handle);
    expect(instance.remove).toHaveBeenCalledTimes(1);

    // The timer is really gone: nothing fires after the timeout would have.
    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });
    expect(vi.getTimerCount()).toBe(0);
  });
});

/**
 * A loaded map holding the given layers, with the given handlers bound.
 *
 * Shared by both suites below: a click is one mechanism, and the model half
 * and the result half only differ in which layer the hit came from.
 */
async function loadedMap(
  handlers: {
    onFeatureClick?: (features: unknown[]) => void;
    onResultReceiverClick?: (features: unknown[]) => void;
  },
  layers: Record<string, { properties: unknown }[]>,
) {
  const MapView = await loadMapView();
  render(<MapView {...handlers} />);
  const instance = latestInstance();
  for (const [id, features] of Object.entries(layers)) {
    instance.withLayer(id, features);
  }
  // The click and hover effects read the map through the ref the `load`
  // handler fills, so nothing is bound until the map has loaded.
  act(() => {
    instance.fire("load");
  });
  return instance;
}

const POINTER = { point: { x: 10, y: 10 } };

describe("MapView result receivers", () => {
  /*
   * A receiver drawn by `ResultLayers` is a click target like any model
   * feature, and it must not arrive through the same callback.
   *
   * The precedence rule — a click selects the model object if there is one and
   * goes to the results table otherwise — is expressed by *which* callback
   * fires, not by the page sniffing an id afterwards. That is the whole point
   * of the second prop: `auto-grid` receivers are named by the run and are in
   * no model store, so a page told only "a feature was clicked" would have to
   * guess which population the id belonged to.
   */

  it("reports a result receiver through its own callback", async () => {
    const onFeatureClick = vi.fn();
    const onResultReceiverClick = vi.fn();
    const map = await loadedMap(
      { onFeatureClick, onResultReceiverClick },
      { [LAYER_IDS.resultReceiverLevel]: [hit("grid-000042")] },
    );

    act(() => {
      map.fire("click", POINTER);
    });

    expect(onResultReceiverClick).toHaveBeenCalledTimes(1);
    expect(onResultReceiverClick.mock.calls[0]?.[0]).toEqual([
      hit("grid-000042"),
    ]);
    // The editor path is untouched: nothing in the model was clicked.
    expect(onFeatureClick).not.toHaveBeenCalled();
  });

  it("gives a model feature under the pointer precedence over a circle", async () => {
    // A receiver the user placed and the run's own circle for it are under the
    // same pixel. The model object wins: the editor is the only thing that can
    // change it, and it is the editor that offers the way on to the table.
    const onFeatureClick = vi.fn();
    const onResultReceiverClick = vi.fn();
    const map = await loadedMap(
      { onFeatureClick, onResultReceiverClick },
      {
        [LAYER_IDS.receiversPoint]: [hit("rcv-1")],
        [LAYER_IDS.resultReceiverLevel]: [hit("rcv-1")],
      },
    );

    act(() => {
      map.fire("click", POINTER);
    });

    expect(onFeatureClick).toHaveBeenCalledTimes(1);
    expect(onFeatureClick.mock.calls[0]?.[0]).toEqual([hit("rcv-1")]);
    expect(onResultReceiverClick).not.toHaveBeenCalled();
  });

  it("shows the pointer cursor over a result receiver", async () => {
    // The cursor comes off the same layer list the click does. A circle that
    // can be clicked and does not say so is a control nobody finds.
    const map = await loadedMap(
      {},
      { [LAYER_IDS.resultReceiverLevel]: [hit("grid-000042")] },
    );

    act(() => {
      map.fire("mousemove", POINTER);
    });
    expect(map.getCanvas().style.cursor).toBe("pointer");

    map.rendered.set(LAYER_IDS.resultReceiverLevel, []);
    act(() => {
      map.fire("mousemove", POINTER);
    });
    expect(map.getCanvas().style.cursor).toBe("");
  });

  it("does nothing on a circle when no result handler was given", async () => {
    // `pages/map.tsx` passes both, but nothing forces a caller to, and a map
    // without the second handler must not route a circle into the first.
    const onFeatureClick = vi.fn();
    const map = await loadedMap(
      { onFeatureClick },
      { [LAYER_IDS.resultReceiverLevel]: [hit("grid-000042")] },
    );

    act(() => {
      map.fire("click", POINTER);
    });

    expect(onFeatureClick).not.toHaveBeenCalled();
  });
});

describe("MapView model feature selection", () => {
  /*
   * A ground zone is drawn from its own source, below everything else, and it
   * is still a model object with a property to edit. It reaches the editor
   * only if its layers are in the list the click query filters by — being
   * rendered is not the same as being selectable, and the feature list is not
   * the path the map's own editing flow takes.
   */

  it("reports a ground zone through the feature callback", async () => {
    const onFeatureClick = vi.fn();
    const map = await loadedMap(
      { onFeatureClick },
      { [LAYER_IDS.groundZoneFill]: [hit("zone-1")] },
    );

    act(() => {
      map.fire("click", POINTER);
    });

    expect(onFeatureClick).toHaveBeenCalledTimes(1);
    expect(onFeatureClick.mock.calls[0]?.[0]).toEqual([hit("zone-1")]);
  });

  it("gives a source standing on a zone precedence over the zone", async () => {
    // The caller opens the editor on the first hit, so the object the reader
    // aimed at has to come first. A zone covers whole hectares; a source on it
    // that could not be clicked would be unreachable on the map.
    const onFeatureClick = vi.fn();
    const map = await loadedMap(
      { onFeatureClick },
      {
        [LAYER_IDS.sourcesPoint]: [hit("src-1")],
        [LAYER_IDS.groundZoneFill]: [hit("zone-1")],
      },
    );

    act(() => {
      map.fire("click", POINTER);
    });

    expect(onFeatureClick.mock.calls[0]?.[0]).toEqual([
      hit("src-1"),
      hit("zone-1"),
    ]);
  });

  it("shows the pointer cursor over a ground zone", async () => {
    const map = await loadedMap(
      {},
      { [LAYER_IDS.groundZoneOutline]: [hit("zone-1")] },
    );

    act(() => {
      map.fire("mousemove", POINTER);
    });
    expect(map.getCanvas().style.cursor).toBe("pointer");
  });
});
