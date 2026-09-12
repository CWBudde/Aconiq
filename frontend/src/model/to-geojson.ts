import type {
  GeoJSONFeatureCollection,
  ModelFeature,
  ModelReceiver,
} from "./types";

/** The part of the model the project persists. */
export interface ModelPayload {
  features: ModelFeature[];
  receivers: ModelReceiver[];
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
 * The model as one v1 FeatureCollection for `POST /api/v1/model`: features
 * first, then receivers, in store order — the order is part of what makes a
 * save reproducible.
 *
 * The calculation area is deliberately not in the payload. The v1 schema
 * (`docs/geojson-schema-v1.md`) knows the kinds source, building, barrier and
 * receiver and nothing else, and the run request has no grid extent either,
 * so `calcArea` stays frontend state and travels only through the
 * localStorage draft. A backend auto-grid therefore uses the source extent;
 * the run dialog says so rather than claiming the area is active, until the
 * PLAN.md item that carries the area to the backend lands.
 */
export function modelToGeoJSON({
  features,
  receivers,
}: ModelPayload): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: [
      ...featuresToGeoJSON(features).features,
      ...receiversToGeoJSON(receivers).features,
    ],
  };
}
