import { describe, expect, it } from "vitest";
import type { RunSummary } from "@/api/client";
import { m } from "@/i18n/messages";
import { formatEta, progressOfRun } from "./run-progress";

function run(progress?: RunSummary["progress"]): RunSummary {
  return {
    id: "run-1",
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    status: "running",
    started_at: "2026-10-13T12:00:00Z",
    finished_at: "2026-10-13T12:00:00Z",
    log_path: ".noise/runs/run-1/run.log",
    artifacts: [],
    ...(progress === undefined ? {} : { progress }),
  };
}

describe("progressOfRun", () => {
  it("reads the server's report, estimate included", () => {
    expect(
      progressOfRun(
        run({
          done: 12,
          total: 130,
          started_at: "2026-10-13T12:00:01Z",
          updated_at: "2026-10-13T12:04:01Z",
          eta_seconds: 2360,
        }),
      ),
    ).toEqual({ done: 12, total: 130, etaSeconds: 2360 });
  });

  it("leaves the estimate out before there is a rate to extrapolate", () => {
    expect(
      progressOfRun(
        run({
          done: 0,
          total: 130,
          started_at: "2026-10-13T12:00:01Z",
          updated_at: "2026-10-13T12:00:01Z",
        }),
      ),
    ).toEqual({ done: 0, total: 130 });
  });

  it("is null, not zero, when there is no report", () => {
    // "Not known" must not be drawn as "nothing done".
    expect(progressOfRun(run())).toBeNull();
  });
});

describe("formatEta", () => {
  it("rounds up, so a running run never claims no time left", () => {
    expect(formatEta(0.2)).toBe(m.status_run_eta_seconds({ seconds: 1 }));
    expect(formatEta(59)).toBe(m.status_run_eta_seconds({ seconds: 59 }));
  });

  it("switches to minutes, then to hours and minutes", () => {
    expect(formatEta(60)).toBe(m.status_run_eta_minutes({ minutes: 1 }));
    expect(formatEta(61)).toBe(m.status_run_eta_minutes({ minutes: 2 }));
    expect(formatEta(3540)).toBe(m.status_run_eta_minutes({ minutes: 59 }));
    expect(formatEta(3599)).toBe(
      m.status_run_eta_hours({ hours: 1, minutes: 0 }),
    );
    expect(formatEta(3601)).toBe(
      m.status_run_eta_hours({ hours: 1, minutes: 1 }),
    );
  });
});
