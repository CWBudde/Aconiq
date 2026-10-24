import { m } from "@/i18n/messages";
import type { ValidationIssue } from "@/model/types";
import { validationIssueText } from "@/model/validation-message";
import { formatNumber } from "@/ui/format";
import type {
  ElevationFinding,
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

/**
 * A window's finding in the reader's language, through `validationIssueText`
 * as every other finding on screen is. The kernel's finding carries its code
 * and English prose but no parameters, so a code whose sentence wants one
 * (`window.invalid` names the property) and a code the catalogue has no
 * sentence for read as a plain finding; the code shown beside it names it.
 * The prose is never shown: it would be English under a German heading.
 */
export function findingText(finding: ElevationFinding): string {
  const issue = plainIssue(finding);
  return issue === null
    ? m.msg_facade_finding_unnamed()
    : validationIssueText(issue);
}

/** The finding as a `ValidationIssue`, for the codes that take no parameters. */
function plainIssue(finding: ElevationFinding): ValidationIssue | null {
  const featureId = finding.feature_id ?? "";
  switch (finding.code) {
    case "feature.id.duplicate":
    case "window.geometry.invalid":
    case "window.facade.unresolved":
    case "window.facade.moved":
    case "window.frame.outside":
    case "window.frame.above_building":
      return {
        level: finding.level,
        code: finding.code,
        featureId,
        params: {},
      };
    default:
      return null;
  }
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
