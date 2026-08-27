import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { NewFeatureDialog } from "./new-feature-dialog";
import { useModelStore } from "@/model/model-store";
import type { Geometry } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The receiver branch of this dialog had never been exercised: `option_receiver`
 * was simply missing from messages/{en,de}.json until recently, which no test
 * and no type check would have caught. These cover the whole `kind` switch —
 * source, building, barrier and receiver — so the next missing case fails here.
 */

const point: Geometry = { type: "Point", coordinates: [10, 51] };
const polygon: Geometry = {
  type: "Polygon",
  coordinates: [
    [
      [0, 0],
      [1, 0],
      [1, 1],
      [0, 1],
      [0, 0],
    ],
  ],
};

beforeEach(() => {
  useModelStore.getState().reset();
});

function renderDialog(geometry: Geometry, onClose = vi.fn()) {
  render(
    <NewFeatureDialog open={true} geometry={geometry} onClose={onClose} />,
  );
  return onClose;
}

/**
 * The first combobox is the kind picker; a second one (source type) appears
 * alongside it whenever the chosen kind is "source".
 */
function openKindPicker() {
  const [trigger] = screen.getAllByRole("combobox");
  if (!trigger) throw new Error("no kind picker rendered");
  fireEvent.click(trigger);
}

function selectKind(kind: string) {
  // Radix Select renders its listbox in a portal, so the option is only in the
  // document once the trigger has been clicked.
  openKindPicker();
  fireEvent.click(screen.getByRole("option", { name: kind }));
}

describe("NewFeatureDialog", () => {
  it("offers every feature kind, and receiver only for point geometry", () => {
    renderDialog(point);
    openKindPicker();

    expect(
      screen.getByRole("option", { name: m.option_source() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: m.option_building() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: m.option_barrier() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: m.option_receiver() }),
    ).toBeInTheDocument();
  });

  it("hides the receiver option for non-point geometry", () => {
    renderDialog(polygon);
    openKindPicker();

    expect(
      screen.queryByRole("option", { name: m.option_receiver() }),
    ).not.toBeInTheDocument();
  });

  it("has a translated label for every kind option", () => {
    // A missing message key compiles to a function returning the key name;
    // catching that here is what the `option_receiver` gap needed.
    for (const label of [
      m.option_source(),
      m.option_building(),
      m.option_barrier(),
      m.option_receiver(),
    ]) {
      expect(label).not.toMatch(/^option_/);
      expect(label.trim()).not.toBe("");
    }
  });

  it("adds a receiver, not a feature, when the receiver kind is chosen", () => {
    const onClose = renderDialog(point);

    selectKind(m.option_receiver());
    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    const state = useModelStore.getState();
    expect(state.receivers).toHaveLength(1);
    expect(state.features).toHaveLength(0);
    // Receivers default to 4 m, features to 5 m.
    expect(state.receivers[0]?.heightM).toBe(4);
    expect(state.receivers[0]?.geometry).toEqual(point);
    expect(onClose).toHaveBeenCalled();
  });

  it("adds a source for point geometry by default", () => {
    renderDialog(point);
    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    const state = useModelStore.getState();
    expect(state.receivers).toHaveLength(0);
    expect(state.features).toHaveLength(1);
    expect(state.features[0]?.kind).toBe("source");
    expect(state.features[0]?.sourceType).toBe("point");
  });

  it("adds a building with a height for polygon geometry", () => {
    renderDialog(polygon);
    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    const state = useModelStore.getState();
    expect(state.features).toHaveLength(1);
    expect(state.features[0]?.kind).toBe("building");
    expect(state.features[0]?.heightM).toBe(5);
  });
});
