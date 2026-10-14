import { describe, expect, it } from "vitest";
import { failureReason, parseTimeline } from "./parse-timeline";

describe("failureReason", () => {
  it("names the last cause before the closing line", () => {
    expect(
      failureReason([
        "2026-10-13T22:37:19Z run started",
        "2026-10-13T22:37:45Z run refused: custom receiver mode is not supported",
        "2026-10-13T22:37:45Z run failed",
      ]),
    ).toBe("run refused: custom receiver mode is not supported");
  });

  it("takes the server's note for a child it had to close", () => {
    expect(
      failureReason([
        "2026-10-13T22:21:16Z computed 0 of 130 receivers",
        "2026-10-13T22:21:29Z run interrupted: the server that started it shut down before it recorded a result",
      ]),
    ).toBe(
      "run interrupted: the server that started it shut down before it recorded a result",
    );
  });

  it("skips progress lines, which say how far a run got and not why it stopped", () => {
    expect(
      failureReason([
        "2026-10-13T22:15:00Z rls19 compute failed: boom",
        "2026-10-13T22:15:07Z computed 11 of 130 receivers",
        "2026-10-13T22:15:08Z run failed",
      ]),
    ).toBe("rls19 compute failed: boom");
  });

  it("is null when the log names nothing", () => {
    expect(failureReason([])).toBeNull();
    expect(failureReason(["2026-10-13T22:15:08Z run failed"])).toBeNull();
  });
});

describe("parseTimeline", () => {
  const before = [
    "2026-10-14T19:06:18Z run started",
    "2026-10-14T19:06:18Z model=.noise/model/model.normalized.geojson",
    "2026-10-14T19:06:19Z road_sources=2",
    "2026-10-14T19:06:19Z receivers=230961 grid=501x461",
  ];

  const step = (lines: string[], index: number) =>
    parseTimeline(lines, "running")[index];

  // The first progress line is written as the walk starts, so it marks the
  // compute step under way, not behind the run.
  it("keeps the compute step active until the last receiver is counted", () => {
    const computing = [
      ...before,
      "2026-10-14T19:06:19Z computed 0 of 230961 receivers",
      "2026-10-14T19:06:20Z computed 53130 of 230961 receivers",
    ];

    expect(step(computing, 4)).toMatchObject({ done: false, active: true });
    expect(step(computing, 5)).toMatchObject({ done: false, active: false });
  });

  it("marks the compute step done at the line that counts every receiver", () => {
    const computed = [
      ...before,
      "2026-10-14T19:06:19Z computed 0 of 230961 receivers",
      "2026-10-14T19:06:29Z computed 230961 of 230961 receivers",
    ];

    expect(step(computed, 4)).toMatchObject({
      done: true,
      timestamp: "2026-10-14T19:06:29Z",
    });
    expect(step(computed, 5)).toMatchObject({ done: false, active: true });
  });

  // "computed 2 of 12" ends in the same digits as a finished count would.
  it("does not read a partial count as a finished one", () => {
    const partial = [
      ...before,
      "2026-10-14T19:06:20Z computed 2 of 12 receivers",
      "2026-10-14T19:06:20Z computed 12 of 120 receivers",
    ];

    expect(step(partial, 4)).toMatchObject({ done: false, active: true });
  });
});
