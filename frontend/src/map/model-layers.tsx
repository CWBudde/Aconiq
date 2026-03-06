import { useEffect } from "react";
import type maplibregl from "maplibre-gl";
import { useMap } from "./use-map";
import { useModelStore } from "@/model/model-store";
import { featuresToSourceGroups } from "@/model/to-geojson";
import {
  SOURCE_IDS,
  BUILDING_LAYERS,
  BARRIER_LAYERS,
  SOURCE_LAYERS,
} from "./layers";

/**
 * Syncs the model store features to MapLibre GeoJSON sources.
 * Must be rendered as a child of MapView (inside MapContext).
 */
export function ModelLayers() {
  const map = useMap();
  const features = useModelStore((s) => s.features);

  useEffect(() => {
    if (!map) return;

    const groups = featuresToSourceGroups(features);

    const entries = [
      [SOURCE_IDS.buildings, groups.buildings],
      [SOURCE_IDS.barriers, groups.barriers],
      [SOURCE_IDS.sources, groups.sources],
    ] as const;

    for (const [sourceId, data] of entries) {
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
    }

    // Ensure layers exist (idempotent — skip if already added)
    const allLayers = [...BUILDING_LAYERS, ...BARRIER_LAYERS, ...SOURCE_LAYERS];
    for (const layer of allLayers) {
      if (!map.getLayer(layer.id)) {
        map.addLayer(layer);
      }
    }
  }, [map, features]);

  return null;
}
