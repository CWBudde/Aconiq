import { m } from "@/i18n/messages";
import { formatNumber } from "@/ui/format";
import type {
  ElevationWindow,
  FacadeElevation,
  FacadeElevationResponse,
} from "@/wasm/types";

/** A window's frame in words, for its tooltip and its accessible name. */
export function windowLabel(w: ElevationWindow): string {
  return m.label_window_frame({
    id: w.id,
    width: formatNumber(w.window.width_m, 2),
    height: formatNumber(w.window.height_m, 2),
    sill: formatNumber(w.window.sill_z_m, 2),
    u: formatNumber(w.window.u_m, 2),
  });
}

/** A facade as the picker names it: its number, the way it faces, its length. */
export function facadeOptionLabel(f: FacadeElevation, n: number): string {
  const values = {
    n,
    facing: compassLabel(f.facing_deg),
    length: formatNumber(f.length_m),
  };
  return f.courtyard
    ? m.label_facade_option_courtyard(values)
    : m.label_facade_option(values);
}

/**
 * The eighth of the compass a bearing (degrees from grid north) falls in. It
 * only names the bearing the Go code measured; it measures nothing.
 */
export function compassLabel(facingDeg: number): string {
  switch (Math.round(facingDeg / 45) % 8) {
    case 1:
      return m.label_compass_ne();
    case 2:
      return m.label_compass_e();
    case 3:
      return m.label_compass_se();
    case 4:
      return m.label_compass_s();
    case 5:
      return m.label_compass_sw();
    case 6:
      return m.label_compass_w();
    case 7:
      return m.label_compass_nw();
    default:
      return m.label_compass_n();
  }
}

export function slabSourceLabel(
  source: FacadeElevationResponse["slab_height_source"],
): string {
  switch (source) {
    case "measured":
      return m.label_slab_source_measured();
    case "levels":
      return m.label_slab_source_levels();
    case "assumed":
      return m.label_slab_source_assumed();
  }
}

export function reviewLabel(status: ElevationWindow["review_status"]): string {
  switch (status) {
    case "proposed":
      return m.label_review_proposed();
    case "confirmed":
      return m.label_review_confirmed();
    case "rejected":
      return m.label_review_rejected();
  }
}
