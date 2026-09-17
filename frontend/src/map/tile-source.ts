/**
 * Where the basemap raster tiles come from.
 *
 * Deliberately a mirror of `@/api/mode`'s override helpers: pure localStorage
 * functions, read through one normaliser, every access wrapped so a browser
 * with storage disabled falls back to the default instead of throwing on the
 * way to a map. Nothing here touches MapLibre — `basemap.ts` builds the style
 * from whatever this returns, so pointing the map at another tile host is a
 * matter of writing this one key.
 *
 * Offline-first cuts both ways here: the shipped default is a public OSM tile
 * server, which an air-gapped install cannot reach, and this override is how
 * such an install points the map at a tile server it does have.
 *
 * Unlike an API base URL, a tile template is not normalised beyond trimming —
 * it ends in `.png` or a style suffix, and stripping a trailing slash off one
 * would corrupt the few templates that genuinely end in a directory.
 */

/** The tile template used when nothing is stored. */
export const DEFAULT_TILE_URL =
  "https://tile.openstreetmap.org/{z}/{x}/{y}.png";

export const TILE_URL_OVERRIDE_KEY = "aconiq.basemap_tile_url";

function readLocalStorageOverride(): string | null {
  try {
    const value = localStorage.getItem(TILE_URL_OVERRIDE_KEY);
    if (!value) return null;
    const normalized = value.trim();
    return normalized.length > 0 ? normalized : null;
  } catch {
    return null;
  }
}

export function getTileURL(): string {
  return readLocalStorageOverride() ?? DEFAULT_TILE_URL;
}

export function hasTileURLOverride(): boolean {
  return readLocalStorageOverride() !== null;
}

export function setTileURLOverride(value: string): void {
  try {
    const normalized = value.trim();
    if (normalized.length === 0) {
      localStorage.removeItem(TILE_URL_OVERRIDE_KEY);
      return;
    }
    localStorage.setItem(TILE_URL_OVERRIDE_KEY, normalized);
  } catch {
    // Storage unavailable or full. Ignore silently.
  }
}

export function clearTileURLOverride(): void {
  try {
    localStorage.removeItem(TILE_URL_OVERRIDE_KEY);
  } catch {
    // Storage unavailable. Ignore silently.
  }
}
