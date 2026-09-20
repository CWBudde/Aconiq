import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { MapContext } from "./use-map";
import { FeatureFocus, type FocusRequest } from "./feature-focus";
import type { DisplayModel } from "./display-model";
import { LAYER_IDS, SOURCE_IDS } from "./layers";

/**
 * The MapLibre surface `FeatureFocus` touches, and only that.
 *
 * Deliberately not shared with `model-layers.test.tsx`'s fake: coverage counts
 * every non-test file under `src/`, so a `fake-map.ts` helper would read as
 * untested source, and the two doubles model different surfaces anyway — that
 * one needs layer visibility, this one needs paint properties.
 *
 * `fitBounds` throws the way MapLibre throws: a latitude outside ±90 is an
 * exception out of the caller's effect, not a silently clamped view.
 */
class FakeMap {
  readonly sources = new Map<string, { data: unknown }>();
  readonly layers: string[] = [];
  readonly fitBoundsCalls: {
    bounds: [[number, number], [number, number]];
    options: { padding?: number; maxZoom?: number; duration?: number };
  }[] = [];
  /** Paint values by `${layerId}/${property}`, the way MapLibre keys them. */
  readonly paint = new Map<string, unknown>();
  callCount = 0;

  getSource(id: string) {
    this.callCount += 1;
    const source = this.sources.get(id);
    if (!source) return undefined;
    return {
      setData: (data: unknown) => {
        source.data = data;
      },
    };
  }

  addSource(id: string, source: { data: unknown }) {
    this.callCount += 1;
    this.sources.set(id, { data: source.data });
  }

  getLayer(id: string) {
    this.callCount += 1;
    return this.layers.includes(id) ? { id } : undefined;
  }

  addLayer(layer: { id: string }) {
    this.callCount += 1;
    this.layers.push(layer.id);
  }

  setPaintProperty(id: string, property: string, value: unknown) {
    this.callCount += 1;
    this.paint.set(`${id}/${property}`, value);
  }

  fitBounds(
    bounds: [[number, number], [number, number]],
    options: { padding?: number; maxZoom?: number; duration?: number },
  ) {
    this.callCount += 1;
    const [[, south], [, north]] = bounds;
    if (Math.abs(south) > 90 || Math.abs(north) > 90) {
      throw new Error("Invalid LngLat latitude value");
    }
    this.fitBoundsCalls.push({ bounds, options });
  }
}

const road: ModelFeature = {
  id: "road-1",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [9.7, 52.35],
      [9.75, 52.38],
    ],
  },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [9.72, 52.36] },
};

/** A road whose "4326" coordinates are in fact UTM metres. */
const metricRoad: ModelFeature = {
  id: "metric-1",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [548000, 5806000],
      [548100, 5806100],
    ],
  },
};

function ready(features: ModelFeature[] = [road]): DisplayModel {
  return {
    status: "ready",
    features,
    receivers: [receiver],
    calcArea: null as CalcArea | null,
    sourceCRS: "EPSG:4326",
    reprojected: false,
  };
}

const projecting: DisplayModel = {
  status: "projecting",
  sourceCRS: "EPSG:25832",
};

function renderFocus(
  map: FakeMap,
  display: DisplayModel,
  request: FocusRequest | null,
) {
  return render(
    <MapContext value={map as unknown as MapLibreMap}>
      <FeatureFocus display={display} request={request} />
    </MapContext>,
  );
}

let warn: ReturnType<typeof vi.spyOn>;
let error: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  vi.useFakeTimers();
  warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
  error = vi.spyOn(console, "error").mockImplementation(() => undefined);
});

afterEach(() => {
  vi.useRealTimers();
  warn.mockRestore();
  error.mockRestore();
});

describe("FeatureFocus camera", () => {
  it("fits the one feature, not the workspace", () => {
    const map = new FakeMap();
    renderFocus(map, ready(), { featureId: "road-1", epoch: 1 });

    expect(map.fitBoundsCalls).toHaveLength(1);
    expect(map.fitBoundsCalls[0]?.bounds).toEqual([
      [9.7, 52.35],
      [9.75, 52.38],
    ]);
  });

  it("caps the zoom, so a receiver does not fill the screen with basemap", () => {
    // A point's extent has no area. Without a ceiling `fitBounds` resolves
    // that to the map's maximum zoom.
    const map = new FakeMap();
    renderFocus(map, ready(), { featureId: "rcv-1", epoch: 1 });

    const call = map.fitBoundsCalls[0];
    expect(call?.options.maxZoom).toBe(18);
    const [[west, south], [east, north]] = call?.bounds ?? [
      [0, 0],
      [0, 0],
    ];
    expect(east - west).toBeGreaterThan(0);
    expect(north - south).toBeGreaterThan(0);
  });

  it("moves again when the same feature is asked for twice", () => {
    // The stepper can land back on a feature the editor already has open, and
    // the reader has panned away since.
    const map = new FakeMap();
    const { rerender } = renderFocus(map, ready(), {
      featureId: "road-1",
      epoch: 1,
    });
    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <FeatureFocus
          display={ready()}
          request={{ featureId: "road-1", epoch: 2 }}
        />
      </MapContext>,
    );

    expect(map.fitBoundsCalls).toHaveLength(2);
  });

  it("serves a request that arrived while the model was still projecting", () => {
    // The `?select=` deep link into a metric project always hits this: the
    // feature genuinely is not on the map yet when the request is made.
    const map = new FakeMap();
    const request: FocusRequest = { featureId: "road-1", epoch: 1 };
    const { rerender } = renderFocus(map, projecting, request);
    expect(map.fitBoundsCalls).toHaveLength(0);

    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <FeatureFocus display={ready()} request={request} />
      </MapContext>,
    );

    expect(map.fitBoundsCalls).toHaveLength(1);
  });

  it("gives up on an id the ready model does not hold", () => {
    const map = new FakeMap();
    renderFocus(map, ready(), { featureId: "ghost", epoch: 1 });

    expect(map.fitBoundsCalls).toHaveLength(0);
    expect(warn).toHaveBeenCalled();
  });

  it("refuses an extent that is not lon/lat instead of throwing", () => {
    const map = new FakeMap();
    renderFocus(map, ready([metricRoad]), {
      featureId: "metric-1",
      epoch: 1,
    });

    expect(map.fitBoundsCalls).toHaveLength(0);
    expect(warn).toHaveBeenCalled();
  });
});

