import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/ui/components/tooltip";
import { UndoRedoBar } from "./undo-redo-bar";
import { isTextEntryTarget } from "./event-utils";
import { useModelStore } from "@/model/model-store";
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
    <TooltipProvider>
      <UndoRedoBar />
      <input aria-label="height" defaultValue="5" />
    </TooltipProvider>,
  );
}

describe("isTextEntryTarget", () => {
  it("recognises the controls that own their undo history", () => {
    expect(isTextEntryTarget(document.createElement("input"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("textarea"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("select"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("div"))).toBe(false);
    expect(isTextEntryTarget(null)).toBe(false);
  });
});

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
