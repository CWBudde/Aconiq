import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import type { Map, MapMouseEvent, MapGeoJSONFeature } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { MapContext } from "./use-map";
import { BASEMAP_SOURCE_ID, basemapStyle, OFFLINE_STYLE } from "./basemap";
import { useMapStore } from "./map-store";
import { OfflineNotice } from "./offline-notice";
import { LAYER_IDS, SOURCE_IDS } from "./layers";

/**
 * How long to wait for the map's `load` event before giving up and showing the
 * "map unavailable" fallback. A WebGL context that is created but never paints
 * (blocked GPU, software rasterizer, remote session) never fires `load`.
 *
 * `load` only fires after the style *and* the first render complete, so a slow
 * network produces the same signal as a broken GPU. The threshold is therefore
 * deliberately generous, and a timeout is treated as a per-mount condition
 * rather than session-wide evidence that WebGL is unusable: a route change
 * remounts and retries, and so does the panel's Retry button. (A basemap
 * switch does not: it only re-runs the init effect, which early-returns while
 * `mapError` is set.)
 */
const MAP_LOAD_TIMEOUT_MS = 15000;

const MAP_TIMEOUT_MESSAGE =
  "The map did not finish loading. WebGL rendering may be blocked or unavailable in this browser.";

const MAP_UNAVAILABLE_MESSAGE = "Map rendering is unavailable in this browser.";

/**
 * Session-scoped kill switch for WebGL map rendering.
 *
 * Module-level (not React state and not the zustand map store) on purpose: it
 * must survive MapView unmount/remount — a basemap switch or a route change
 * recreates the component — while still resetting on a full page reload.
 *
 * Set only when `new maplibregl.Map()` throws, which is the one signal that is
 * genuinely permanent for the session: the browser cannot give us a WebGL
 * context at all, so every remount would fail identically. A load timeout is
 * not such a signal (it also fires on a slow network), and neither is
 * `webglcontextlost`, which is transient by definition — there is a
 * `webglcontextrestored` handler below that expects to recover from it.
 *
 * Cleared only by the Retry button on the unavailable panel. The user knows
 * things the code cannot (a GPU switch, a settled network, an extension turned
 * off), so the retry is theirs to make; if the constructor throws again the
 * switch is simply set again and the panel returns.
 */
let webglDisabledForSession = false;

function disableWebGLForSession(): void {
  webglDisabledForSession = true;
}

function enableWebGLForSession(): void {
  webglDisabledForSession = false;
}

/**
 * Does this MapLibre `error` event say the basemap tiles are unreachable?
 *
 * MapLibre funnels everything through one `error` event: a failed tile request,
 * a malformed style, a GeoJSON source that could not be parsed. Only the first
 * is a reason to fall back to a plain background, so the test is the source id
 * — `ModelLayers` adds its own sources under the ids in `layers.ts`, and one of
 * those failing means the *model* is broken, which the offline basemap would
 * hide rather than help.
 *
 * Typed structurally rather than against `ErrorEvent`: the events maplibre
 * emits for tile failures carry `sourceId` at runtime, but the published type
 * for `map.on("error")` is the bare `ErrorEvent`, which does not declare it.
 */
function isBasemapTileFailure(event: unknown): boolean {
  if (typeof event !== "object" || event === null) return false;
  return (event as { sourceId?: unknown }).sourceId === BASEMAP_SOURCE_ID;
}

/** Layers that are interactive (click/hover targets) */
const INTERACTIVE_LAYERS = [
  LAYER_IDS.sourcesPoint,
  LAYER_IDS.sourcesLine,
  LAYER_IDS.sourcesArea,
  LAYER_IDS.buildingsFill,
  LAYER_IDS.barrierLine,
  LAYER_IDS.receiversPoint,
];

interface MapViewProps {
  children?: React.ReactNode;
  /**
   * Initial center [lng, lat]. Default: center of Germany. Read once on mount;
   * later changes are ignored so the user's pan is never yanked back.
   */
  center?: [number, number];
  /** Initial zoom level. Read once on mount, like `center`. */
  zoom?: number;
  /** Called when a feature is clicked. */
  onFeatureClick?: (features: MapGeoJSONFeature[], e: MapMouseEvent) => void;
  /** Called when the hovered feature changes. */
  onFeatureHover?: (feature: MapGeoJSONFeature | null) => void;
}

