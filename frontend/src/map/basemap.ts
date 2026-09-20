import type { RasterLayerSpecification, StyleSpecification } from "maplibre-gl";
import { m } from "@/i18n/messages";
import { getTileURL } from "./tile-source";

/**
 * Available basemap styles.
 *
 * All three are the same raster tile source under different paint settings —
 * the source is whatever `tile-source.ts` resolves, which defaults to the
 * public OpenStreetMap tile server. Styles are therefore built on demand
 * rather than frozen at module load: the tile URL is a per-browser setting,
 * and a style captured at import time would ignore it.
 *
 * Fallback: {@link OFFLINE_STYLE}, a minimal inline style for offline or
 * air-gapped use, and for a session whose tiles have started failing.
 */

/**
 * The source id every basemap style uses.
 *
 * Exported because `map-view.tsx` matches MapLibre's `error` events against it
 * to tell a failing basemap apart from a failing model source: both arrive on
 * the same event, and only the first one is a reason to fall back.
 */
export const BASEMAP_SOURCE_ID = "osm";

function createRasterStyle(
  name: string,
  tileUrl: string,
  backgroundColor: string,
  rasterPaint?: RasterLayerSpecification["paint"],
): StyleSpecification {
  const rasterLayer: RasterLayerSpecification = {
    id: "osm-raster",
    type: "raster",
    source: BASEMAP_SOURCE_ID,
    paint: rasterPaint ?? {},
  };

  return {
    version: 8,
    name,
    sources: {
      [BASEMAP_SOURCE_ID]: {
        type: "raster",
        tiles: [tileUrl],
        tileSize: 256,
        attribution: "&copy; OpenStreetMap contributors",
      },
    },
    layers: [
      {
        id: "background",
        type: "background",
        paint: {
          "background-color": backgroundColor,
        },
      },
      rasterLayer,
    ],
  };
}

interface BasemapRecipe {
  /** The style name MapLibre reports; not user-facing. */
  name: string;
  background: string;
  paint?: RasterLayerSpecification["paint"];
  /** What the picker calls it. */
  label: () => string;
}

/**
 * The three recipes, and the reason each one carries the paint it does.
 *
 * All three draw the same raster tiles, so the paint is the entire difference
 * between them — and it has to be big enough to see. `light` used to carry no
 * paint at all and `bright` a 0.1 saturation nudge over it, which made the two
 * the same picture: the tile server's own output, twice. A basemap the picker
 * offers by name has to look like its name.
 *
 * `light` is the one the map defaults to, and its job is to stay out of the
 * way of what is drawn on top — the result contours and the model geometry
 * carry the colour there. Lifting the black point is what does most of the
 * work: desaturating alone would not, because full-strength grey road casings
 * compete with a contour line just as well as coloured ones do. The
 * desaturation stays mild on purpose, so that `light` reads as a pale map and
 * not as a second `dark`; the three have to differ from each other, not only
 * from the raw tiles.
 *
 * `bright` is the opposite choice, for reading the map itself — finding the
 * street a receiver sits on, checking a building footprint against what is
 * there. It pushes saturation and contrast past the raw tiles rather than
 * matching them.
 */
const BASEMAP_RECIPES = {
  /** Muted backdrop — the default, and the one to overlay results on. */
  light: {
    name: "osm-light",
    background: "#eef2e8",
    paint: {
      "raster-saturation": -0.25,
      "raster-contrast": -0.15,
      "raster-brightness-min": 0.38,
    },
    label: () => m.label_basemap_light(),
  },
  /** Full-strength basemap, for reading the map rather than the overlay. */
  bright: {
    name: "osm-bright",
    background: "#f4f0e8",
    paint: {
      "raster-saturation": 0.45,
      "raster-contrast": 0.18,
    },
    label: () => m.label_basemap_bright(),
  },
  /** Dark-ish basemap variant without a heavy vector style */
  dark: {
    name: "osm-dark",
    background: "#20252b",
    paint: {
      "raster-saturation": -0.85,
      "raster-brightness-min": 0.05,
      "raster-brightness-max": 0.75,
      "raster-contrast": 0.2,
    },
    label: () => m.label_basemap_dark(),
  },
} as const satisfies Record<string, BasemapRecipe>;

export type BasemapId = keyof typeof BASEMAP_RECIPES;

/** The picker's order, and the set a stored preference is validated against. */
export const BASEMAP_IDS = Object.keys(BASEMAP_RECIPES) as BasemapId[];

export const DEFAULT_BASEMAP: BasemapId = "light";

export function basemapLabel(id: BasemapId): string {
  return BASEMAP_RECIPES[id].label();
}

export function isBasemapId(value: unknown): value is BasemapId {
  // `Object.hasOwn` rather than `in`: `in` walks the prototype chain, so
  // `"constructor"` and `"toString"` would pass as ids and `basemapStyle`
  // would read a function off `Object.prototype` as a recipe — a style with no
  // background colour, which MapLibre rejects instead of falling back.
  return typeof value === "string" && Object.hasOwn(BASEMAP_RECIPES, value);
}

/**
 * The style for one basemap, against the tile URL in force right now.
 *
 * `tileUrl` is a parameter rather than a read so a test can pin it; callers in
 * the app pass nothing and get the stored override or the default.
 */
export function basemapStyle(
  id: BasemapId,
  tileUrl: string = getTileURL(),
): StyleSpecification {
  // Widened on purpose: each recipe's literal type names its own paint keys,
  // so the union has to be read through the declared shape.
  const recipe: BasemapRecipe = BASEMAP_RECIPES[id];
  return createRasterStyle(
    recipe.name,
    tileUrl,
    recipe.background,
    recipe.paint,
  );
}

// --- Persisted basemap choice -----------------------------------------------

/**
 * The picker's choice survives a reload, through the same pattern as the tile
 * URL: plain localStorage, every access guarded, an unreadable or unknown
 * value silently treated as "not set". The value is validated against
 * {@link BASEMAP_IDS} rather than trusted, because a stale key from an older
 * build would otherwise index `BASEMAP_RECIPES` with `undefined`.
 */
export const BASEMAP_STORAGE_KEY = "aconiq.basemap";

export function readStoredBasemap(): BasemapId {
  try {
    const value = localStorage.getItem(BASEMAP_STORAGE_KEY);
    return isBasemapId(value) ? value : DEFAULT_BASEMAP;
  } catch {
    return DEFAULT_BASEMAP;
  }
}

export function storeBasemap(id: BasemapId): void {
  try {
    localStorage.setItem(BASEMAP_STORAGE_KEY, id);
  } catch {
    // Storage unavailable or full. Ignore silently.
  }
}

/**
 * Minimal fallback style for offline use (no external tiles).
 * Shows a plain background with no features.
 *
 * It is not a basemap the user can pick: the map falls back to it on its own
 * when the tile source starts failing, and picks the chosen basemap back up
 * once the notice's Retry clears that.
 */
export const OFFLINE_STYLE: StyleSpecification = {
  version: 8,
  name: "offline-fallback",
  sources: {},
  layers: [
    {
      id: "background",
      type: "background",
      paint: {
        "background-color": "#f0f0f0",
      },
    },
  ],
};
