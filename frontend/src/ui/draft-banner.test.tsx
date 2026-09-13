import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DraftBanner } from "./draft-banner";
import { useModelStore } from "@/model/model-store";
import { DRAFT_KEY } from "@/model/use-autosave";
import {
  projectHydrationStore,
  resetProjectHydration,
} from "@/model/use-project-hydration";
import type { ModelFeature } from "@/model/types";

/**
 * The banner no longer decides whether a draft is worth offering — the
 * hydration hook does, and these drive it through that store. The decision
 * moved because a child effect runs before its parent's: asked here, the
 * question was answered before the project had been looked at, and could only
 * ever say yes to a draft over an empty map.
 */

const sampleFeature: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

/** Hydration has run and decided the draft is worth offering. */
function offerDraft() {
  projectHydrationStore.setState({
    started: true,
    status: "settled",
    draftOffered: true,
  });
}

beforeEach(() => {
  localStorage.clear();
  useModelStore.getState().reset();
  resetProjectHydration();
});

describe("DraftBanner", () => {
  it("renders nothing when the hydration store offers no draft", () => {
    projectHydrationStore.setState({ started: true, draftOffered: false });
    const { container } = render(<DraftBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing before the hydration decision has been made", () => {
    // A draft that turns out to be the project's own model is restored
    // silently; showing the offer first would flash a choice that is about to
    // be withdrawn.
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    resetProjectHydration();
    expect(projectHydrationStore.getState().draftOffered).toBe(true);

    const { container } = render(<DraftBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("offers no draft when the model already holds features at startup", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    useModelStore.getState().loadFeatures([sampleFeature]);
    resetProjectHydration();

    expect(projectHydrationStore.getState().draftOffered).toBe(false);
    const { container } = render(<DraftBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("offers a draft found over an empty model at startup", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    resetProjectHydration();

    expect(projectHydrationStore.getState().draftOffered).toBe(true);
  });

  it("shows recovery banner when the draft is offered", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    offerDraft();
    render(<DraftBanner />);
    expect(screen.getByText(/unsaved draft found/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /restore/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /discard/i }),
    ).toBeInTheDocument();
  });

  it("still offers a draft that diverges from a hydrated model", () => {
    // The case the old mount rule could never reach: after a hash mismatch
    // the store is populated from the project, and the draft is exactly the
    // work the user would otherwise lose.
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    offerDraft();
    useModelStore.getState().hydrateModel({
      features: [sampleFeature],
      receivers: [],
      calcArea: null,
    });

    render(<DraftBanner />);
    expect(screen.getByText(/unsaved draft found/i)).toBeInTheDocument();
  });

  it("restores features and keeps the draft on Restore", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    offerDraft();
    render(<DraftBanner />);
    fireEvent.click(screen.getByRole("button", { name: /restore/i }));
    expect(useModelStore.getState().features).toHaveLength(1);
    // A restored model is content the project has not seen; leaving the
    // draft in place is what keeps it recoverable if the next autosave never
    // runs (the earlier discard here lost it on the following reload).
    expect(useModelStore.getState().dirty).toBe(true);
    expect(localStorage.getItem(DRAFT_KEY)).not.toBeNull();
    expect(screen.queryByText(/unsaved draft found/i)).not.toBeInTheDocument();
  });

  it("clears draft and hides banner on Discard", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    offerDraft();
    render(<DraftBanner />);
    fireEvent.click(screen.getByRole("button", { name: /discard/i }));
    expect(useModelStore.getState().features).toHaveLength(0);
    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
    expect(screen.queryByText(/unsaved draft found/i)).not.toBeInTheDocument();
  });
});
