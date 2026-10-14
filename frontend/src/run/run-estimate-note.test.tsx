import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RunEstimate } from "@/model/run-estimate";
import type { RunEstimateState } from "@/api/hooks";
import { RunEstimateNote } from "./run-estimate-note";

/**
 * What the reader is told about the time a run will take. The arithmetic is
 * `model/run-estimate.test.ts`'s; what is checked here is that every state
 * says something, that the warning and its suggestions appear only for a long
 * run, and that a suggestion writes its parameters into the form.
 */

const EXTENT = { minX: 0, minY: 0, maxX: 1000, maxY: 1000 };
const SIZING = { resolutionM: 10, paddingM: 20 };

function estimate(wallS: number): RunEstimate {
  return {
    available: true,
    standard_id: "rls19-road",
    wall_s: { low: wallS * 0.8, estimate: wallS, high: wallS * 1.3 },
    cpu_s: { low: wallS * 6, estimate: wallS * 8, high: wallS * 10 },
    cpu_per_receiver_s: {
      low: (wallS * 6) / 11_025,
      estimate: (wallS * 8) / 11_025,
      high: (wallS * 10) / 11_025,
    },
    parallelism: 8,
    inputs: { receivers: 11_025, workers: 8, units: 44_000_000 },
    probe: {
      probe_receivers: 16,
      measured_receivers: 16,
      units_timed: 412,
      budget_s: 3,
      elapsed_s: 3,
      prepare_s: 0.4,
      budget_exhausted: true,
    },
    alternatives: [
      {
        params: { segment_length_mode: "distance_scaled" },
        measured: true,
        units: 1_200_000,
        cpu_s: { low: 300, estimate: 400, high: 600 },
        wall_s: { low: 40, estimate: 55, high: 80 },
      },
    ],
  };
}

function renderNote(
  state: RunEstimateState,
  onApplyParams: (params: Record<string, string>) => void = () => undefined,
) {
  render(
    <RunEstimateNote
      state={state}
      extent={EXTENT}
      sizing={SIZING}
      params={{}}
      onApplyParams={onApplyParams}
    />,
  );
}

describe("RunEstimateNote", () => {
  it("says nothing when nothing was asked", () => {
    renderNote({ status: "idle" });
    expect(screen.queryByTestId("run-estimate")).toBeNull();
  });

  it("says it is estimating without blocking anything", () => {
    renderNote({ status: "estimating" });
    expect(screen.getByTestId("run-estimate").textContent).toMatch(
      /estimating/i,
    );
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("says so when the estimate failed or the standard has no probe", () => {
    renderNote({ status: "failed" });
    expect(screen.getByTestId("run-estimate").textContent).toMatch(
      /could not be estimated/i,
    );
  });

  it("says so when the standard has no probe", () => {
    renderNote({
      status: "ready",
      estimate: { available: false, reason: "no estimate available" },
    });
    expect(screen.getByTestId("run-estimate").textContent).toMatch(
      /no run-time estimate/i,
    );
  });

  it("prints the band quietly for a short run", () => {
    renderNote({ status: "ready", estimate: estimate(420) });
    const note = screen.getByTestId("run-estimate");
    // 336–546 s, told in minutes and rounded outwards.
    expect(note.textContent).toContain("≈ 5–10 min");
    expect(note.textContent).toContain("11,025");
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("warns about a long run and applies a suggestion when clicked", async () => {
    const applied: Record<string, string>[] = [];
    renderNote({ status: "ready", estimate: estimate(3 * 3600) }, (params) => {
      applied.push(params);
    });

    const note = screen.getByTestId("run-estimate");
    expect(note.textContent).toContain("≈ 2–4 h");

    const scaled = screen.getByRole("button", {
      name: /distance-scaled teilstücke — ≈ 40–80 s/i,
    });
    await userEvent.click(scaled);
    expect(applied).toEqual([{ segment_length_mode: "distance_scaled" }]);

    const grid = screen.getByRole("button", { name: /m grid spacing/i });
    await userEvent.click(grid);
    expect(applied[1]?.grid_resolution_m).toMatch(/^\d+$/);
  });
});
