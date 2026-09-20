import { useEffect, useRef } from "react";
import type maplibregl from "maplibre-gl";
import { useMap } from "./use-map";
import type { DisplayModel } from "./display-model";
import { type Bounds, computeGeometryBounds, toLngLatBounds } from "./extent";
import { prefersReducedMotion } from "./motion";
import {
  FLASH_FADE_MS,
  FLASH_HOLD_MS,
  FLASH_HOLD_REDUCED_MS,
  FLASH_LAYERS,
  FLASH_OPACITY,
  FLASH_OPACITY_PROPERTIES,
  SOURCE_IDS,
} from "./layers";

/**
 * Room around the focused feature, in pixels. More than the workspace fit's 48:
 * one feature wants air, and the docked editor is about to open over the left
 * `w-80` of the canvas.
 */
const FOCUS_PADDING = 64;
/**
 * A point's extent is a box of zero area, and `fitBounds` would resolve that to
 * the map's maximum zoom — a receiver filling the screen with basemap. This is
 * the ceiling that makes a point land at a readable scale.
 */
const FOCUS_MAX_ZOOM = 18;
const FOCUS_DURATION_MS = 600;
/** ~11 m. Below this an extent is treated as a point and padded out. */
const MIN_FOCUS_SPAN_DEG = 1e-4;

const EMPTY_COLLECTION: GeoJSON.FeatureCollection = {
  type: "FeatureCollection",
  features: [],
};

/**
 * A request to take the reader to a feature.
 *
 * `epoch` is what distinguishes two requests for the same id — asking for the
 * feature already open has to move the camera again, the same reason
 * `selectionEpoch` exists for the geometry editor.
 */
export interface FocusRequest {
  featureId: string;
  epoch: number;
}

export interface FeatureFocusProps {
  /** The projected model. Never the store: MapLibre takes lon/lat only. */
  display: DisplayModel;
  request: FocusRequest | null;
}

/**
 * Moves the camera to a requested feature and flashes a halo around it.
 *
 * Rendered inside `<MapView>` so `useMap()` resolves, and *after* `ModelLayers`
 * so that a `?select=` deep link arriving on the first paint of a project wins
 * over that component's one-shot fit to the whole workspace.
 *
 * It is separate from the selection highlight, which `ModelLayers` owns because
 * only that component knows which GeoJSON source an id was drawn from. A camera
 * move needs the geometry and not the source, so it does not belong there — and
 * the two are genuinely different events: every focus selects, but a click on
 * the canvas selects without focusing.
 */
export function FeatureFocus({ display, request }: FeatureFocusProps): null {
  const map = useMap();
  const servedRef = useRef<FocusRequest | null>(null);
  const timersRef = useRef<number[]>([]);

  useEffect(() => {
    if (!map || !request) return;
    if (servedRef.current === request) return;
    // Not ready yet: leave the request unserved rather than giving up on it.
    // `display` is a dependency, so this effect runs again when the projection
    // lands — which is the case a deep link into a metric project always hits.
    if (display.status !== "ready") return;

    servedRef.current = request;

    const geometry = focusGeometry(request.featureId, display);
    if (!geometry) {
      // Ready, and the id still names nothing drawn: the feature does not
      // exist. Served above, so this is not retried on every reprojection.
      console.warn(
        `FeatureFocus: nothing on the map is called "${request.featureId}"`,
      );
      return;
    }

    const bounds = computeGeometryBounds(geometry.coordinates);
    if (!bounds) return;

    const lngLat = toLngLatBounds(padDegenerate(bounds));
    if (!lngLat) {
      console.warn(
        "FeatureFocus: the feature's extent is not lon/lat, so the view was not moved",
        bounds,
      );
      return;
    }

    const reduced = prefersReducedMotion();
    const duration = reduced ? 0 : FOCUS_DURATION_MS;

    try {
      map.fitBounds(lngLat, {
        padding: FOCUS_PADDING,
        maxZoom: FOCUS_MAX_ZOOM,
        duration,
      });
    } catch (error) {
      // MapLibre answers an impossible latitude with a throw, and it must not
      // take this effect's caller down with it — the same lesson `fitToWorkspace`
      // learned when its call sat outside the try blocks around it.
      console.error("FeatureFocus: could not move the view", error);
    }

    flash(map, geometry, timersRef, duration, reduced);
  }, [map, request, display]);

  // Covers unmount and the map being rebuilt under a basemap switch: a timer
  // that fires afterwards would reach a map that throws on every call.
  //
  // Capturing the array here is only safe because `flash` **truncates it and
  // never replaces it**. It used to assign a fresh one per request, which left
  // this cleanup holding the first flash's array while the live timers sat in
  // the newest — so every flash after the first ran on past unmount. A single
  // flash could not show it, which is why the regression test in
  // `feature-focus.test.tsx` fires two.
  useEffect(() => {
    const timers = timersRef.current;
    return () => {
      clearFlashTimers(timers);
    };
  }, [map]);

  return null;
}

