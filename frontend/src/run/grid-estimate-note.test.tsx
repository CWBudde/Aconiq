import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { gridShape, RECEIVER_WARNING_THRESHOLD } from "@/model/grid-estimate";
import type { GridExtentState } from "./use-grid-extent";
import { GridEstimateNote } from "./grid-estimate-note";

/**
 * What the reader is told about the grid they are about to compute. The
 * numbers are `model/grid-estimate.test.ts`'s; what is checked here is that
 * each state says something, that the warning appears where it should and
 * nowhere else, and that the offered resolution is one that actually helps.
 */

/** A 2 km square: 205 × 205 = 42 025 receivers at the default 10 m. */
const BIG: GridExtentState = {
  status: "ready",
  extent: { minX: 0, minY: 0, maxX: 2000, maxY: 2000 },
};

/** A 200 × 100 m lot: 25 × 15 = 375 receivers at the default 10 m. */
const SMALL: GridExtentState = {
  status: "ready",
  extent: { minX: 0, minY: 0, maxX: 200, maxY: 100 },
};

const DEFAULTS = { grid_resolution_m: "10", grid_padding_m: "20" };

function renderNote(
  state: GridExtentState,
  params: Record<string, string> = DEFAULTS,
  onUseResolution: (resolutionM: number) => void = () => undefined,
) {
  render(
    <GridEstimateNote
      state={state}
      params={params}
      onUseResolution={onUseResolution}
    />,
  );
}

describe("GridEstimateNote", () => {
  it("prints the shape and the count for a grid that is not large", () => {
    renderNote(SMALL);
    const note = screen.getByTestId("grid-estimate");
    expect(note.textContent).toContain("375");
    expect(note.textContent).toContain("25");
    expect(note.textContent).toContain("15");
    // Not a warning: 375 receivers is nothing to interrupt anyone about.
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("warns above the threshold and offers a resolution that clears it", async () => {
    const chosen: number[] = [];
    renderNote(BIG, DEFAULTS, (resolutionM) => {
      chosen.push(resolutionM);
    });

    const note = screen.getByTestId("grid-estimate");
    expect(note.getAttribute("data-variant")).toBe("warning");

    await userEvent.click(screen.getByRole("button"));
    expect(chosen).toHaveLength(1);

    // The offer has to be an offer: the grid it produces must be under the
    // threshold, or the button moves the reader nowhere.
    const relieved = gridShape(
      { minX: 0, minY: 0, maxX: 2000, maxY: 2000 },
      { resolutionM: chosen[0] as number, paddingM: 20 },
    );
    expect(relieved?.count).toBeLessThanOrEqual(RECEIVER_WARNING_THRESHOLD);
  });

  it("follows the parameters it is given", () => {
    // The same extent that warned at 10 m goes quiet at 50 m. This is the
    // reaction the field is supposed to produce as the reader types.
    renderNote(BIG, { grid_resolution_m: "50", grid_padding_m: "20" });
    expect(
      screen.getByTestId("grid-estimate").getAttribute("data-variant"),
    ).toBeNull();
  });

  it("says a resolution of zero describes no grid", () => {
    renderNote(BIG, { grid_resolution_m: "0", grid_padding_m: "20" });
    expect(screen.getByTestId("grid-estimate").textContent).not.toBe("");
    expect(screen.queryByRole("button")).toBeNull();
  });

  it.each([["projecting"], ["no-extent"], ["failed"]] as const)(
    "says something while %s",
    (status) => {
      renderNote({ status });
      expect(screen.getByTestId("grid-estimate").textContent.trim()).not.toBe(
        "",
      );
    },
  );

  it.each([["idle"], ["unavailable"]] as const)(
    "renders nothing while %s",
    (status) => {
      // Silence rather than a placeholder: there is no metric extent to count
      // cells over, and a row of dashes would only ask the reader why.
      renderNote({ status });
      expect(screen.queryByTestId("grid-estimate")).toBeNull();
    },
  );

  it("does not fire a live region on every keystroke", () => {
    // The count follows the resolution field as it is typed. A warning
    // callout defaults to `role="alert"`, which would interrupt a screen
    // reader once per character.
    renderNote(BIG);
    expect(screen.getByTestId("grid-estimate").getAttribute("role")).toBe(
      "group",
    );
  });
});
