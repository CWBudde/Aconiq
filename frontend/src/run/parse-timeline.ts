import type { RunStatus } from "@/ui/status-badge";
import { m } from "@/i18n/messages";

/**
 * The run pipeline as a timeline, recovered from the run log.
 *
 * Deliberately a parser rather than a contract: the CLI writes the log and the
 * API serves it verbatim. The stages below are the fixed sequence a run goes
 * through and the patterns are how each one announces itself; a log line
 * nobody recognises simply leaves its stage pending.
 *
 * The log is written as the run goes, so in API mode a running run's timeline
 * moves while it computes rather than only once it has finished. The
 * machine-readable progress feed is not this: it is the run's `progress`
 * (`RunSummary.progress`, read from `progress.json`), which `RunProgressBar`
 * draws. The "computed X of Y receivers" lines that same feed writes into the
 * log are what marks the compute stage here.
 */

export interface TimelineStep {
  label: string;
  timestamp?: string;
  done: boolean;
  active: boolean;
}

const LOG_STAGES: Array<{ key: string; pattern: RegExp; label: () => string }> =
  [
    { key: "started", pattern: /run started/, label: m.timeline_run_started },
    { key: "model", pattern: /model=/, label: m.timeline_loading_model },
    {
      key: "sources",
      pattern: /(?:sources|road_sources)=\d+/,
      label: m.timeline_extracting_sources,
    },
    {
      key: "receivers",
      pattern: /receivers=\d+/,
      label: m.timeline_building_receivers,
    },
    {
      key: "compute",
      // The engine's stage events, or the receiver-count progress lines the
      // standards that walk receivers themselves (RLS-19, ISO 9613) write.
      pattern: /stage=compute|computed \d+ of \d+ receivers/,
      label: m.timeline_computing,
    },
    {
      key: "persist",
      pattern: /(?:output_hash=|persisted=)/,
      label: m.timeline_persisting_outputs,
    },
    {
      key: "done",
      pattern: /run (?:completed|failed)/,
      label: m.timeline_finalised,
    },
  ];

export function parseTimeline(
  lines: string[],
  status: RunStatus,
): TimelineStep[] {
  const matched = new Map<string, string>();

  for (const line of lines) {
    const ts = line.slice(0, 20);
    for (const stage of LOG_STAGES) {
      if (!matched.has(stage.key) && stage.pattern.test(line)) {
        matched.set(stage.key, ts);
      }
    }
  }

  const steps: TimelineStep[] = LOG_STAGES.map((stage, i) => {
    const ts = matched.get(stage.key);
    const done = matched.has(stage.key);
    const prevStage = LOG_STAGES[i - 1];
    const nextStage = LOG_STAGES[i + 1];
    const active =
      !done &&
      status === "running" &&
      (!prevStage || matched.has(prevStage.key)) &&
      (!nextStage || !matched.has(nextStage.key));
    return {
      label: stage.label(),
      ...(ts !== undefined && { timestamp: ts }),
      done,
      active,
    };
  });

  return steps;
}

/** The log's own timestamp prefix, e.g. `2026-10-13T22:37:45Z `. */
const LOG_TIMESTAMP = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z /;

/** Lines that close a failed run without saying why it failed. */
const NOT_A_REASON = /^(?:run failed|computed \d+ of \d+ receivers)$/;

/**
 * Why a failed run failed, as its log says it: the last line before the
 * closing "run failed" that names a cause — "run cancelled: …",
 * "run refused: …", "run interrupted: …", "<module> compute failed: …".
 *
 * A run that fails after `aconiq serve` has answered 202 has no response left
 * to carry the reason, so the log is where it is, and this is what lets the
 * run panel say it rather than leaving the reader to scroll for it. `null`
 * when the log names nothing.
 */
export function failureReason(lines: string[]): string | null {
  for (let i = lines.length - 1; i >= 0; i--) {
    const message = (lines[i] ?? "").replace(LOG_TIMESTAMP, "").trim();
    if (message === "" || NOT_A_REASON.test(message)) continue;
    return message;
  }
  return null;
}

/** What the step's marker says out loud. */
export function stepStateLabel(step: TimelineStep): string {
  if (step.done) return m.timeline_status_done();
  if (step.active) return m.timeline_status_active();
  return m.timeline_status_pending();
}
