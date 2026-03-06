import { useEffect, useState } from "react";
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
    <div className="absolute bottom-2 right-2 z-10 rounded-md border bg-background/90 px-2 py-1 shadow-sm backdrop-blur-sm">
      <span className="text-[10px] font-mono tabular-nums text-muted-foreground">
        {coords.lat.toFixed(6)}, {coords.lng.toFixed(6)}
      </span>
    </div>
  );
}
