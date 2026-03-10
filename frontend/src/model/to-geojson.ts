import type {
  GeoJSONFeatureCollection,
  ModelFeature,
  ModelReceiver,
} from "./types";

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
