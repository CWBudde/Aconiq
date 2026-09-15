import { describe, expect, it } from "vitest";
import { EXPORT_FORMATS, RUN_ID_PLACEHOLDER, exportCommand } from "./cli";

describe("exportCommand", () => {
  it("names the run it was given", () => {
    expect(exportCommand("run-7")).toBe("aconiq export --run-id run-7");
  });

  it("stands in a placeholder rather than emitting a blank --run-id", () => {
    // `aconiq export` with no --run-id exports the *latest* run, so a blank id
    // would silently act on a different run than the one on screen.
    const expected = `aconiq export --run-id ${RUN_ID_PLACEHOLDER}`;

    expect(exportCommand("")).toBe(expected);
    expect(exportCommand("   ")).toBe(expected);
  });

  it("uses a placeholder no shell reads as an operator", () => {
    // The command is rendered in a copy field. `<run-id>` looks like a blank
    // to fill in and parses as redirection, so the pasted line dies in the
    // shell before `aconiq` runs at all.
    expect(RUN_ID_PLACEHOLDER).toMatch(/^[A-Za-z0-9_.-]+$/);
  });

  it("trims a run id rather than pasting whitespace into the command", () => {
    expect(exportCommand("  run-7 ")).toBe("aconiq export --run-id run-7");
  });

  it("omits --format when no format is asked for", () => {
    expect(exportCommand("run-7")).not.toContain("--format");
    expect(exportCommand("run-7", {})).not.toContain("--format");
    expect(exportCommand("run-7", { formats: [] })).not.toContain("--format");
  });

  it("joins formats with commas in the order given", () => {
    expect(exportCommand("run-7", { formats: ["geotiff", "gpkg"] })).toBe(
      "aconiq export --run-id run-7 --format geotiff,gpkg",
    );
  });

  it("emits every format the CLI declares", () => {
    // The tuple is the contract with `export.go`'s --format list; a member it
    // could not spell would be a command the CLI rejects.
    for (const format of EXPORT_FORMATS) {
      expect(exportCommand("run-7", { formats: [format] })).toBe(
        `aconiq export --run-id run-7 --format ${format}`,
      );
    }
  });

  it("adds --pdf only when asked", () => {
    expect(exportCommand("run-7", { pdf: true })).toBe(
      "aconiq export --run-id run-7 --pdf",
    );
    expect(exportCommand("run-7", { pdf: false })).not.toContain("--pdf");
    expect(exportCommand("run-7", {})).not.toContain("--pdf");
  });

  it("orders flags as `aconiq export --help` lists them", () => {
    // --run-id, --format, --pdf: the registration order in export.go, so a
    // user comparing the copied line against the CLI's own help reads the
    // same sequence.
    expect(exportCommand("r1", { formats: ["cog"], pdf: true })).toBe(
      "aconiq export --run-id r1 --format cog --pdf",
    );
  });
});
