import {
  MODEL_LAYER_GROUPS,
  RESULT_LAYER_GROUPS,
  type LayerGroup,
} from "./layers";

/**
 * The map preferences that outlive a reload, other than the basemap.
 *
 * A fourth member of the family `@/api/mode`, `./tile-source` and
 * `./basemap`'s stored-choice helpers already form: pure localStorage
 * functions, every access guarded so a browser with storage disabled falls
 * back to the default instead of throwing on the way to a map, and a value
 * that cannot be read or makes no sense treated as "not set".
 *
 * Nothing here touches MapLibre. `map-store.ts` seeds itself from these and
 * writes back through them, exactly as it already does for the basemap, so
 * these are the only place the keys are spelled.
 */

/** Which layer groups the user has switched off. */
export const LAYER_VISIBILITY_STORAGE_KEY = "aconiq.map_layers";

/** Which band of a result table the map paints. */
export const RESULT_INDICATOR_STORAGE_KEY = "aconiq.result_indicator";

export interface LayerVisibility {
  [groupId: string]: boolean;
}

/**
 * The group ids that exist in this build.
 *
 * A stored key is validated against them rather than trusted. Two reasons, and
 * only the second is about safety: a renamed or removed group would otherwise
 * keep its entry forever and the key would grow without bound, and a
 * hand-edited value must not reach `setLayoutProperty`, which takes
 * `"visible"`/`"none"` and not whatever JSON happened to parse.
 */
function knownGroupIds(): Set<string> {
  const groups: LayerGroup[] = [...MODEL_LAYER_GROUPS, ...RESULT_LAYER_GROUPS];
  return new Set(groups.map((group) => group.id));
}

export function readStoredLayerVisibility(): LayerVisibility {
  try {
    const raw = localStorage.getItem(LAYER_VISIBILITY_STORAGE_KEY);
    if (!raw) return {};

    const parsed: unknown = JSON.parse(raw);
    if (
      typeof parsed !== "object" ||
      parsed === null ||
      Array.isArray(parsed)
    ) {
      return {};
    }

    const known = knownGroupIds();
    const visibility: LayerVisibility = {};
    for (const [groupId, visible] of Object.entries(parsed)) {
      if (!known.has(groupId)) continue;
      if (typeof visible !== "boolean") continue;
      visibility[groupId] = visible;
    }
    return visibility;
  } catch {
    // Storage unavailable, or the value is not JSON at all.
    return {};
  }
}

export function storeLayerVisibility(visibility: LayerVisibility): void {
  try {
    // An empty record is the default, so it is a removal rather than a stored
    // `{}` — the same shape `setTileURLOverride` gives an empty string.
    if (Object.keys(visibility).length === 0) {
      localStorage.removeItem(LAYER_VISIBILITY_STORAGE_KEY);
      return;
    }
    localStorage.setItem(
      LAYER_VISIBILITY_STORAGE_KEY,
      JSON.stringify(visibility),
    );
  } catch {
    // Storage unavailable or full. Ignore silently.
  }
}

/**
 * The remembered indicator, or `null` for "whatever the table lists first".
 *
 * Deliberately not validated against a list of indicators, because there is no
 * such list here: which bands exist is a property of the run's receiver table,
 * which arrives long after this is read. The stored value is a *preference*,
 * and `ResultLayers` resolves it against the table it actually has — see the
 * note there before changing this.
 */
export function readStoredResultIndicator(): string | null {
  try {
    const value = localStorage.getItem(RESULT_INDICATOR_STORAGE_KEY);
    if (!value) return null;
    const normalized = value.trim();
    return normalized.length > 0 ? normalized : null;
  } catch {
    return null;
  }
}

export function storeResultIndicator(indicator: string | null): void {
  try {
    const normalized = indicator?.trim() ?? "";
    if (normalized.length === 0) {
      localStorage.removeItem(RESULT_INDICATOR_STORAGE_KEY);
      return;
    }
    localStorage.setItem(RESULT_INDICATOR_STORAGE_KEY, normalized);
  } catch {
    // Storage unavailable or full. Ignore silently.
  }
}
