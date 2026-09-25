import type { LonLatBBox } from "@/model/footprint";

/**
 * A bounding box as the reader is typing it, in WGS84 degrees.
 *
 * Held as text, not numbers, so a half-typed value survives a re-render. The
 * OpenStreetMap and LGLN tabs share one: the page holds it, so a box typed —
 * or geolocated — into one tab is the box the other one loads.
 */
export interface BBoxText {
  south: string;
  west: string;
  north: string;
  east: string;
}

/**
 * The box as numbers, or `null` while any of the four is missing or not a
 * number. It does not reorder a box typed inside out; the server refuses one,
 * and says so in its own words.
 */
export function parseBBox(text: BBoxText): LonLatBBox | null {
  const south = parseFloat(text.south);
  const west = parseFloat(text.west);
  const north = parseFloat(text.north);
  const east = parseFloat(text.east);
  if (isNaN(south) || isNaN(west) || isNaN(north) || isNaN(east)) {
    return null;
  }
  return { south, west, north, east };
}
