import { describe, expect, it } from "vitest";
import { failureReason } from "./parse-timeline";

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
