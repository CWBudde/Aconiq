import { useEffect, useState } from "react";
import { formatCoordinate } from "@/ui/format";
import { MapPanel } from "./map-panel";
import { useMap } from "./use-map";

interface Coords {
  lng: number;
  lat: number;
}

export function CoordinateDisplay() {
  const map = useMap();
  const [coords, setCoords] = useState<Coords | null>(null);

  useEffect(() => {
    if (!map) return;

    const handler = (e: { lngLat: Coords }) => {
      setCoords({ lng: e.lngLat.lng, lat: e.lngLat.lat });
    };

    map.on("mousemove", handler);
    return () => {
      map.off("mousemove", handler);
    };
  }, [map]);

  if (!coords) return null;

  return (
    <MapPanel
      position="bottom-right"
      inset="bottom-2 right-2"
      translucent
      className="px-2 py-1"
    >
      <span className="font-mono text-2xs tabular-nums text-muted-foreground">
        {formatCoordinate(coords.lat, 6)}, {formatCoordinate(coords.lng, 6)}
      </span>
    </MapPanel>
  );
}
