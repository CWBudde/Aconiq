import { MapPanel } from "./map-panel";
import { DISPLAY_CRS } from "./display-model";
import type { DisplayModel } from "./display-model";
import { m } from "@/i18n/messages";

/**
 * What the map has to say about the CRS it is drawing in.
 *
 * Four states, one panel. `role="status"` rather than `alert`: none of these
 * is an interruption, and a projection finishing is exactly the kind of quiet
 * change a live region is for. The `ready` state carries a badge only when the
 * coordinates were actually moved — a reader looking at a model in WGS84 needs
 * no notice that it is in WGS84.
 *
 * It also carries why drawing is off, on the same condition `pages/map.tsx`
 * computes `drawingDisabled` from — the store's CRS is not the display one. The
 * toolbar can only say it in a tooltip and the start panel is not up over a
 * workspace that has content, so on a populated metric model this panel is the
 * one surface a user who followed `?draw=1` and saw nothing happen can read.
 */
export function CRSNotice({ model }: { model: DisplayModel }) {
  if (model.status === "ready" && !model.reprojected) {
    return null;
  }

  const drawingDisabled = model.sourceCRS !== DISPLAY_CRS;

  return (
    <MapPanel
      position="top-right"
      inset="right-12 top-3"
      width="w-72"
      translucent
      role="status"
      aria-label={m.label_crs_notice()}
      className="space-y-1 text-xs leading-relaxed"
    >
      <p>{noticeText(model)}</p>
      {drawingDisabled ? (
        <p className="text-muted-foreground">
          {m.msg_draw_disabled_crs({ crs: model.sourceCRS })}
        </p>
      ) : null}
    </MapPanel>
  );
}

function noticeText(model: DisplayModel): string {
  switch (model.status) {
    case "ready":
      return m.msg_map_crs_reprojected({ crs: model.sourceCRS });
    case "projecting":
      return m.msg_map_crs_projecting({ crs: model.sourceCRS });
    case "unsupported":
      return m.msg_map_crs_unsupported({ crs: model.sourceCRS });
    case "failed":
      return m.msg_map_crs_failed({
        crs: model.sourceCRS,
        reason: model.error.message,
      });
  }
}
