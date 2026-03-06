import type { MapMouseEvent } from "maplibre-gl";

/** Extract [lng, lat] tuple from a MapLibre mouse event */
export function eventToLngLat(e: MapMouseEvent): [number, number] {
  return [e.lngLat.lng, e.lngLat.lat];
}
