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
 * The id every emitted calculation area carries.
 *
 * Fixed, not generated: the saved file's bytes are the model's identity —
 * `POST /api/v1/model` answers with their SHA-256 and the frontend keeps that
 * receipt — so an id that changed per save would make unchanged content hash
 * differently. The Go side pins the same property from its end
 * (`TestSaveModelIsByteDeterministic`). Every other id in the model is a
 * `crypto.randomUUID()` or an `osm-way-<id>`, so a bare constant collides
 * with none of them.
 */
export const CALC_AREA_FEATURE_ID = "calc-area";

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

export function receiversToGeoJSON(
  receivers: ModelReceiver[],
): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: receivers.map((r) => ({
      type: "Feature" as const,
      id: r.id,
      properties: {
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
 */
export function calcAreaToGeoJSON(
  area: CalcArea,
): GeoJSONFeatureCollection["features"][number] {
  return {
    type: "Feature" as const,
    id: CALC_AREA_FEATURE_ID,
    properties: { kind: "calc-area" },
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
  return {
    type: "FeatureCollection",
    features: [
      ...featuresToGeoJSON(features).features,
      ...receiversToGeoJSON(receivers).features,
      ...(calcArea === null ? [] : [calcAreaToGeoJSON(calcArea)]),
    ],
  };
}
