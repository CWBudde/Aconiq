/**
 * What the automatic receiver grid will cost, under the grid fieldset's
 * legend, while the reader is still typing into it.
 *
 * `grid_resolution_m` used to sit among nineteen fields with no feedback of
 * any kind. Ten metres over a 2 km × 2 km site is some 42 000 receivers, and
 * nothing said so: the field defaulted to the value that quietly commits the
 * user to an hour of compute, and the first sign of it was a progress bar
 * that would not move. This is the number itself — exact, free, and derived
 * from the same `model/grid-estimate` the run builds its receivers from.
 *
 * TODO: the count is only half of "what will this run cost". Wall time is the
 * half a reader actually wants, and it does not follow from the count — a
 * scene with buildings costs tens of milliseconds a receiver, a bare one well
 * under a millisecond. The follow-up is a timing probe: run a fixed
 * 16-receiver sample of this grid through the WASM kernel, measure it, and
 * scale to the full count. It needs the kernel, so it would speak in browser
 * mode and stay silent in API mode rather than guess.
 */

import { AlertCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Callout } from "@/ui/callout";
import { formatNumber } from "@/ui/format";
import {
  gridShape,
  gridSizingFromParams,
  RECEIVER_WARNING_THRESHOLD,
  resolutionForReceiverBudget,
} from "@/model/grid-estimate";
import type { GridExtentState } from "@/run/use-grid-extent";
import { m } from "@/i18n/messages";

/** The quiet one-liner: the reader glances at it and reads on. */
function Note({ children }: { children: React.ReactNode }) {
  return (
    <p
      className="-mt-1 text-xs text-muted-foreground"
      data-testid="grid-estimate"
    >
      {children}
    </p>
  );
}

export function GridEstimateNote({
  state,
  params,
  onUseResolution,
}: {
  state: GridExtentState;
  params: Record<string, string>;
  /** Writes a coarser `grid_resolution_m` into the form. */
  onUseResolution: (resolutionM: number) => void;
}) {
  // Nothing was asked, or nothing can be answered. Silence rather than a
  // placeholder: a backend that cannot project has no metric extent to count
  // cells over, and a row of dashes under the legend would only ask the
  // reader to work out why.
  if (state.status === "idle" || state.status === "unavailable") return null;
  if (state.status === "projecting")
    return <Note>{m.status_grid_estimate()}</Note>;
  if (state.status === "failed")
    return <Note>{m.msg_grid_estimate_failed()}</Note>;
  if (state.status === "no-extent")
    return <Note>{m.msg_grid_extent_unknown()}</Note>;

  const sizing = gridSizingFromParams(params);
  const shape = gridShape(state.extent, sizing);
  if (shape === null) return <Note>{m.msg_grid_resolution_invalid()}</Note>;

  const line = m.msg_grid_receiver_estimate({
    count: formatNumber(shape.count, 0),
    width: formatNumber(shape.width, 0),
    height: formatNumber(shape.height, 0),
  });

  if (shape.count <= RECEIVER_WARNING_THRESHOLD) return <Note>{line}</Note>;

  const suggestion = resolutionForReceiverBudget(
    state.extent,
    sizing.paddingM,
    RECEIVER_WARNING_THRESHOLD,
  );

  return (
    // `role="group"` rather than the warning variant's default `alert`: the
    // count follows the resolution field keystroke by keystroke, and a live
    // region would interrupt a screen-reader user on every one of them. The
    // box is part of the field's own description, not an event.
    <Callout
      variant="warning"
      icon={AlertCircle}
      role="group"
      data-testid="grid-estimate"
    >
      <div className="space-y-2">
        <p>{line}</p>
        <p>{m.msg_grid_receiver_warning()}</p>
        {suggestion === null ? null : (
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              onUseResolution(suggestion);
            }}
          >
            {m.action_use_grid_resolution({
              resolution: formatNumber(suggestion, 0),
            })}
          </Button>
        )}
      </div>
    </Callout>
  );
}
