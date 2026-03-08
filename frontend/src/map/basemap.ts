import type { StyleSpecification } from "maplibre-gl";

/**
 * Available basemap styles.
 *
 * Uses OpenFreeMap for tile hosting (free, no API key, open-source).
 * Fallback: a minimal inline style for offline/air-gapped use.
 */

function createRasterStyle(
  name: string,
  backgroundColor: string,
  rasterPaint?: NonNullable<StyleSpecification["layers"]>[number]["paint"],
): StyleSpecification {
  return {
    version: 8,
    name,
    sources: {
      osm: {
        type: "raster",
        tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
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
      {
        id: "osm-raster",
        type: "raster",
        source: "osm",
        paint: rasterPaint ?? {},
      },
    ],
  };
}

export const BASEMAP_STYLES = {
  /** Light basemap — good for noise level overlays */
  light: createRasterStyle("osm-light", "#eef2e8"),
  /** Standard basemap with terrain context */
  bright: createRasterStyle("osm-bright", "#f4f0e8", {
    "raster-saturation": 0.1,
    "raster-contrast": 0.05,
  }),
  /** Dark-ish basemap variant without a heavy vector style */
  dark: createRasterStyle("osm-dark", "#20252b", {
    "raster-saturation": -0.85,
    "raster-brightness-min": 0.05,
    "raster-brightness-max": 0.75,
    "raster-contrast": 0.2,
  }),
} as const;

export type BasemapId = keyof typeof BASEMAP_STYLES;

export const DEFAULT_BASEMAP: BasemapId = "light";

/**
 * Minimal fallback style for offline use (no external tiles).
 * Shows a plain background with no features.
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
