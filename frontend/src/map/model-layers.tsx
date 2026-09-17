import { useEffect, useRef } from "react";
import type maplibregl from "maplibre-gl";
import { useMap } from "./use-map";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { featuresToSourceGroups, receiversToGeoJSON } from "@/model/to-geojson";
import { computeWorkspaceBounds, toLngLatBounds } from "./extent";
import { useDisplayModel } from "./display-model";
import { CRSNotice } from "./crs-notice";
import {
  SOURCE_IDS,
  BUILDING_LAYERS,
  BARRIER_LAYERS,
  SOURCE_LAYERS,
  RECEIVER_LAYERS,
  CALC_AREA_LAYERS,
} from "./layers";

const EMPTY_COLLECTION: GeoJSON.FeatureCollection = {
  type: "FeatureCollection",
  features: [],
};

// Module-level, so that "nothing to draw" is the *same* empty array on every
// render. A fresh `[]` per render would change the sync effect's dependencies
// every time and re-run it forever.
const NO_FEATURES: ModelFeature[] = [];
const NO_RECEIVERS: ModelReceiver[] = [];

/**
 * Syncs the model store features to MapLibre GeoJSON sources.
 * Must be rendered as a child of MapView (inside MapContext).
 *
 * It draws {@link useDisplayModel}'s answer, not the store's arrays: the store
 * may be in a projected CRS, and MapLibre takes lon/lat and nothing else. While
 * that answer is anything but `ready` the sources are emptied rather than left
 * as they were — a stale model still drawn, in the wrong place, is the bug this
 * component had.
 *
 * It renders the notice itself rather than returning `null` so that there is
 * exactly one `useDisplayModel` instance on the map, and therefore one
 * projection per store change.
 */
export function ModelLayers() {
  const map = useMap();
  const display = useDisplayModel();
  const previousContentCountRef = useRef(0);

  const ready = display.status === "ready";
  const features = ready ? display.features : NO_FEATURES;
  const receivers = ready ? display.receivers : NO_RECEIVERS;
  const calcArea = ready ? display.calcArea : null;

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
      } catch (error) {
        // Bailing out silently left the map showing stale or missing data with
        // nothing anywhere to say why. The early return stays — a style that
        // rejects one source will reject the rest — but the cause is reported.
        console.error(
          `ModelLayers: could not sync GeoJSON source "${sourceId}"`,
          error,
        );
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
      } catch (error) {
        console.error(`ModelLayers: could not add layer "${layer.id}"`, error);
        return;
      }
    }

    // A fit only makes sense over a model that is actually being drawn, and
    // the counter must only advance on `ready` — advancing it on the
    // `projecting` render would consume the one-shot fit on an empty map and
    // leave the reprojected model off-screen for good.
    if (!ready) return;

    // Counted over everything `fitToWorkspace` frames, not features alone. The
    // feature count cancelled the very fix the shared bounds helper is here
    // for: a receiver-only or calc-area-only workspace has an extent, and
    // gating on `features.length` left it at the fallback view for good.
    const contentCount =
      features.length + receivers.length + (calcArea ? 1 : 0);

    // Bring freshly imported data into view once instead of leaving it off-screen.
    if (previousContentCountRef.current === 0 && contentCount > 0) {
      fitToWorkspace(map, features, receivers, calcArea);
    }
    previousContentCountRef.current = contentCount;
  }, [map, ready, features, receivers, calcArea]);

  return <CRSNotice model={display} />;
}

/**
 * Fits the view to everything the workspace holds, and does nothing at all if
 * the extent is not a lon/lat one.
 *
 * Both halves are deliberate. The traversal is `extent.ts`'s, so a
 * receiver-only import comes into view — the copy this file used to keep
 * visited features alone. And the guard is what keeps a 4326-*labelled* store
 * that actually holds metres from throwing `Invalid LngLat latitude value` out
 * of the effect above: the call was outside both `try` blocks, so it took the
 * whole sync down with it.
 */
function fitToWorkspace(
  map: maplibregl.Map,
  features: ModelFeature[],
  receivers: ModelReceiver[],
  calcArea: CalcArea | null,
): void {
  const bounds = computeWorkspaceBounds(features, receivers, calcArea);
  if (!bounds) return;

  const lngLat = toLngLatBounds(bounds);
  if (!lngLat) {
    console.warn(
      "ModelLayers: the model's extent is not lon/lat, so the view was not fitted to it",
      bounds,
    );
    return;
  }

  try {
    map.fitBounds(lngLat, { padding: 48, duration: 0 });
  } catch (error) {
    console.error("ModelLayers: could not fit the view to the model", error);
  }
}

function isMapStyleReady(map: maplibregl.Map): boolean {
  try {
    const style = map.getStyle();
    return Boolean(style.sources);
  } catch {
    return false;
  }
}

function calcAreaToGeoJSON(
  calcArea: CalcArea | null,
): GeoJSON.FeatureCollection {
  if (!calcArea) {
    return EMPTY_COLLECTION;
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