describe("FeatureFocus flash", () => {
  const lineOpacity = `${LAYER_IDS.flashLine}/line-opacity`;
  const lineTransition = `${LAYER_IDS.flashLine}/line-opacity-transition`;

  it("adds its source and layers once, above everything else", () => {
    const map = new FakeMap();
    const { rerender } = renderFocus(map, ready(), {
      featureId: "road-1",
      epoch: 1,
    });
    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <FeatureFocus
          display={ready()}
          request={{ featureId: "road-1", epoch: 2 }}
        />
      </MapContext>,
    );

    expect(map.layers).toEqual([LAYER_IDS.flashLine, LAYER_IDS.flashPoint]);
    expect(map.sources.has(SOURCE_IDS.flash)).toBe(true);
  });

  it("shows the halo at full strength, then fades it, then clears it", () => {
    const map = new FakeMap();
    renderFocus(map, ready(), { featureId: "road-1", epoch: 1 });

    const data = map.sources.get(SOURCE_IDS.flash)
      ?.data as GeoJSON.FeatureCollection;
    expect(data.features).toHaveLength(1);
    expect(map.paint.get(lineOpacity)).toBe(0.75);
    // Phase 1 must not animate, or the halo fades *in* from nothing.
    expect(map.paint.get(lineTransition)).toEqual({ duration: 0, delay: 0 });

    // Phase 2 is deferred a tick on purpose: two paint writes in one frame are
    // coalesced, and the transition would run from 0 to 0.
    vi.advanceTimersByTime(0);
    expect(map.paint.get(lineOpacity)).toBe(0);
    expect(map.paint.get(lineTransition)).toEqual({
      duration: 600,
      delay: 1500,
    });

    vi.advanceTimersByTime(5000);
    const cleared = map.sources.get(SOURCE_IDS.flash)
      ?.data as GeoJSON.FeatureCollection;
    expect(cleared.features).toHaveLength(0);
  });

  it("does not let a superseded flash clear the one that replaced it", () => {
    const map = new FakeMap();
    const { rerender } = renderFocus(map, ready(), {
      featureId: "road-1",
      epoch: 1,
    });
    vi.advanceTimersByTime(800);

    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <FeatureFocus
          display={ready()}
          request={{ featureId: "rcv-1", epoch: 2 }}
        />
      </MapContext>,
    );
    // Past the first flash's own clear deadline, but not the second's.
    vi.advanceTimersByTime(1400);

    const data = map.sources.get(SOURCE_IDS.flash)
      ?.data as GeoJSON.FeatureCollection;
    expect(data.features).toHaveLength(1);
  });

  it("touches nothing after a second flash is unmounted", () => {
    // The regression this file missed the first time: `flash()` used to
    // replace the timer array, while the cleanup held the one captured when
    // its effect ran. Flash 1 was covered by accident — the focus effect runs
    // before the cleanup effect registers, so the array it captured was flash
    // 1's. Every flash after that leaked its timers past unmount.
    const map = new FakeMap();
    const { rerender, unmount } = renderFocus(map, ready(), {
      featureId: "road-1",
      epoch: 1,
    });
    rerender(
      <MapContext value={map as unknown as MapLibreMap}>
        <FeatureFocus
          display={ready()}
          request={{ featureId: "rcv-1", epoch: 2 }}
        />
      </MapContext>,
    );

    unmount();
    const settled = map.callCount;
    vi.advanceTimersByTime(10_000);

    expect(map.callCount).toBe(settled);
  });

  it("touches nothing after it is unmounted", () => {
    // A timer firing after `map.remove()` reaches a map that throws on
    // every call.
    const map = new FakeMap();
    const { unmount } = renderFocus(map, ready(), {
      featureId: "road-1",
      epoch: 1,
    });

    unmount();
    const settled = map.callCount;
    vi.advanceTimersByTime(10_000);

    expect(map.callCount).toBe(settled);
  });

  it("holds the halo still for a reader who asked for reduced motion", () => {
    vi.spyOn(window, "matchMedia").mockReturnValue({
      matches: true,
    } as MediaQueryList);

    const map = new FakeMap();
    renderFocus(map, ready(), { featureId: "road-1", epoch: 1 });

    expect(map.fitBoundsCalls[0]?.options.duration).toBe(0);
    vi.advanceTimersByTime(0);
    // No fade, and a longer dwell: a ring that appears and vanishes with no
    // transition needs it to be noticed at all.
    expect(map.paint.get(lineTransition)).toEqual({
      duration: 0,
      delay: 1500,
    });
  });
});
