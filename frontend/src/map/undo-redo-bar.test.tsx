import { beforeEach, describe, expect, it } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/ui/components/tooltip";
import { UndoRedoBar } from "./undo-redo-bar";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";
import type { ModelFeature } from "@/model/types";

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

beforeEach(() => {
  useModelStore.getState().reset();
});

function renderBar() {
  render(
    <TooltipProvider delayDuration={0}>
      <UndoRedoBar />
      <input aria-label="height" defaultValue="5" />
    </TooltipProvider>,
  );
}

function button(name: string): HTMLElement {
  return screen.getByRole("button", { name });
}

describe("UndoRedoBar keyboard shortcut", () => {
  it("undoes a model edit when the shortcut fires outside a text field", () => {
    renderBar();
    useModelStore.getState().addFeature(source);
    expect(useModelStore.getState().features).toHaveLength(1);

    fireEvent.keyDown(window, { key: "z", ctrlKey: true });

    expect(useModelStore.getState().features).toHaveLength(0);
  });

  it("leaves the model alone when the shortcut fires inside a text field", () => {
    renderBar();
    useModelStore.getState().addFeature(source);

    // The regression: this global handler hijacked Ctrl+Z from every focused
    // input, so undoing a typo in the feature editor reverted a map edit
    // instead — and the typed text could not be recovered.
    fireEvent.keyDown(screen.getByLabelText("height"), {
      key: "z",
      ctrlKey: true,
    });

    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("does not redo from inside a text field either", () => {
    renderBar();
    useModelStore.getState().addFeature(source);
    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toHaveLength(0);

    fireEvent.keyDown(screen.getByLabelText("height"), {
      key: "y",
      ctrlKey: true,
    });
    expect(useModelStore.getState().features).toHaveLength(0);

    fireEvent.keyDown(window, { key: "y", ctrlKey: true });
    expect(useModelStore.getState().features).toHaveLength(1);
  });
});

/**
 * The bar rests with nothing to undo and nothing to redo, which is exactly the
 * state both buttons have to stay reachable in.
 *
 * `disabled` here was never a user-visible bug — both tooltips render text
 * byte-identical to the button's own `aria-label`, so a screen reader lost
 * nothing, and a sighted mouse user lost the "(Ctrl+Z)" hint in a state where
 * the shortcut would do nothing anyway. It was a trap: the first real reason
 * put in one of these tooltips would have been unreachable.
 *
 * What pins the fix is `aria-disabled` plus Tab-reachability, not the hover.
 * jsdom computes no Tailwind stylesheet, so `disabled:pointer-events-none`
 * from `ui/components/button.tsx` is never actually applied here and a hover
 * would pass over a real `disabled` button too — the same reasoning
 * `map/draw-toolbar.test.tsx` spells out at length.
 */
describe("UndoRedoBar when an action is unavailable", () => {
  it("marks both buttons refused without making them unreachable", () => {
    renderBar();

    for (const label of [m.tooltip_undo(), m.tooltip_redo()]) {
      expect(button(label)).toHaveAttribute("aria-disabled", "true");
      expect(button(label)).not.toBeDisabled();
    }
  });

  it("accepts no click while refused", async () => {
    renderBar();
    useModelStore.getState().addFeature(source);

    // Redo is refused here: an edit was made and nothing was undone. A real
    // click, not a dispatched one — `aria-disabled` does not stop activation
    // by itself, so the handler has to swallow it.
    await userEvent.click(button(m.tooltip_redo()));

    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("is reachable by keyboard, so the tooltip is not mouse-only", async () => {
    renderBar();

    await userEvent.tab();

    // A `disabled` button is skipped by Tab entirely; this is the assertion
    // that catches a regression to one.
    expect(button(m.tooltip_undo())).toHaveFocus();

    await userEvent.tab();
    expect(button(m.tooltip_redo())).toHaveFocus();
  });

  it("opens the undo tooltip in the state that used to kill it", async () => {
    renderBar();

    await userEvent.hover(button(m.tooltip_undo()));

    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      m.tooltip_undo(),
    );
  });

  it("drops the refused marking once an edit can be undone", () => {
    renderBar();
    // Inside `act`, because this assertion reads what the bar rendered rather
    // than what the store holds, and the edit has to reach React first.
    act(() => {
      useModelStore.getState().addFeature(source);
    });

    expect(button(m.tooltip_undo())).not.toHaveAttribute("aria-disabled");
    expect(button(m.tooltip_redo())).toHaveAttribute("aria-disabled", "true");
  });
});

describe("UndoRedoBar buttons", () => {
  it("undoes and redoes by click, not only by shortcut", async () => {
    renderBar();
    useModelStore.getState().addFeature(source);

    await userEvent.click(button(m.tooltip_undo()));
    expect(useModelStore.getState().features).toHaveLength(0);

    await userEvent.click(button(m.tooltip_redo()));
    expect(useModelStore.getState().features).toHaveLength(1);
  });
});
