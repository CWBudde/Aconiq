import { MapPanel } from "./map-panel";
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
 */
export function CRSNotice({ model }: { model: DisplayModel }) {
  if (model.status === "ready" && !model.reprojected) {
    return null;
  }

  return (
    <MapPanel
      position="top-right"
      inset="right-12 top-3"
      width="w-72"
      translucent
      role="status"
      aria-label={m.label_crs_notice()}
      className="text-xs leading-relaxed"
    >
      {noticeText(model)}
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
