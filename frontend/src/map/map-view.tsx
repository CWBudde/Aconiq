import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import type { Map, MapMouseEvent, MapGeoJSONFeature } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { MapContext } from "./use-map";
import { BASEMAP_STYLES } from "./basemap";
import { FallbackMap } from "./fallback-map";
import { useMapStore } from "./map-store";
import { LAYER_IDS, SOURCE_IDS } from "./layers";

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
  /** Initial center [lng, lat]. Default: center of Germany. */
  center?: [number, number];
  /** Initial zoom level. */
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
  const [fallbackMode, setFallbackMode] = useState(false);
  const basemap = useMapStore((s) => s.basemap);

  // Initialize map
  useEffect(() => {
    if (fallbackMode) return;
    if (!containerRef.current) return;

    const m = new maplibregl.Map({
      container: containerRef.current,
      style: BASEMAP_STYLES[basemap],
      center,
      zoom,
      attributionControl: {},
    });

    m.addControl(new maplibregl.NavigationControl(), "top-right");
    m.addControl(
      new maplibregl.ScaleControl({ unit: "metric" }),
      "bottom-left",
    );

    const canvas = m.getCanvas();
    const handleContextLost = (event: Event) => {
      event.preventDefault();
      setFallbackMode(true);
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
        setFallbackMode(true);
      }
    }, 4000);

    return () => {
      window.clearTimeout(fallbackTimer);
      canvas.removeEventListener("webglcontextlost", handleContextLost);
      canvas.removeEventListener("webglcontextrestored", handleContextRestored);
      mapRef.current = null;
      setMap(null);
      m.remove();
    };
    // Only re-create the map on basemap change
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [basemap, fallbackMode, center, zoom]);

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
        {fallbackMode ? (
          <FallbackMap center={center} />
        ) : (
          <div ref={containerRef} className="absolute inset-0" />
        )}
        {!fallbackMode && map ? children : null}
      </div>
    </MapContext>
  );
}

// Re-export for convenience
export { SOURCE_IDS, LAYER_IDS };