export function MapView({
  children,
  center = [10.45, 51.16],
  zoom = 6,
  onFeatureClick,
  onFeatureHover,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<Map | null>(null);
  const [map, setMap] = useState<Map | null>(null);
  const [mapError, setMapError] = useState<string | null>(
    webglDisabledForSession ? MAP_UNAVAILABLE_MESSAGE : null,
  );
  const basemap = useMapStore((s) => s.basemap);
  const tilesFailed = useMapStore((s) => s.tilesFailed);
  const reportTilesFailed = useMapStore((s) => s.reportTilesFailed);

  // `center`/`zoom` are documented as the *initial* viewport, so they are read
  // through a ref that the props never update. Keeping them in the init
  // effect's dependency list tore the map down and rebuilt it on every model
  // edit (the caller re-derives the view from the model), losing the user's
  // pan/zoom, re-adding every layer and re-arming the load-timeout fallback.
  //
  // The *map* does update it, on teardown: a basemap switch and a tile-failure
  // fallback both rebuild the map, and a rebuild that read the mount-time
  // props would throw the user back to the initial view. The workspace page
  // computes its view once, so nothing else would ever put them back.
  const viewRef = useRef({ center, zoom });

  // Initialize map
  useEffect(() => {
    if (mapError) return;
    if (!containerRef.current) return;

    let m: Map;
    try {
      m = new maplibregl.Map({
        container: containerRef.current,
        style: tilesFailed ? OFFLINE_STYLE : basemapStyle(basemap),
        center: viewRef.current.center,
        zoom: viewRef.current.zoom,
        attributionControl: {},
      });
    } catch {
      disableWebGLForSession();
      setMapError(MAP_UNAVAILABLE_MESSAGE);
      return;
    }

    m.addControl(new maplibregl.NavigationControl(), "top-right");
    m.addControl(
      new maplibregl.ScaleControl({ unit: "metric" }),
      "bottom-left",
    );

    // A lost WebGL context is transient by definition, and MapLibre handles the
    // restore itself: its own canvas listeners call `preventDefault()`, stash
    // the style, then rebuild the painter and resize on `webglcontextrestored`.
    // This component must merely keep the canvas mounted meanwhile — a lost
    // context must never set `mapError`, because the error panel would unmount
    // the very canvas the restored event fires on.

    m.on("load", () => {
      mapRef.current = m;
      setMap(m);
    });

    // A tile source that cannot be reached is a basemap problem, not a map
    // problem: the canvas, the model layers and every interaction still work.
    // So it raises a flag the style is selected from and leaves `mapError`
    // alone — see `offline-notice.tsx` for why the two stay separate.
    const handleError = (event: unknown) => {
      if (isBasemapTileFailure(event)) reportTilesFailed();
    };
    m.on("error", handleError);

    const fallbackTimer = window.setTimeout(() => {
      if (!mapRef.current) {
        setMapError(MAP_TIMEOUT_MESSAGE);
      }
    }, MAP_LOAD_TIMEOUT_MS);

    return () => {
      window.clearTimeout(fallbackTimer);
      // Remember where the user was before the instance goes: this cleanup is
      // the last moment the viewport exists, and the next instance reads it.
      try {
        const center = m.getCenter();
        viewRef.current = {
          center: [center.lng, center.lat],
          zoom: m.getZoom(),
        };
      } catch {
        // A map that never painted has no viewport to remember; the last
        // known one stays in force.
      }
      m.off("error", handleError);
      mapRef.current = null;
      setMap(null);
      m.remove();
    };
    // Rebuilds the map on a basemap, tile-failure or error-state change.
  }, [basemap, mapError, tilesFailed, reportTilesFailed]);

  // Clearing `mapError` re-runs the init effect; the session switch is reset
  // unconditionally because a timeout never set it and a throw needs it reset.
  const retryMapInit = () => {
    enableWebGLForSession();
    setMapError(null);
  };

  // Feature click handler
  useEffect(() => {
    const m = mapRef.current;
    if (!m || !onFeatureClick) return;

    const handler = (e: MapMouseEvent) => {
      const features = m.queryRenderedFeatures(e.point, {
        layers: INTERACTIVE_LAYERS.filter((id) => {
          try {
            return m.getLayer(id) != null;
          } catch {
            return false;
          }
        }),
      });
      if (features.length > 0) {
        onFeatureClick(features, e);
      }
    };

    m.on("click", handler);
    return () => {
      m.off("click", handler);
    };
  }, [map, onFeatureClick]);

  // Feature hover handler (cursor + callback)
  useEffect(() => {
    const m = mapRef.current;
    if (!m) return;

    const handleMove = (e: MapMouseEvent) => {
      const features = m.queryRenderedFeatures(e.point, {
        layers: INTERACTIVE_LAYERS.filter((id) => {
          try {
            return m.getLayer(id) != null;
          } catch {
            return false;
          }
        }),
      });

      const canvas = m.getCanvas();
      if (features.length > 0) {
        canvas.style.cursor = "pointer";
        onFeatureHover?.(features[0] ?? null);
      } else {
        canvas.style.cursor = "";
        onFeatureHover?.(null);
      }
    };

    m.on("mousemove", handleMove);
    return () => {
      m.off("mousemove", handleMove);
    };
  }, [map, onFeatureHover]);

  return (
    <MapContext value={map}>
      <div className="relative flex flex-1">
        {mapError ? (
          <div className="absolute inset-0 flex items-center justify-center bg-background p-8 text-center">
            <Card className="max-w-md space-y-2 p-6">
              <p className="text-lg font-semibold">Map unavailable</p>
              <p className="text-sm text-muted-foreground">{mapError}</p>
              <Button variant="outline" size="sm" onClick={retryMapInit}>
                Retry
              </Button>
            </Card>
          </div>
        ) : (
          <>
            <div ref={containerRef} className="absolute inset-0" />
            {/* Rendered here rather than by the page: the listener that raises
                the flag lives in this component, so every map that uses it
                explains a failed basemap the same way. */}
            <OfflineNotice />
          </>
        )}
        {children}
      </div>
    </MapContext>
  );
}

// Re-export for convenience
export { SOURCE_IDS, LAYER_IDS };
