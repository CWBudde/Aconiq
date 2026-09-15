import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ModeGate } from "./mode-gate";
import { Button } from "@/ui/components/button";

const state = vi.hoisted(() => ({ canExport: false }));

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.canExport ? "browser" : "http",
        canExport: state.canExport,
        runsAgainstSavedModel: !state.canExport,
        runsChangeExternally: !state.canExport,
      };
    },
  },
}));

const onClick = vi.fn();

/**
 * Rendered with no `TooltipProvider` around it. That absence is itself an
 * assertion: pages are rendered standalone in unit tests, Radix's tooltip
 * context has no default value, and a `ModeGate` that relied on a provider
 * further up would throw in every one of them.
 */
function renderGate(canExport: boolean) {
  state.canExport = canExport;
  render(
    <ModeGate capability="canExport" reason="No bundles here.">
      <Button onClick={onClick}>Generate</Button>
    </ModeGate>,
  );
  return screen.getByRole("button", { name: "Generate" });
}

beforeEach(() => {
  onClick.mockClear();
});

describe("ModeGate", () => {
  it("renders the control untouched when the capability is present", async () => {
    const button = renderGate(true);

    expect(button).not.toHaveAttribute("aria-disabled");
    expect(button).not.toHaveAttribute("data-mode-gated");
    await userEvent.click(button);
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("marks the control unavailable without removing it", () => {
    const button = renderGate(false);

    expect(button).toHaveAttribute("aria-disabled", "true");
    // Not the real attribute: `disabled` takes the element out of the focus
    // order and kills its pointer events, which is what silences the tooltip.
    expect(button).not.toBeDisabled();
  });

  it("swallows activation while gated", async () => {
    const button = renderGate(false);

    await userEvent.click(button);
    expect(onClick).not.toHaveBeenCalled();
  });

  it("shows the reason on hover", async () => {
    // The assertion that matters. Every DOM-shaped check above still passes
    // when the tooltip never opens — which is exactly what a `disabled`
    // trigger does, and what makes `undo-redo-bar.tsx`'s tooltips silent.
    const button = renderGate(false);

    await userEvent.hover(button);
    expect(await screen.findAllByText("No bundles here.")).not.toHaveLength(0);
  });

  it("shows the reason on keyboard focus", async () => {
    const button = renderGate(false);

    await userEvent.tab();
    expect(button).toHaveFocus();
    expect(await screen.findAllByText("No bundles here.")).not.toHaveLength(0);
  });

  it("points the control at the reason for assistive technology", async () => {
    const button = renderGate(false);
    await userEvent.hover(button);
    // Radix wires `aria-describedby` when the tooltip opens, which is after
    // the delay — reading the attribute straight after the hover finds nothing.
    await screen.findAllByText("No bundles here.");

    const describedBy = button.getAttribute("aria-describedby");
    expect(describedBy).not.toBeNull();
    expect(document.getElementById(describedBy ?? "")).toHaveTextContent(
      "No bundles here.",
    );
  });
});
