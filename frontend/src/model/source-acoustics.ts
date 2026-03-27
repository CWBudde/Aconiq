import type { ModelFeature } from "./types";

export const RLS19_SURFACE_TYPES = [
  "SMA",
  "SMA-5-8",
  "SMA-8-11",
  "AB",
  "OPA",
  "OPA-11",
  "OPA-8",
  "Pflaster",
  "Pflaster-eben",
  "Pflaster-sonstig",
  "Beton",
  "LOA",
  "SMA-LA-8",
  "DSH-V",
  "Gussasphalt",
  "Gussasphalt-nicht-geriffelt",
  "beschaedigt",
] as const;

export const RLS19_JUNCTION_TYPES = [
  "none",
  "signalized",
  "roundabout",
  "other",
] as const;

export const RLS19_SPEED_KEYS = [
  "speed_pkw_kph",
  "speed_lkw1_kph",
  "speed_lkw2_kph",
  "speed_krad_kph",
] as const;

export const RLS19_TRAFFIC_KEYS = [
  "traffic_day_pkw",
  "traffic_day_lkw1",
  "traffic_day_lkw2",
  "traffic_day_krad",
  "traffic_night_pkw",
  "traffic_night_lkw1",
  "traffic_night_lkw2",
  "traffic_night_krad",
] as const;

export function getFeatureProperties(
  feature: ModelFeature,
): Record<string, unknown> {
  return feature.properties ?? {};
}

export function getFeatureString(
  feature: ModelFeature,
  ...keys: string[]
): string | undefined {
  const props = getFeatureProperties(feature);
  for (const key of keys) {
    const value = props[key];
    if (typeof value !== "string") {
      continue;
    }
    const trimmed = value.trim();
    if (trimmed !== "") {
      return trimmed;
    }
  }
  return undefined;
}

export function getFeatureNumber(
  feature: ModelFeature,
  ...keys: string[]
): number | undefined {
  const props = getFeatureProperties(feature);
  for (const key of keys) {
    const raw = props[key];
    const value =
      typeof raw === "number"
        ? raw
        : typeof raw === "string"
          ? Number.parseFloat(raw)
          : Number.NaN;
    if (Number.isFinite(value)) {
      return value;
    }
  }
  return undefined;
}

export function getFeatureBoolean(
  feature: ModelFeature,
  key: string,
): boolean | undefined {
  const value = getFeatureProperties(feature)[key];
  return typeof value === "boolean" ? value : undefined;
}

export function setFeatureProperty(
  feature: ModelFeature,
  key: string,
  value: string | number | boolean | undefined,
  ...aliases: string[]
): ModelFeature {
  const nextProperties = { ...(feature.properties ?? {}) };
  Reflect.deleteProperty(nextProperties, `${key}_inferred`);
  for (const alias of aliases) {
    Reflect.deleteProperty(nextProperties, alias);
    Reflect.deleteProperty(nextProperties, `${alias}_inferred`);
  }
  if (value == null || value === "") {
    Reflect.deleteProperty(nextProperties, key);
  } else {
    nextProperties[key] = value;
  }

  return {
    ...feature,
    properties:
      Object.keys(nextProperties).length > 0 ? nextProperties : undefined,
  };
}

export function getRLS19ReviewRequired(feature: ModelFeature): boolean {
  return (
    getFeatureBoolean(feature, "source_acoustics_review_required") === true
  );
}

export function getInferredFlag(feature: ModelFeature, key: string): boolean {
  return getFeatureBoolean(feature, `${key}_inferred`) === true;
}
