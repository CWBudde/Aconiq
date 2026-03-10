import { useEffect, useRef } from "react";
import type maplibregl from "maplibre-gl";
import { useMap } from "./use-map";
import { useModelStore } from "@/model/model-store";
import type { CalcArea } from "@/model/types";
import { featuresToSourceGroups, receiversToGeoJSON } from "@/model/to-geojson";
import {
  SOURCE_IDS,
  BUILDING_LAYERS,
  BARRIER_LAYERS,
  SOURCE_LAYERS,
  RECEIVER_LAYERS,
  CALC_AREA_LAYERS,
} from "./layers";

/**
 * Syncs the model store features to MapLibre GeoJSON sources.
 * Must be rendered as a child of MapView (inside MapContext).
 */
export function ModelLayers() {
  const map = useMap();
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const calcArea = useModelStore((s) => s.calcArea);
  const previousFeatureCountRef = useRef(0);

  useEffect(() => {
    if (!map) return;
    if (!isMapStyleReady(map)) return;

    const groups = featuresToSourceGroups(features);
    const calcAreaGeoJSON = calcAreaToGeoJSON(calcArea);

    const entries = [
      [SOURCE_IDS.calcArea, calcAreaGeoJSON],
      [SOURCE_IDS.buildings, groups.buildings],
      [SOURCE_IDS.barriers, groups.barriers],
      [SOURCE_IDS.sources, groups.sources],
      [SOURCE_IDS.receivers, receiversToGeoJSON(receivers)],
    ] as const;

    for (const [sourceId, data] of entries) {
      try {
        const existing = map.getSource(sourceId);
        if (existing && "setData" in existing) {
          (existing as maplibregl.GeoJSONSource).setData(
            data as unknown as GeoJSON.GeoJSON,
          );
        } else if (!existing) {
          map.addSource(sourceId, {
            type: "geojson",
            data: data as unknown as GeoJSON.GeoJSON,
          });
        }
      } catch {
        return;
      }
    }

    // Ensure layers exist (idempotent — skip if already added)
    // Calc area layers go below model feature layers
    const allLayers = [
      ...CALC_AREA_LAYERS,
      ...BUILDING_LAYERS,
      ...BARRIER_LAYERS,
      ...SOURCE_LAYERS,
      ...RECEIVER_LAYERS,
    ];
    for (const layer of allLayers) {
      try {
        if (!map.getLayer(layer.id)) {
          map.addLayer(layer);
        }
      } catch {
        return;
      }
    }

    // Bring freshly imported data into view once instead of leaving it off-screen.
    if (previousFeatureCountRef.current === 0 && features.length > 0) {
      const bounds = computeFeatureBounds(features);
      if (bounds) {
        map.fitBounds(bounds, {
          padding: 48,
          duration: 0,
        });
      }
    }
    previousFeatureCountRef.current = features.length;
  }, [map, features, receivers, calcArea]);

  return null;
}

function isMapStyleReady(map: maplibregl.Map): boolean {
  try {
    const style = map.getStyle();
    return Boolean(style && style.sources);
  } catch {
    return false;
  }
}

function calcAreaToGeoJSON(
  calcArea: CalcArea | null,
): GeoJSON.FeatureCollection {
  if (!calcArea) {
    return { type: "FeatureCollection", features: [] };
  }
  return {
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        properties: {},
        geometry: calcArea.geometry,
      },
    ],
  };
}

function computeFeatureBounds(
  features: ReturnType<typeof useModelStore.getState>["features"],
): maplibregl.LngLatBoundsLike | null {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;

  function visit(coords: unknown): void {
    if (!Array.isArray(coords)) return;
    if (
      coords.length >= 2 &&
      typeof coords[0] === "number" &&
      typeof coords[1] === "number"
    ) {
      minX = Math.min(minX, coords[0]);
      minY = Math.min(minY, coords[1]);
      maxX = Math.max(maxX, coords[0]);
      maxY = Math.max(maxY, coords[1]);
      return;
    }
    for (const value of coords) {
      visit(value);
    }
  }

  for (const feature of features) {
    visit(feature.geometry.coordinates);
  }

  if (!Number.isFinite(minX)) {
    return null;
  }

  return [
    [minX, minY],
    [maxX, maxY],
  ];
}
