/**
 * What the run will cost in time, under the parameters, while the reader is
 * still choosing them — and, when the answer is long, what would shorten it.
 *
 * The receiver count above the grid fields says how many points; it cannot
 * say how long, because what one receiver costs depends on the buildings
 * around it more than on anything the dialog shows. This is the backend's own
 * measurement of the run the dialog describes (see `useRunEstimate`), shown
 * as the band it measured, never as a single figure it cannot vouch for.
 *
 * A warning is never a refusal. The run button stays where it is; the
 * suggestions only write parameters into the form.
 */

import { AlertCircle, Loader2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Callout } from "@/ui/callout";
import { formatNumber } from "@/ui/format";
import type { GridExtent, GridSizing } from "@/model/grid-estimate";
import {
  ESTIMATE_WARNING_SECONDS,
  roundBand,
  suggestFasterRun,
  type EstimateBand,
  type EstimateSuggestion,
} from "@/model/run-estimate";
import type { RunEstimateState } from "@/api/hooks";
import { m } from "@/i18n/messages";

/** A band in the reader's locale: "≈ 6–9 min" / "≈ 6–9 Min.". */
function formatEstimateBand(band: EstimateBand): string {
  const rounded = roundBand(band);
  if (rounded === "under-a-second") return m.duration_under_a_second();

  const low = formatNumber(rounded.low, 0);
  const high = formatNumber(rounded.high, 0);

  if (rounded.low >= rounded.high) {
    const value = high;
    if (rounded.unit === "s") return m.duration_single_s({ value });
    if (rounded.unit === "min") return m.duration_single_min({ value });
    return m.duration_single_h({ value });
  }

  if (rounded.unit === "s") return m.duration_band_s({ low, high });
  if (rounded.unit === "min") return m.duration_band_min({ low, high });
  return m.duration_band_h({ low, high });
}

function suggestionLabel(suggestion: EstimateSuggestion): string {
  const band = formatEstimateBand(suggestion.wall);
  const resolution = formatNumber(suggestion.resolutionM ?? 0, 0);
  const receivers = formatNumber(suggestion.receivers ?? 0, 0);

  if (suggestion.resolutionM === undefined) {
    return m.action_use_estimate_distance_scaled({ band });
  }
  if (suggestion.distanceScaled) {
    return m.action_use_estimate_both({ resolution, receivers, band });
  }
  return m.action_use_estimate_grid({ resolution, receivers, band });
}

function Note({ children }: { children: React.ReactNode }) {
  return (
    <p className="text-xs text-muted-foreground" data-testid="run-estimate">
      {children}
    </p>
  );
}

export function RunEstimateNote({
  state,
  extent,
  sizing,
  params,
  onApplyParams,
}: {
  state: RunEstimateState;
  /** The auto-grid extent, or null for a custom receiver set. */
  extent: GridExtent | null;
  sizing: GridSizing;
  params: Record<string, string>;
  /** Writes the suggested parameters into the form. */
  onApplyParams: (params: Record<string, string>) => void;
}) {
  if (state.status === "idle") return null;
  if (state.status === "estimating") {
    return (
      <Note>
        <Loader2
          aria-hidden="true"
          className="mr-1 inline h-3 w-3 animate-spin align-[-2px]"
        />
        {m.status_run_estimating()}
      </Note>
    );
  }
  if (state.status === "failed")
    return <Note>{m.msg_run_estimate_failed()}</Note>;

  const { estimate } = state;
  if (!estimate.available || estimate.wall_s === undefined) {
    return <Note>{m.msg_run_estimate_unavailable()}</Note>;
  }

  const line = m.msg_run_estimate({
    band: formatEstimateBand(estimate.wall_s),
  });
  const detail =
    estimate.inputs === undefined || estimate.probe === undefined
      ? null
      : m.msg_run_estimate_detail({
          receivers: formatNumber(estimate.inputs.receivers, 0),
          workers: formatNumber(estimate.inputs.workers, 0),
          timed: formatNumber(estimate.probe.units_timed, 0),
        });

  if (estimate.wall_s.estimate <= ESTIMATE_WARNING_SECONDS) {
    return (
      <Note>
        {line}
        {detail === null ? null : (
          <span className="block opacity-80">{detail}</span>
        )}
      </Note>
    );
  }

  const suggestions = suggestFasterRun(estimate, extent, sizing, params);

  return (
    // A group, not an alert, for the reason `GridEstimateNote` gives: the
    // estimate follows the form, and a live region would interrupt a
    // screen-reader user every time it lands.
    <Callout
      variant="warning"
      icon={AlertCircle}
      role="group"
      data-testid="run-estimate"
    >
      <div className="space-y-2">
        <p>{line}</p>
        {detail === null ? null : <p className="text-xs">{detail}</p>}
        {suggestions.length === 0 ? null : (
          <>
            <p>{m.msg_run_estimate_warning()}</p>
            <div className="flex flex-col items-start gap-2">
              {suggestions.map((suggestion) => (
                <Button
                  key={JSON.stringify(suggestion.params)}
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    onApplyParams(suggestion.params);
                  }}
                >
                  {suggestionLabel(suggestion)}
                </Button>
              ))}
            </div>
          </>
        )}
      </div>
    </Callout>
  );
}
