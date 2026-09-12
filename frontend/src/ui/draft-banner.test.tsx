import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DraftBanner } from "./draft-banner";
import { useModelStore } from "@/model/model-store";
import { DRAFT_KEY } from "@/model/use-autosave";
import type { ModelFeature } from "@/model/types";

const sampleFeature: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

beforeEach(() => {
  localStorage.clear();
  useModelStore.getState().reset();
});

describe("DraftBanner", () => {
  it("renders nothing when no draft exists", () => {
    const { container } = render(<DraftBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when draft exists but model already has features", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    useModelStore.getState().loadFeatures([sampleFeature]);
    const { container } = render(<DraftBanner />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows recovery banner when draft exists and model is empty", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
    render(<DraftBanner />);
    expect(screen.getByText(/unsaved draft found/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /restore/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /discard/i }),
    ).toBeInTheDocument();
  });

  it("restores features and keeps the draft on Restore", () => {
    localStorage.setItem(DRAFT_KEY, JSON.stringify([sampleFeature]));
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
    render(<DraftBanner />);
    fireEvent.click(screen.getByRole("button", { name: /discard/i }));
    expect(useModelStore.getState().features).toHaveLength(0);
    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
    expect(screen.queryByText(/unsaved draft found/i)).not.toBeInTheDocument();
  });
});
