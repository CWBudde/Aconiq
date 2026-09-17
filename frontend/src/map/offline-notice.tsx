import { Button } from "@/ui/components/button";
import { MapPanel } from "./map-panel";
import { useMapStore } from "./map-store";
import { m } from "@/i18n/messages";

/**
 * What the map has to say when its tiles stop arriving.
 *
 * `role="status"` rather than `alert`, and deliberately not the "Map
 * unavailable" panel: a failed tile is not a failed map. The canvas is intact,
 * every model and result layer is still drawn, and only the imagery underneath
 * is missing — so this is a notice over a working map, not a replacement for
 * one. Routing it through `mapError` would unmount the canvas, which is the
 * one thing that must not happen here (and is why a lost WebGL context does
 * not set `mapError` either).
 *
 * Retry clears the flag, which rebuilds the map on the chosen basemap. There is
 * no automatic retry: tiles fail for reasons that do not fix themselves on a
 * timer — an air-gapped network, a blocked host, a tile URL with a typo in it —
 * and a map that silently re-requested them would keep the failure invisible.
 *
 * Its own inset: the top-right corner is taken by the layer control and the CRS
 * notice, so this stacks above the coordinate readout on the bottom edge, where
 * it is central enough to be read and small enough not to cover the model.
 */
export function OfflineNotice() {
  const tilesFailed = useMapStore((s) => s.tilesFailed);
  const clearTilesFailed = useMapStore((s) => s.clearTilesFailed);

  if (!tilesFailed) return null;

  return (
    <MapPanel
      position="bottom-center"
      inset="bottom-14 left-1/2 -translate-x-1/2"
      width="w-80"
      translucent
      role="status"
      aria-label={m.label_basemap_offline()}
      className="space-y-1 text-xs leading-relaxed"
    >
      <p>{m.msg_basemap_tiles_failed()}</p>
      <Button
        variant="outline"
        size="sm"
        className="h-7 text-xs"
        onClick={clearTilesFailed}
      >
        {m.action_retry()}
      </Button>
    </MapPanel>
  );
}
