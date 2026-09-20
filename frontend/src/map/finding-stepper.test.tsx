import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { m } from "@/i18n/messages";
import { FindingStepper } from "./finding-stepper";

function renderStepper(
  position = 3,
  total = 608,
  onSignOff: (() => void) | null = null,
) {
  const onStep = vi.fn<(direction: 1 | -1) => void>();
  const onClose = vi.fn();
  render(
    <FindingStepper
      position={position}
      total={total}
      onStep={onStep}
      onSignOff={onSignOff}
      onClose={onClose}
    />,
  );
  return { onStep, onClose };
}

describe("FindingStepper", () => {
  it("says where in the queue the reader is", () => {
    renderStepper();
    expect(
      screen.getByText(m.label_finding_position({ position: 3, total: 608 })),
    ).toBeInTheDocument();
  });

  it("steps forwards and backwards", () => {
    const { onStep } = renderStepper();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_next_finding() }),
    );
    expect(onStep).toHaveBeenCalledWith(1);

    fireEvent.click(
      screen.getByRole("button", { name: m.action_previous_finding() }),
    );
    expect(onStep).toHaveBeenCalledWith(-1);
  });

  it("stops stepping on request", () => {
    const { onClose } = renderStepper();
    fireEvent.click(
      screen.getByRole("button", { name: m.action_stop_stepping() }),
    );
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("offers a sign-off only when the finding under the cursor has one", () => {
    // Most findings are defects. A greyed-out "accept this" against a
    // self-intersecting line would suggest the app can wave one through, so the
    // button is absent rather than disabled.
    renderStepper();
    expect(
      screen.queryByRole("button", {
        name: m.action_mark_reviewed_and_next(),
      }),
    ).not.toBeInTheDocument();

    const onSignOff = vi.fn();
    renderStepper(3, 608, onSignOff);
    fireEvent.click(
      screen.getByRole("button", { name: m.action_mark_reviewed_and_next() }),
    );
    expect(onSignOff).toHaveBeenCalledTimes(1);
  });

  it("announces each step, and names its keyboard shortcuts", () => {
    // `role="status"` is what makes the counter reach a screen reader on every
    // step; without it the position changes silently.
    renderStepper();
    expect(screen.getByRole("status")).toHaveAccessibleName(m.label_findings());
    expect(
      screen.getByRole("button", { name: m.action_next_finding() }),
    ).toHaveAttribute("aria-keyshortcuts", "Alt+ArrowDown");
    expect(
      screen.getByRole("button", { name: m.action_previous_finding() }),
    ).toHaveAttribute("aria-keyshortcuts", "Alt+ArrowUp");
  });
});