/** The geometry behind an id, looked up the way `selectionTarget` looks up its source. */
function focusGeometry(
  featureId: string,
  display: DisplayModel,
): { type: string; coordinates: unknown } | null {
  if (display.status !== "ready") return null;

  const feature = display.features.find((f) => f.id === featureId);
  if (feature) return feature.geometry;

  const receiver = display.receivers.find((r) => r.id === featureId);
  if (receiver) return receiver.geometry;

  return null;
}

/**
 * Grows an extent that is a point, or a straight north–south line, into a box.
 *
 * Belt to {@link FOCUS_MAX_ZOOM}'s braces: a zero span is a division by zero in
 * MapLibre's scale computation, and a receiver is always one.
 */
function padDegenerate(bounds: Bounds): Bounds {
  const padded = { ...bounds };
  if (padded.east - padded.west < MIN_FOCUS_SPAN_DEG) {
    const centre = (padded.west + padded.east) / 2;
    padded.west = centre - MIN_FOCUS_SPAN_DEG / 2;
    padded.east = centre + MIN_FOCUS_SPAN_DEG / 2;
  }
  if (padded.north - padded.south < MIN_FOCUS_SPAN_DEG) {
    const centre = (padded.south + padded.north) / 2;
    padded.south = centre - MIN_FOCUS_SPAN_DEG / 2;
    padded.north = centre + MIN_FOCUS_SPAN_DEG / 2;
  }
  return padded;
}

/**
 * Puts a halo on the focused feature, holds it, and fades it out.
 *
 * Three phases, and the middle one has to be deferred: two paint writes in one
 * frame are coalesced, so setting the opacity full and then zero synchronously
 * would transition from 0 to 0 and show nothing. A `setTimeout(…, 0)` rather
 * than `requestAnimationFrame` because a test can advance it.
 *
 * The fade's `delay` carries the camera flight as well as the hold, which is
 * why there is no third timer and no `moveend` listener: the halo stays at full
 * strength for the whole flight — the target visibly approaching — and starts
 * fading once the reader has arrived.
 */
function flash(
  map: maplibregl.Map,
  geometry: { type: string; coordinates: unknown },
  timersRef: { current: number[] },
  cameraDuration: number,
  reduced: boolean,
): void {
  // A superseded flash must not let its own cleanup wipe the one that replaced
  // it: stepping the queue quickly is the ordinary case, not an edge one.
  // Truncated, never reassigned — the unmount cleanup has to be able to find
  // these timers, and a fresh array here would leave it holding the old one.
  clearFlashTimers(timersRef.current);

  if (!ensureFlashLayers(map)) return;

  const hold = reduced ? FLASH_HOLD_REDUCED_MS : FLASH_HOLD_MS;
  const fade = reduced ? 0 : FLASH_FADE_MS;

  const shown = withMap("show the focus halo", () => {
    setFlashData(map, {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: {},
          geometry: geometry as unknown as GeoJSON.Geometry,
        },
      ],
    });
    for (const [layerId, property] of FLASH_OPACITY_PROPERTIES) {
      map.setPaintProperty(layerId, `${property}-transition`, {
        duration: 0,
        delay: 0,
      });
      map.setPaintProperty(layerId, property, FLASH_OPACITY);
    }
  });
  if (!shown) return;

  timersRef.current.push(
    window.setTimeout(() => {
      withMap("fade the focus halo", () => {
        for (const [layerId, property] of FLASH_OPACITY_PROPERTIES) {
          map.setPaintProperty(layerId, `${property}-transition`, {
            duration: fade,
            delay: cameraDuration + hold,
          });
          map.setPaintProperty(layerId, property, 0);
        }
      });
    }, 0),
    window.setTimeout(
      () => {
        withMap("clear the focus halo", () => {
          setFlashData(map, EMPTY_COLLECTION);
        });
      },
      cameraDuration + hold + fade + 50,
    ),
  );
}

/** Cancels every pending flash timer, emptying the array in place. */
function clearFlashTimers(timers: number[]): void {
  for (const id of timers) clearTimeout(id);
  timers.length = 0;
}

/** Adds the flash source and layers if this map does not have them yet. */
function ensureFlashLayers(map: maplibregl.Map): boolean {
  return withMap("add the focus halo layers", () => {
    if (!map.getSource(SOURCE_IDS.flash)) {
      map.addSource(SOURCE_IDS.flash, {
        type: "geojson",
        data: EMPTY_COLLECTION as unknown as GeoJSON.GeoJSON,
      });
    }
    // Added last, so the halo is above every model and result layer. A basemap
    // switch rebuilds the map and drops these; the next request re-adds them.
    for (const layer of FLASH_LAYERS) {
      if (!map.getLayer(layer.id)) map.addLayer(layer);
    }
  });
}

function setFlashData(
  map: maplibregl.Map,
  data: GeoJSON.FeatureCollection,
): void {
  const source = map.getSource(SOURCE_IDS.flash);
  if (source && "setData" in source) {
    (source as maplibregl.GeoJSONSource).setData(
      data as unknown as GeoJSON.GeoJSON,
    );
  }
}

/** Runs a block of map calls, reporting rather than throwing. */
function withMap(what: string, run: () => void): boolean {
  try {
    run();
    return true;
  } catch (error) {
    console.error(`FeatureFocus: could not ${what}`, error);
    return false;
  }
}
