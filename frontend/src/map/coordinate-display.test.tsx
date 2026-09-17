import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { CoordinateDisplay } from "./coordinate-display";
import { MapContext } from "./use-map";
import { useModelStore } from "@/model/model-store";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { getLocale, overwriteGetLocale, type Locale } from "@/i18n/runtime";

/**
 * The readout is a plain panel fed by one MapLibre event, so nothing here
 * needs WebGL — the event is delivered directly to the handler the component
 * registered. What it is worth pinning is the order of the two numbers, the
 * second line the panel owes a project stored in metres, and that the listener
 * is taken back off on unmount.
 */

const projection = vi.hoisted(() => {
  /** Multiplies by 100 000, so a projected coordinate is recognisable on sight. */
  const scaled = (req: TransformRequest): Promise<TransformResponse> =>
    Promise.resolve({
      source_crs: req.source_crs,
      target_crs: req.target_crs,
      applied: true,
      coordinates: req.coordinates.map((value) => value * 100000),
    });

  const value: {
    canReprojectForDisplay: boolean;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
    readonly scaled: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    requests: [],
    respond: scaled,
    scaled,
  };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: "browser",
        canExport: false,
        runsAgainstSavedModel: false,
        runsChangeExternally: false,
        exportsOutliveRunDelete: false,
        canReprojectForDisplay: projection.canReprojectForDisplay,
      };
    },
    transformCoordinates: (req: TransformRequest) => {
      projection.requests.push(req);
      return projection.respond(req);
    },
  },
}));

type MoveHandler = (event: { lngLat: { lng: number; lat: number } }) => void;

/** The `on`/`off` surface `CoordinateDisplay` uses, and only that. */
class FakeMap {
  readonly handlers = new Map<string, Set<MoveHandler>>();

  on(event: string, handler: MoveHandler) {
    const set = this.handlers.get(event) ?? new Set<MoveHandler>();
    set.add(handler);
    this.handlers.set(event, set);
  }

  off(event: string, handler: MoveHandler) {
    this.handlers.get(event)?.delete(handler);
  }

  moveTo(lng: number, lat: number) {
    act(() => {
      for (const handler of this.handlers.get("mousemove") ?? []) {
        handler({ lngLat: { lng, lat } });
      }
    });
  }

  listenerCount(event: string): number {
    return this.handlers.get(event)?.size ?? 0;
  }
}

function renderDisplay(map: FakeMap | null) {
  return render(
    <MapContext value={map as unknown as MapLibreMap | null}>
      <CoordinateDisplay />
    </MapContext>,
  );
}

const originalGetLocale = getLocale;

function useLocale(locale: Locale) {
  overwriteGetLocale(() => locale);
}

beforeEach(() => {
  useModelStore.getState().reset();
  projection.canReprojectForDisplay = true;
  projection.requests = [];
  projection.respond = projection.scaled;
});

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
});

/** Puts the store in a metric CRS without putting anything in it. */
function storeMetric() {
  act(() => {
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
  });
}

