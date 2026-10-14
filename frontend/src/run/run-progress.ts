import type { RunSummary } from "@/api/client";
import type { RunProgress } from "@/api/hooks";
import { m } from "@/i18n/messages";

/**
 * A running run's progress as the server reported it, in the shape the bar
 * draws, or `null` when there is none yet — the run is still loading its
 * model, or its standard does not count receivers as it goes.
 */
export function progressOfRun(run: RunSummary): RunProgress | null {
  const report = run.progress;
  if (report === undefined) return null;
  return {
    done: report.done,
    total: report.total,
    ...(report.eta_seconds === undefined
      ? {}
      : { etaSeconds: report.eta_seconds }),
  };
}

/**
 * A remaining time as a person reads it: seconds under a minute, whole
 * minutes under an hour, hours and minutes beyond. Rounded up, because "0 s
 * left" on a run that is still computing reads as a bar that has stalled.
 */
export function formatEta(seconds: number): string {
  const whole = Math.max(Math.ceil(seconds), 1);
  if (whole < 60) return m.status_run_eta_seconds({ seconds: whole });
  const minutes = Math.ceil(whole / 60);
  if (minutes < 60) return m.status_run_eta_minutes({ minutes });
  return m.status_run_eta_hours({
    hours: Math.floor(minutes / 60),
    minutes: minutes % 60,
  });
}
