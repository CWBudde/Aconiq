import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import type { Map, MapMouseEvent, MapGeoJSONFeature } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { MapContext } from "./use-map";
import { BASEMAP_STYLES } from "./basemap";
import { useMapStore } from "./map-store";
import { LAYER_IDS, SOURCE_IDS } from "./layers";

/**
 * How long to wait for the map's `load` event before giving up and showing the
 * "map unavailable" fallback. A WebGL context that is created but never paints
 * (blocked GPU, software rasterizer, remote session) never fires `load`.
 *
 * `load` only fires after the style *and* the first render complete, so a slow
 * network produces the same signal as a broken GPU. The threshold is therefore
 * deliberately generous, and a timeout is treated as a per-mount condition
 * rather than session-wide evidence that WebGL is unusable: remounting (a route
 * change or basemap switch) retries.
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
 */
let webglDisabledForSession = false;

function disableWebGLForSession(): void {
  webglDisabledForSession = true;
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

  // `center`/`zoom` are documented as the *initial* viewport, so they are read
  // through a ref that is never updated. Keeping them in the init effect's
  // dependency list tore the map down and rebuilt it on every model edit (the
  // caller re-derives the view from the model), losing the user's pan/zoom,
  // re-adding every layer and re-arming the load-timeout fallback.
  const initialViewRef = useRef({ center, zoom });

  // Initialize map
  useEffect(() => {
    if (mapError) return;
    if (!containerRef.current) return;

    let m: Map;
    try {
      m = new maplibregl.Map({
        container: containerRef.current,
        style: BASEMAP_STYLES[basemap],
        center: initialViewRef.current.center,
        zoom: initialViewRef.current.zoom,
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

    const canvas = m.getCanvas();
    const handleContextLost = (event: Event) => {
      event.preventDefault();
      setMapError("WebGL context was lost.");
    };
    const handleContextRestored = () => {
      m.resize();
    };
    canvas.addEventListener("webglcontextlost", handleContextLost);
    canvas.addEventListener("webglcontextrestored", handleContextRestored);

    m.on("load", () => {
      mapRef.current = m;
      setMap(m);
    });

    const fallbackTimer = window.setTimeout(() => {
      if (!mapRef.current) {
        setMapError(MAP_TIMEOUT_MESSAGE);
      }
    }, MAP_LOAD_TIMEOUT_MS);

    return () => {
      window.clearTimeout(fallbackTimer);
      canvas.removeEventListener("webglcontextlost", handleContextLost);
      canvas.removeEventListener("webglcontextrestored", handleContextRestored);
      mapRef.current = null;
      setMap(null);
      m.remove();
    };
    // Rebuilds the map only on a basemap or error-state change.
  }, [basemap, mapError]);

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
            <div className="max-w-md space-y-2 rounded-2xl border bg-card p-6 shadow-sm">
              <p className="text-lg font-semibold">Map unavailable</p>
              <p className="text-sm text-muted-foreground">{mapError}</p>
            </div>
          </div>
        ) : (
          <div ref={containerRef} className="absolute inset-0" />
        )}
        {children}
      </div>
    </MapContext>
  );
}

// Re-export for convenience
export { SOURCE_IDS, LAYER_IDS };
