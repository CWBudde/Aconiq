import { createContext, useContext } from "react";
import type { Map } from "maplibre-gl";

/**
 * Context for sharing the MapLibre map instance with child components.
 * The map may be null before initialization completes.
 */
export const MapContext = createContext<Map | null>(null);

export function useMap(): Map | null {
  return useContext(MapContext);
}
