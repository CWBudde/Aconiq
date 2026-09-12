import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";

// maplibre-gl cannot run in jsdom (no WebGL), so the whole module is replaced
// by a fake whose `Map` records instances and can be told to throw. The fake
// lives in `vi.hoisted` so the same class survives `vi.resetModules()` below.
const fake = vi.hoisted(() => {
  type Handler = () => void;

  class FakeMap {
    private readonly handlers = new Map<string, Set<Handler>>();
    private readonly canvas = document.createElement("canvas");
    readonly addControl = vi.fn();
    readonly resize = vi.fn();
    readonly remove = vi.fn();
    readonly getLayer = vi.fn(() => undefined);
    readonly queryRenderedFeatures = vi.fn(() => []);

    constructor() {
      state.constructCalls += 1;
      if (state.throwOnConstruct) {
        throw new Error("WebGL is not supported");
      }
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

    /** Invoke every handler registered for `event`, as maplibre would. */
    fire(event: string): void {
      for (const handler of this.handlers.get(event) ?? []) {
        handler();
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
async function loadMapView() {
  const mod = await import("./map-view");
  vi.useFakeTimers();
  return mod.MapView;
}

function latestInstance() {
  const instance = state.instances.at(-1);
  if (!instance) throw new Error("no map instance was created");
  return instance;
}

const MAP_LOAD_TIMEOUT_MS = 15000;

// The first import pays for transforming the module graph and can exceed the
// per-test timeout on a cold cache; later re-imports after `resetModules` only
// re-evaluate.
beforeAll(async () => {
  await import("./map-view");
}, 60000);

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

    expect(screen.queryByText("Map unavailable")).not.toBeInTheDocument();
    expect(container.querySelector("div.absolute.inset-0")).not.toBeNull();
    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });
    expect(screen.queryByText("Map unavailable")).not.toBeInTheDocument();
  });

  it("shows the timeout panel without load and recovers on retry", async () => {
    const MapView = await loadMapView();
    render(<MapView />);
    const first = latestInstance();
    const removeListener = vi.spyOn(first.getCanvas(), "removeEventListener");

    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });

    expect(screen.getByText("Map unavailable")).toBeInTheDocument();
    expect(screen.getByText(/did not finish loading/)).toBeInTheDocument();
    // The error re-runs the init effect, whose cleanup tears the stale map
    // down completely.
    expect(first.remove).toHaveBeenCalledTimes(1);
    expect(removeListener).toHaveBeenCalledWith(
      "webglcontextlost",
      expect.any(Function),
    );
    expect(removeListener).toHaveBeenCalledWith(
      "webglcontextrestored",
      expect.any(Function),
    );

    fireEvent.click(screen.getByRole("button", { name: "Retry" }));

    expect(state.constructCalls).toBe(2);
    act(() => {
      latestInstance().fire("load");
    });
    expect(screen.queryByText("Map unavailable")).not.toBeInTheDocument();
  });

  it("shows the unavailable panel when the constructor throws and retries", async () => {
    state.throwOnConstruct = true;
    const MapView = await loadMapView();
    render(<MapView />);

    expect(state.constructCalls).toBe(1);
    expect(screen.getByText("Map unavailable")).toBeInTheDocument();
    expect(screen.getByText(/rendering is unavailable/)).toBeInTheDocument();

    // Still broken: the panel persists and the button stays available.
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.constructCalls).toBe(2);
    expect(screen.getByText("Map unavailable")).toBeInTheDocument();

    // Fixed in the meantime: the retry clears the session switch, so the map
    // is created and rendered.
    state.throwOnConstruct = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.constructCalls).toBe(3);
    act(() => {
      latestInstance().fire("load");
    });
    expect(screen.queryByText("Map unavailable")).not.toBeInTheDocument();
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
    expect(screen.getByText("Map unavailable")).toBeInTheDocument();
  });

  it("survives a lost WebGL context and resizes on restore", async () => {
    const MapView = await loadMapView();
    render(<MapView />);
    const instance = latestInstance();
    act(() => {
      instance.fire("load");
    });

    const lost = new Event("webglcontextlost", { cancelable: true });
    act(() => {
      instance.getCanvas().dispatchEvent(lost);
    });

    expect(lost.defaultPrevented).toBe(true);
    expect(screen.queryByText("Map unavailable")).not.toBeInTheDocument();
    expect(instance.remove).not.toHaveBeenCalled();

    act(() => {
      instance.getCanvas().dispatchEvent(new Event("webglcontextrestored"));
    });
    expect(instance.resize).toHaveBeenCalledTimes(1);
  });

  it("clears the timer and removes the canvas listeners on unmount", async () => {
    const MapView = await loadMapView();
    const { unmount } = render(<MapView />);
    const instance = latestInstance();
    const removeListener = vi.spyOn(
      instance.getCanvas(),
      "removeEventListener",
    );
    const clearTimeout = vi.spyOn(window, "clearTimeout");

    unmount();

    expect(clearTimeout).toHaveBeenCalledTimes(1);
    expect(removeListener).toHaveBeenCalledWith(
      "webglcontextlost",
      expect.any(Function),
    );
    expect(removeListener).toHaveBeenCalledWith(
      "webglcontextrestored",
      expect.any(Function),
    );
    expect(instance.remove).toHaveBeenCalledTimes(1);

    // The timer is really gone: nothing fires after the timeout would have.
    act(() => {
      vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
    });
    expect(vi.getTimerCount()).toBe(0);
  });
});
