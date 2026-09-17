import type { ModelFeature, ModelReceiver } from "./types";

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

/** The first key holding a non-blank string, trimmed. */
function readString(
  properties: Record<string, unknown>,
  keys: string[],
): string | undefined {
  for (const key of keys) {
    const value = properties[key];
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

export function getFeatureString(
  feature: ModelFeature,
  ...keys: string[]
): string | undefined {
  return readString(getFeatureProperties(feature), keys);
}

/**
 * The same read against a receiver.
 *
 * A receiver is not a {@link ModelFeature} — it has no `kind` and no geometry
 * type beyond Point — but it carries the same free-form property bag, and
 * `bimschv16_area_category` lives in it. Without this the editor could show a
 * receiver's height and nothing else.
 */
export function getReceiverString(
  receiver: ModelReceiver,
  ...keys: string[]
): string | undefined {
  return readString(receiver.properties ?? {}, keys);
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

/**
 * The property bag a write leaves behind, or undefined once it holds nothing.
 *
 * An empty string and `undefined` both remove the key: an emptied field means
 * "unset", never "the empty string". The `_inferred` marker goes with it,
 * because a value chosen by hand is no longer an import's guess, and so do the
 * aliases, so two spellings of the same property cannot end up disagreeing.
 */
function writeProperty(
  properties: Record<string, unknown> | undefined,
  key: string,
  value: string | number | boolean | undefined,
  aliases: string[],
): Record<string, unknown> | undefined {
  const next = { ...(properties ?? {}) };
  Reflect.deleteProperty(next, `${key}_inferred`);
  for (const alias of aliases) {
    Reflect.deleteProperty(next, alias);
    Reflect.deleteProperty(next, `${alias}_inferred`);
  }
  if (value == null || value === "") {
    Reflect.deleteProperty(next, key);
  } else {
    next[key] = value;
  }
  return Object.keys(next).length > 0 ? next : undefined;
}

export function setFeatureProperty(
  feature: ModelFeature,
  key: string,
  value: string | number | boolean | undefined,
  ...aliases: string[]
): ModelFeature {
  const nextProperties = writeProperty(feature.properties, key, value, aliases);

  const next: ModelFeature = { ...feature };
  if (nextProperties !== undefined) {
    next.properties = nextProperties;
  } else {
    // Drop the key entirely rather than assigning `undefined`
    // (ModelFeature.properties is "absent or a value").
    Reflect.deleteProperty(next, "properties");
  }
  return next;
}

/**
 * The same write against a receiver, returning the whole receiver.
 *
 * `updateReceiver` is a full-object replace through the command stack, so the
 * caller hands it this result and gets one undoable edit — the same shape
 * `setFeatureProperty` plus `updateFeature` has on the feature side.
 */
export function setReceiverProperty(
  receiver: ModelReceiver,
  key: string,
  value: string | number | boolean | undefined,
  ...aliases: string[]
): ModelReceiver {
  const nextProperties = writeProperty(
    receiver.properties,
    key,
    value,
    aliases,
  );

  const next: ModelReceiver = { ...receiver };
  if (nextProperties !== undefined) {
    next.properties = nextProperties;
  } else {
    Reflect.deleteProperty(next, "properties");
  }
  return next;
}

export function getRLS19ReviewRequired(feature: ModelFeature): boolean {
  return (
    getFeatureBoolean(feature, "source_acoustics_review_required") === true
  );
}

export function getInferredFlag(feature: ModelFeature, key: string): boolean {
  return getFeatureBoolean(feature, `${key}_inferred`) === true;
}
