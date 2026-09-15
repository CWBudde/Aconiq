import type { RunStatus } from "@/ui/status-badge";
import { m } from "@/i18n/messages";

/**
 * The run pipeline as a timeline, recovered from the run log.
 *
 * Deliberately a parser rather than a contract: the CLI writes the log, the API
 * serves it verbatim, and neither promises a machine-readable progress feed. So
 * the stages below are the fixed sequence a run goes through and the patterns
 * are how each one announces itself; a log line nobody recognises simply leaves
 * its stage pending.
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
      pattern: /stage=compute/,
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

/** What the step's marker says out loud. */
export function stepStateLabel(step: TimelineStep): string {
  if (step.done) return m.timeline_status_done();
  if (step.active) return m.timeline_status_active();
  return m.timeline_status_pending();
}
