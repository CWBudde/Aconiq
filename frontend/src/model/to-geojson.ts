import type { GeoJSONFeatureCollection, ModelFeature } from "./types";

export function featuresToGeoJSON(
  features: ModelFeature[],
): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: features.map((f) => ({
      type: "Feature" as const,
      id: f.id,
      properties: {
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
