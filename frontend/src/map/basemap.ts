import type { StyleSpecification } from "maplibre-gl";

/**
 * Available basemap styles.
 *
 * Uses OpenFreeMap for tile hosting (free, no API key, open-source).
 * Fallback: a minimal inline style for offline/air-gapped use.
 */

export const BASEMAP_STYLES = {
  /** Light basemap — good for noise level overlays */
  light: "https://tiles.openfreemap.org/styles/positron",
  /** Standard basemap with terrain context */
  bright: "https://tiles.openfreemap.org/styles/bright",
  /** Dark basemap — high-contrast result display */
  dark: "https://tiles.openfreemap.org/styles/dark",
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
