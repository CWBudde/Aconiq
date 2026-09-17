import type {
  CalcArea,
  GeoJSONFeatureCollection,
  ModelFeature,
  ModelReceiver,
} from "./types";

/**
 * The part of the model the project persists.
 *
 * `calcArea` is required rather than optional so that adding it made the
 * compiler enumerate the call sites instead of letting one keep silently
 * sending a model without the area the user drew.
 */
export interface ModelPayload {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
}

/**
 * The id an emitted calculation area carries when it has none of its own.
 *
 * Derived, not generated: the saved file's bytes are the model's identity —
 * `POST /api/v1/model` answers with their SHA-256 and the frontend keeps that
 * receipt — so an id that changed per save would make unchanged content hash
 * differently. The Go side pins the same property from its end
 * (`TestSaveModelIsByteDeterministic`).
 *
 * It is a starting point rather than the answer, because nothing reserves it:
 * an imported file may carry a source with exactly this id, and the backend
 * refuses the whole model with `feature.id.duplicate` when two features share
 * one. See `resolveCalcAreaID`.
 */
export const CALC_AREA_FEATURE_ID = "calc-area";

/**
 * The id to emit for this area: the one the project already gave it, else the
 * first unoccupied `calc-area`, `calc-area-1`, `calc-area-2`… .
 *
 * Deterministic in the model's own content — the same model emits the same id
 * every time, which is what the hash receipt needs — while never colliding
 * with a feature that got there first. A stored id that a feature has since
 * taken loses to that feature rather than breaking the save.
 */
function resolveCalcAreaID(area: CalcArea, taken: ReadonlySet<string>): string {
  if (area.id != null && area.id !== "" && !taken.has(area.id)) {
    return area.id;
  }

  let candidate = CALC_AREA_FEATURE_ID;
  let suffix = 0;
  while (taken.has(candidate)) {
    suffix += 1;
    candidate = `${CALC_AREA_FEATURE_ID}-${String(suffix)}`;
  }

  return candidate;
}

export function featuresToGeoJSON(
  features: ModelFeature[],
): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: features.map((f) => ({
      type: "Feature" as const,
      id: f.id,
      properties: {
        ...(f.properties ?? {}),
        kind: f.kind,
        ...(f.sourceType != null ? { source_type: f.sourceType } : {}),
        ...(f.heightM != null ? { height_m: f.heightM } : {}),
      },
      geometry: {
        type: f.geometry.type,
        coordinates: f.geometry.coordinates as unknown,
      },
    })),
  };
}

interface SourceGroups {
  sources: GeoJSONFeatureCollection;
  buildings: GeoJSONFeatureCollection;
  barriers: GeoJSONFeatureCollection;
}

export function featuresToSourceGroups(features: ModelFeature[]): SourceGroups {
  return {
    sources: featuresToGeoJSON(features.filter((f) => f.kind === "source")),
    buildings: featuresToGeoJSON(features.filter((f) => f.kind === "building")),
    barriers: featuresToGeoJSON(features.filter((f) => f.kind === "barrier")),
  };
}

/**
 * The receivers as `receiver` features.
 *
 * The carried properties go first and `kind`/`height_m` last, exactly as
 * {@link featuresToGeoJSON} does it: the store's own fields win, and everything
 * else the receiver arrived with survives the round trip. Emitting only the two
 * derived properties dropped `bimschv16_area_category` on the first save, and
 * `assessment/bimschv16` then skipped the receiver as missing its category.
 */
export function receiversToGeoJSON(
  receivers: ModelReceiver[],
): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: receivers.map((r) => ({
      type: "Feature" as const,
      id: r.id,
      properties: {
        ...(r.properties ?? {}),
        kind: "receiver",
        height_m: r.heightM,
      },
      geometry: {
        type: r.geometry.type,
        coordinates: r.geometry.coordinates as unknown,
      },
    })),
  };
}

/**
 * The calculation area as the one `calc-area` feature schema v1 allows.
 *
 * No `height_m`: the backend validator applies no height rule to this kind by
 * design — an area is a footprint bounding the receiver grid, not an object
 * sound travels around — so emitting one would be a property nothing reads.
 * The geometry is a Polygon and never a MultiPolygon, which the validator
 * refuses rather than reducing to its envelope.
 *
 * The carried properties go first and `kind` last, exactly as
 * {@link receiversToGeoJSON} does it: the store's own field wins, and
 * everything else the area arrived with survives the round trip. Emitting
 * `kind` alone dropped `soundplan_base_elevation_m` — which `aconiq import
 * --soundplan` reads off the first vertex of the bundle's `CalcArea.geo` — on
 * the first save from the map, with no recovery short of re-importing.
 */
export function calcAreaToGeoJSON(
  area: CalcArea,
  taken: ReadonlySet<string> = new Set<string>(),
): GeoJSONFeatureCollection["features"][number] {
  const id = resolveCalcAreaID(area, taken);
  const carried = area.properties ?? {};

  return {
    type: "Feature" as const,
    id,
    properties: {
      ...carried,
      // A carried `properties.id` is rewritten rather than passed on, and only
      // when the area carried one. The backend's `featureID` reads
      // `properties.id` *before* the GeoJSON `id` member, so one still naming
      // an id a feature has since taken would recreate exactly the
      // `feature.id.duplicate` that `resolveCalcAreaID` just stepped past.
      // Rewriting an existing key leaves it where it was, so the emitted key
      // order — and with it the saved file's bytes — stays deterministic.
      ...("id" in carried ? { id } : {}),
      kind: "calc-area",
    },
    geometry: {
      type: area.geometry.type,
      coordinates: area.geometry.coordinates as unknown,
    },
  };
}

/**
 * The model as one v1 FeatureCollection for `POST /api/v1/model`: features
 * first, then receivers, then the calculation area, in store order — the
 * order is part of what makes a save reproducible.
 *
 * The area goes last so that every payload written before it existed keeps a
 * byte-identical prefix: a model that gained no area serialises exactly as it
 * did, and one that did grows only at the end. It is a model feature rather
 * than a run setting because that is how the backend reads it — `aconiq run`
 * and `POST /api/v1/runs` both resolve the auto-grid extent from the saved
 * model (`resolveGridReceivers`), so an area that never reaches the file
 * never reaches a run either.
 */
export function modelToGeoJSON({
  features,
  receivers,
  calcArea,
}: ModelPayload): GeoJSONFeatureCollection {
  const emitted = [
    ...featuresToGeoJSON(features).features,
    ...receiversToGeoJSON(receivers).features,
  ];
  if (calcArea !== null) {
    // The ids already in the payload, so the area can pick one none of them
    // holds: the backend rejects the model outright when two features share an
    // id, and an imported file is free to carry one called `calc-area`.
    const taken = new Set<string>();
    for (const feature of emitted) {
      if (typeof feature.id === "string") taken.add(feature.id);
    }
    emitted.push(calcAreaToGeoJSON(calcArea, taken));
  }

  return { type: "FeatureCollection", features: emitted };
}
