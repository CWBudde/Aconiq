import { useEffect, useRef, useState } from "react";
import { backend } from "@/api/backend";
import { useModelStore } from "@/model/model-store";
import { formatCoordinate } from "@/ui/format";
import { DISPLAY_CRS } from "./display-model";
import { MapPanel } from "./map-panel";
import { useMap } from "./use-map";

interface Coords {
  lng: number;
  lat: number;
}

/** Where the pointer is in the CRS the model is stored in. */
interface ProjectedCoords {
  crs: string;
  x: number;
  y: number;
}

/**
 * How long the pointer has to settle before the position is projected.
 *
 * MapLibre fires `mousemove` per frame, and in API mode each projection is an
 * HTTP round trip. A trailing debounce means a pointer sweeping across the map
 * costs one request instead of sixty, and a reader who has stopped to read a
 * coordinate waits an eighth of a second for it.
 */
const PROJECT_DEBOUNCE_MS = 120;

/**
 * Where the pointer is — in WGS 84, which is what MapLibre knows, and in the
 * CRS the model is stored in, which is what the user is working in.
 *
 * The second line is the point of this component. A project in EPSG:25832 is
 * drawn in WGS 84 (see `display-model.ts`) and the map's event carries only
 * lon/lat, so the readout used to answer in degrees for a model measured in
 * metres — numbers a user could not check against anything in their own data.
 * The projection is `aconiq.transform`'s, through `backend.transformCoordinates`,
 * for the same reason the display projection is: the frontend must not grow a
 * second transverse-Mercator implementation.
 *
 * It is a readout and nothing more. Nothing here writes to the store, and the
 * projected pair is display-only — `use-draw-projection.ts` is the one path
 * that turns a map coordinate into a model one.
 */
export function CoordinateDisplay() {
  const map = useMap();
  const crs = useModelStore((s) => s.crs);
  const [coords, setCoords] = useState<Coords | null>(null);
  const [projected, setProjected] = useState<ProjectedCoords | null>(null);

  const projectable =
    crs !== DISPLAY_CRS && backend.capabilities.canReprojectForDisplay;

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

  // Monotonic, compared on arrival: answers can overtake each other, and an
  // older one landing last would pin the readout to a position the pointer has
  // already left.
  const requestRef = useRef(0);

  useEffect(() => {
    if (!projectable || coords === null) {
      requestRef.current += 1;
      setProjected(null);
      return;
    }

    const request = (requestRef.current += 1);
    // The pointer has moved, so the pair on screen belongs to a position it has
    // left. Keeping it until the replacement arrives would show the new lon/lat
    // beside the old easting/northing — two coordinates for two different
    // points, presented as one — and a slow or failing transform would leave
    // that standing. The readout says nothing rather than something false.
    setProjected(null);

    const timer = setTimeout(() => {
      void backend
        .transformCoordinates({
          source_crs: DISPLAY_CRS,
          target_crs: crs,
          coordinates: [coords.lng, coords.lat],
        })
        .then(
          (response) => {
            if (requestRef.current !== request) return;
            const x = response.coordinates[0];
            const y = response.coordinates[1];
            if (x === undefined || y === undefined) return;
            setProjected({ crs: response.target_crs, x, y });
          },
          () => {
            if (requestRef.current !== request) return;
            // A failed projection leaves the WGS 84 line standing rather than
            // blanking the panel: it is still true, and the map itself says
            // what it could not project in `CRSNotice`.
            setProjected(null);
          },
        );
    }, PROJECT_DEBOUNCE_MS);

    return () => {
      clearTimeout(timer);
    };
  }, [coords, crs, projectable]);

  if (!coords) return null;

  return (
    <MapPanel
      position="bottom-center"
      translucent
      className="px-2 py-1 text-center"
    >
      {/* Easting then northing, which is EPSG:25832's own axis order and the
          order every projected coordinate in this project is written in — while
          the WGS 84 line below stays latitude first, the display convention for
          degrees. The two orders differ, so each line is named: the CRS code
          labels the projected pair, and the unlabelled one is the geographic
          one the map itself is drawn in. */}
      {projected ? (
        <div className="flex items-baseline justify-center gap-1.5">
          <span className="text-2xs text-muted-foreground">
            {projected.crs}
          </span>
          <span className="font-mono text-2xs tabular-nums">
            {formatCoordinate(projected.x, 2)},{" "}
            {formatCoordinate(projected.y, 2)}
          </span>
        </div>
      ) : null}
      <span className="font-mono text-2xs tabular-nums text-muted-foreground">
        {formatCoordinate(coords.lat, 6)}, {formatCoordinate(coords.lng, 6)}
      </span>
    </MapPanel>
  );
}
