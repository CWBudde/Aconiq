import { useEffect, useRef } from "react";
import type maplibregl from "maplibre-gl";
import { useMap } from "./use-map";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";
import { featuresToSourceGroups, receiversToGeoJSON } from "@/model/to-geojson";
import { computeWorkspaceBounds, toLngLatBounds } from "./extent";
import type { DisplayModel } from "./display-model";
import { CRSNotice } from "./crs-notice";
import {
  SOURCE_IDS,
  SELECTED_STATE_KEY,
  BUILDING_LAYERS,
  GROUND_ZONE_LAYERS,
  BARRIER_LAYERS,
  SOURCE_LAYERS,
  REVIEW_LAYERS,
  RECEIVER_LAYERS,
  CALC_AREA_LAYERS,
  MODEL_LAYER_GROUPS,
} from "./layers";
import { useMapStore } from "./map-store";

const EMPTY_COLLECTION: GeoJSON.FeatureCollection = {
  type: "FeatureCollection",
  features: [],
};

// Module-level, so that "nothing to draw" is the *same* empty array on every
// render. A fresh `[]` per render would change the sync effect's dependencies
// every time and re-run it forever.
const NO_FEATURES: ModelFeature[] = [];
const NO_RECEIVERS: ModelReceiver[] = [];

export interface ModelLayersProps {
  /** The projected model this component draws — see the note below. */
  display: DisplayModel;
  /**
   * The feature or receiver the editor is open on, or `null`.
   *
   * It arrives here rather than being read from a store because this component
   * is the one place that knows which GeoJSON source a model id ended up in —
   * `featuresToSourceGroups` splits the features by kind, and a feature state
   * is addressed by (source, id). A selection highlight written anywhere else
   * would have to duplicate that split.
   */
  selectedFeatureId?: string | null;
}

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
 * It renders the notice itself rather than returning `null` so that the map has
 * one place where an unprojectable model is explained.
 *
 * The answer arrives as a prop rather than from a `useDisplayModel` call here.
 * There must be exactly one instance of that hook on the map — a second one is
 * a second projection of the whole workspace per store change — and `pages/map.tsx`
 * holds it, because the draw gate needs the same answer: a model the map could
 * not project is one no finished shape can be placed in either.
 */
export function ModelLayers({
  display,
  selectedFeatureId = null,
}: ModelLayersProps) {
  const map = useMap();
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
      [SOURCE_IDS.groundZones, groups.groundZones],
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
      // Above the calculation area and below everything else: a ground zone is
      // the surface the rest of the model stands on.
      ...GROUND_ZONE_LAYERS,
      ...BUILDING_LAYERS,
      ...BARRIER_LAYERS,
      // Under the sources, so a flagged road reads as a red line on a violet
      // casing rather than as a violet line.
      ...REVIEW_LAYERS,
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

    // A layer comes back at the visibility its specification declares, which is
    // not necessarily the one the user chose: `LayerControl` writes a toggle
    // straight onto the map and keeps the same answer in the store, and a
    // basemap switch or a tile-failure fallback tears the map down and has this
    // effect add the layers again. Without this the hidden group reappears on
    // the canvas while the control still labels it as hidden.
    //
    // Read imperatively rather than subscribed to: the store is consulted at
    // the moment the layers exist, so a toggle does not have to re-run the
    // source sync above to be honoured — the control has already applied it.
    applyLayerVisibility(map, useMapStore.getState().layerVisibility);

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

  useFeatureSelection(map, selectedFeatureId, features, receivers);

  return <CRSNotice model={display} />;
}

/** Which GeoJSON source a feature of this kind is drawn from. */
const SOURCE_ID_FOR_KIND: Record<ModelFeature["kind"], string> = {
  source: SOURCE_IDS.sources,
  building: SOURCE_IDS.buildings,
  barrier: SOURCE_IDS.barriers,
  "ground-zone": SOURCE_IDS.groundZones,
};

interface SelectionTarget {
  source: string;
  id: string;
}

/**
 * The (source, id) pair MapLibre needs, or null when the id names nothing that
 * is currently drawn.
 *
 * Null covers more than a typo: a feature the editor is open on is not drawn
 * while the display model is projecting, and a selected feature can be deleted
 * out from under the panel. Both must clear the highlight rather than leave a
 * state key on a feature that no longer exists — `setFeatureState` stores the
 * state whether or not the feature is there, so a stale write comes back as a
 * highlight on whatever id is reused next.
 */
function selectionTarget(
  selectedFeatureId: string | null,
  features: ModelFeature[],
  receivers: ModelReceiver[],
): SelectionTarget | null {
  if (selectedFeatureId == null || selectedFeatureId === "") return null;

  const feature = features.find((f) => f.id === selectedFeatureId);
  if (feature) {
    return { source: SOURCE_ID_FOR_KIND[feature.kind], id: feature.id };
  }

  const receiver = receivers.find((r) => r.id === selectedFeatureId);
  if (receiver) {
    return { source: SOURCE_IDS.receivers, id: receiver.id };
  }

  return null;
}

/**
 * Marks the edited feature on the map, and unmarks the one before it.
 *
 * The feature state is a property of the *map*, not of the model: the model is
 * what a run reads, and a highlight is not an input to anything. So this
 * writes `feature-state` and never a model property, which is also why it
 * survives an undo of the edit the panel is making.
 *
 * `to-geojson.ts` puts the model id on the emitted feature's `id` member, which
 * is what makes the pair addressable at all — a GeoJSON source with no feature
 * ids accepts every `setFeatureState` call and paints none of them.
 */
function useFeatureSelection(
  map: maplibregl.Map | null,
  selectedFeatureId: string | null,
  features: ModelFeature[],
  receivers: ModelReceiver[],
): void {
  const appliedRef = useRef<SelectionTarget | null>(null);

  useEffect(() => {
    if (!map) return;

    const target = selectionTarget(selectedFeatureId, features, receivers);
    const applied = appliedRef.current;

    if (
      applied &&
      (!target || applied.id !== target.id || applied.source !== target.source)
    ) {
      try {
        map.removeFeatureState(applied, SELECTED_STATE_KEY);
      } catch (error) {
        console.error(
          `ModelLayers: could not clear the selection on "${applied.id}"`,
          error,
        );
      }
      appliedRef.current = null;
    }

    if (!target) return;

    try {
      map.setFeatureState(target, { [SELECTED_STATE_KEY]: true });
      appliedRef.current = target;
    } catch (error) {
      console.error(
        `ModelLayers: could not select "${target.id}" on the map`,
        error,
      );
    }
  }, [map, selectedFeatureId, features, receivers]);
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

/**
 * Puts the stored show/hide answer back onto the layers that are on the map.
 *
 * Model groups only: they are the layers this component adds, and the result
 * groups are not on the map yet. A group with no stored answer falls back to
 * its own `defaultVisible`, the same reading `LayerControl` shows.
 */
function applyLayerVisibility(
  map: maplibregl.Map,
  visibility: Record<string, boolean>,
): void {
  for (const group of MODEL_LAYER_GROUPS) {
    const visible = visibility[group.id] ?? group.defaultVisible;
    for (const layerId of group.layerIds) {
      try {
        map.setLayoutProperty(
          layerId,
          "visibility",
          visible ? "visible" : "none",
        );
      } catch {
        // The layer is not on this style. The next sync adds it and reapplies.
      }
    }
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
