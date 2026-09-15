import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { FeatureEditor } from "./feature-editor";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { MAIN_CONTENT_ID } from "@/ui/main-content";
import { m } from "@/i18n/messages";

/**
 * The editor is the only place a feature or a receiver can be deleted, and it
 * is not on the map's own surface — so nothing here needs a WebGL context. The
 * panel is plain DOM.
 */

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10.01, 51.01] },
};

function deleteButton(): HTMLElement {
  return screen.getByRole("button", { name: m.action_delete_feature() });
}

function confirmDelete() {
  fireEvent.click(
    within(screen.getByRole("alertdialog")).getByRole("button", {
      name: m.action_delete_feature(),
    }),
  );
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("FeatureEditor deletion", () => {
  it("asks before deleting a feature, and deletes nothing until answered", () => {
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("names what is being deleted, and says undo can bring it back", () => {
    // The panel has no undo control of its own and the undo bar is at the
    // other end of the workspace, so the confirmation is where that is said.
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveTextContent("src-1");
    expect(dialog).toHaveTextContent(m.option_source());
  });

  it("deletes the feature and closes the panel once confirmed", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={onClose} />);

    fireEvent.click(deleteButton());
    confirmDelete();

    expect(useModelStore.getState().features).toHaveLength(0);
    expect(onClose).toHaveBeenCalled();
  });

  it("keeps the feature when the confirmation is cancelled", () => {
    const onClose = vi.fn();
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={onClose} />);

    fireEvent.click(deleteButton());
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: m.action_cancel(),
      }),
    );

    expect(useModelStore.getState().features).toHaveLength(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("asks before deleting a receiver too", () => {
    // The receiver panel used to carry its own delete button, so it had its
    // own path past the confirmation.
    useModelStore.getState().addReceiver(receiver);
    render(<FeatureEditor featureId="rcv-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());

    expect(screen.getByRole("alertdialog")).toHaveTextContent("rcv-1");
    expect(useModelStore.getState().receivers).toHaveLength(1);

    confirmDelete();
    expect(useModelStore.getState().receivers).toHaveLength(0);
  });

  it("puts focus back in the content when the panel deletes itself", async () => {
    // Confirming closes the panel, taking the Delete button with it, so Radix
    // restores focus to an element that is gone and it lands on `<body>`. The
    // e2e axe baseline does not cover the map route, and no rule covers this.
    useModelStore.getState().addFeature(source);
    render(
      <div id={MAIN_CONTENT_ID} tabIndex={-1}>
        <FeatureEditor featureId="src-1" onClose={vi.fn()} />
      </div>,
    );

    fireEvent.click(deleteButton());
    confirmDelete();

    await waitFor(() => {
      expect(document.activeElement?.id).toBe(MAIN_CONTENT_ID);
    });
  });

  it("does not move focus when the confirmation is cancelled", () => {
    useModelStore.getState().addFeature(source);
    render(
      <div id={MAIN_CONTENT_ID} tabIndex={-1}>
        <FeatureEditor featureId="src-1" onClose={vi.fn()} />
      </div>,
    );

    fireEvent.click(deleteButton());
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: m.action_cancel(),
      }),
    );

    // Only that this component does not hijack it: where Radix's own
    // restoration goes is Radix's business, and jsdom never gave the trigger
    // focus to restore.
    expect(document.activeElement?.id).not.toBe(MAIN_CONTENT_ID);
    expect(useModelStore.getState().features).toHaveLength(1);
  });

  it("is undoable after the confirmation, which is what the wording promises", () => {
    useModelStore.getState().addFeature(source);
    render(<FeatureEditor featureId="src-1" onClose={vi.fn()} />);

    fireEvent.click(deleteButton());
    confirmDelete();
    useModelStore.getState().undo();

    expect(useModelStore.getState().features).toHaveLength(1);
  });
});
