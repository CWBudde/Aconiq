import { Loader2 } from "lucide-react";
import type { RunProgress } from "@/api/hooks";
import { m } from "@/i18n/messages";
import { formatEta } from "@/run/run-progress";

/**
 * What a run in flight shows: a determinate bar once the backend has reported
 * once, and a plain phase label before that.
 *
 * Two places draw it. The run dialog, while browser mode's `startRun` is
 * pending — there the reports come from the kernel, several a second. And the
 * run's own panel, while the API computes it in the background — there they
 * come from the run's `progress` in the run list, pushed by the event stream
 * or polled when it is down, about once every two seconds at most and with an
 * estimate of the time left.
 *
 * What is left on the label is the stretch before the receiver count is even
 * known: spawning the worker or the process, loading and projecting the model,
 * preparing the scene. A run whose standard does not count receivers as it
 * goes stays on it for the whole run.
 *
 * A bar at 0 of 13,000 is not a bar that has stalled; a spinner that has said
 * "starting" for a minute is. That is why the kernel and the server both
 * report 0 of n the moment the walk begins.
 *
 * The bar carries an accessible name: a screen reader announcing "62%" with
 * nothing to attach it to is a worse answer than none, and axe fails an
 * unnamed progressbar outright. `aria-valuetext` repeats the visible sentence
 * because "62" out of "1600" is not what a listener wants to hear.
 */
export function RunProgressBar({ progress }: { progress: RunProgress | null }) {
  if (progress === null) {
    return (
      <p
        className="flex items-center gap-2 text-xs text-muted-foreground"
        data-testid="run-progress-starting"
      >
        <Loader2 aria-hidden="true" className="h-3.5 w-3.5 animate-spin" />
        {m.status_starting_run()}
      </p>
    );
  }

  const { done, total, etaSeconds } = progress;
  // A run with no receivers is refused before it starts, so the guard is not
  // for a case that happens — it is for not dividing by a number this
  // component was handed rather than computed.
  const percent = total > 0 ? Math.round(Math.min(done / total, 1) * 100) : 0;
  const label = m.status_run_progress({ done, total });
  // No estimate at the very end: "about 1 s left" under a full bar is noise.
  const eta =
    etaSeconds !== undefined && done < total ? formatEta(etaSeconds) : null;

  return (
    <div className="space-y-1.5" data-testid="run-progress">
      <div
        role="progressbar"
        aria-label={m.label_run_progress()}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={done}
        aria-valuetext={eta === null ? label : `${label}, ${eta}`}
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
      >
        {/* `ease-linear`: the bar should move at the speed the receivers are
            computed at, and an ease-out would make every tick look like it
            was slowing down. */}
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-150 ease-linear"
          style={{ width: `${String(percent)}%` }}
        />
      </div>
      <p className="flex justify-between gap-2 text-xs text-muted-foreground">
        <span>{label}</span>
        {eta === null ? null : (
          <span data-testid="run-progress-eta">{eta}</span>
        )}
      </p>
    </div>
  );
}