describe("CoordinateDisplay", () => {
  it("shows nothing until the pointer has been over the map", () => {
    // There is no meaningful coordinate before the first move, and a panel
    // reading "0.000000, 0.000000" over Germany would be a lie.
    const map = new FakeMap();
    const { container } = renderDisplay(map);

    expect(container).toBeEmptyDOMElement();
  });

  it("reads latitude first, then longitude", () => {
    // The panel is unlabelled — two bare numbers — so a swapped pair is
    // invisible on screen and only wrong. MapLibre's event carries `lngLat`,
    // the display convention is the other way round, and that crossing is the
    // whole of this component's logic.
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    expect(screen.getByText("51.250000, 10.500000")).toBeInTheDocument();
  });

  it("holds six decimals, which is about 0.1 m of ground", () => {
    // Fewer digits round a receiver off its building. The value is truncated
    // for display only; nothing downstream reads this panel.
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.123456789, 51.987654321);

    expect(screen.getByText("51.987654, 10.123457")).toBeInTheDocument();
  });

  it("follows the pointer rather than pinning the first position", () => {
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10, 51);
    map.moveTo(11, 52);

    expect(screen.getByText("52.000000, 11.000000")).toBeInTheDocument();
    expect(screen.queryByText("51.000000, 10.000000")).not.toBeInTheDocument();
  });

  it("formats in the active locale", () => {
    // The numbers go through `formatCoordinate`, so German reads a comma
    // decimal separator. Hardcoding `toFixed` here would show a point to every
    // user regardless of language.
    useLocale("de");
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    expect(screen.getByText("51,250000, 10,500000")).toBeInTheDocument();
  });

  it("takes its listener back off the map when it unmounts", () => {
    // The map outlives this component — it is remounted on every route change
    // into and out of the workspace — so a leaked handler accumulates one
    // `setState` on an unmounted tree per navigation.
    const map = new FakeMap();
    const view = renderDisplay(map);
    expect(map.listenerCount("mousemove")).toBe(1);

    view.unmount();

    expect(map.listenerCount("mousemove")).toBe(0);
  });

  it("renders nothing and registers nothing before the map exists", () => {
    // `MapView` renders its children while `map` is still null.
    const { container } = renderDisplay(null);
    expect(container).toBeEmptyDOMElement();
  });

  it("asks for nothing while the store is in the CRS the map draws in", () => {
    // WGS 84 in and WGS 84 out is a round trip through the backend for no
    // information, once per settled pointer position.
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    expect(projection.requests).toEqual([]);
  });

  it("reads the pointer out in the CRS the model is stored in", async () => {
    // The whole point of the second line: a project in EPSG:25832 is drawn in
    // WGS 84, so the map's own event answers in degrees for a model measured
    // in metres — numbers the user cannot check against their own data.
    storeMetric();
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    await waitFor(() => {
      expect(
        screen.getByText("1,050,000.00, 5,125,000.00"),
      ).toBeInTheDocument();
    });
    // Easting first, which is EPSG:25832's own axis order, and the CRS names
    // the line so the two orders on the panel cannot be confused.
    expect(projection.requests).toEqual([
      {
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        coordinates: [10.5, 51.25],
      },
    ]);
    expect(screen.getByText("EPSG:25832")).toBeInTheDocument();
    // The geographic line stays: it is still true, and it is what the basemap
    // and every WGS 84 import are in.
    expect(screen.getByText("51.250000, 10.500000")).toBeInTheDocument();
  });

  it("drops the projected pair as soon as the pointer leaves the position", async () => {
    // The two lines must describe one point. Keeping the old easting/northing
    // beside the new lon/lat while the debounce and the round trip run puts two
    // different positions on the panel as though they were one — and a slow or
    // failing transform leaves that standing.
    storeMetric();
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);
    await waitFor(() => {
      expect(
        screen.getByText("1,050,000.00, 5,125,000.00"),
      ).toBeInTheDocument();
    });

    let answer: ((response: TransformResponse) => void) | null = null;
    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });

    map.moveTo(11.5, 52.25);

    // The geographic line has already moved, so the projected one must not
    // still be answering for where the pointer was.
    expect(screen.getByText("52.250000, 11.500000")).toBeInTheDocument();
    expect(
      screen.queryByText("1,050,000.00, 5,125,000.00"),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(answer).not.toBeNull();
    });
    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: [1150000, 5225000],
      });
    });
    await waitFor(() => {
      expect(
        screen.getByText("1,150,000.00, 5,225,000.00"),
      ).toBeInTheDocument();
    });
  });

  it("projects where the pointer stopped, not every frame it crossed", async () => {
    // MapLibre fires `mousemove` per frame and in API mode each projection is
    // an HTTP round trip, so the readout debounces rather than asking sixty
    // times for a single sweep.
    storeMetric();
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.1, 51.1);
    map.moveTo(10.2, 51.2);
    map.moveTo(10.5, 51.25);

    await waitFor(() => {
      expect(projection.requests).toHaveLength(1);
    });
    expect(projection.requests[0]?.coordinates).toEqual([10.5, 51.25]);
  });

  it("keeps the geographic line when the projection fails", async () => {
    // `CRSNotice` is where a broken projection is explained; blanking the
    // readout as well would take away the one coordinate that is still true.
    storeMetric();
    projection.respond = () => Promise.reject(new Error("no zone"));
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    await waitFor(() => {
      expect(projection.requests).toHaveLength(1);
    });
    expect(screen.getByText("51.250000, 10.500000")).toBeInTheDocument();
    expect(screen.queryByText("EPSG:25832")).toBeNull();
  });

  it("asks nothing of a backend that cannot project", async () => {
    storeMetric();
    projection.canReprojectForDisplay = false;
    const map = new FakeMap();
    renderDisplay(map);

    map.moveTo(10.5, 51.25);

    await waitFor(() => {
      expect(screen.getByText("51.250000, 10.500000")).toBeInTheDocument();
    });
    expect(projection.requests).toEqual([]);
  });
});
