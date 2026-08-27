import type { MapMouseEvent } from "maplibre-gl";

/** Extract [lng, lat] tuple from a MapLibre mouse event */
export function eventToLngLat(e: MapMouseEvent): [number, number] {
  return [e.lngLat.lng, e.lngLat.lat];
}

/**
 * True when the event target is a control with its own undo history that the
 * browser already handles (text inputs, textareas, selects, contenteditable).
 * Global keyboard shortcuts must bow out for these.
 */
export function isTextEntryTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}
