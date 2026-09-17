import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import type { Map as MapLibreMap } from "maplibre-gl";
import { CoordinateDisplay } from "./coordinate-display";
import { MapContext } from "./use-map";
import { getLocale, overwriteGetLocale, type Locale } from "@/i18n/runtime";

/**
 * The readout is a plain panel fed by one MapLibre event, so nothing here
 * needs WebGL — the event is delivered directly to the handler the component
 * registered. What it is worth pinning is the order of the two numbers and
 * that the listener is taken back off on unmount.
 */

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

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
});

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
});
