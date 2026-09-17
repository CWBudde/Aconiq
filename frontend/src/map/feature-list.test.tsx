import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FeatureList } from "./feature-list";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The map's own selection is a click on a canvas, which a keyboard cannot
 * produce. This list is the same selection by another route, so what it has to
 * prove is that every entry is a real control — reachable by Tab and activated
 * by Enter — and that activating one reports the id the page selects by.
 */

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const building: ModelFeature = {
  id: "bld-1",
  kind: "building",
  heightM: 8,
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [0, 0],
        [1, 0],
        [1, 1],
        [0, 0],
      ],
    ],
  },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10.1, 51.1] },
};

function load() {
  useModelStore.getState().loadModel({
    features: [source, building],
    receivers: [receiver],
    calcArea: null,
  });
}

function renderList(selectedId: string | null = null) {
  const onSelect = vi.fn<(id: string) => void>();
  render(<FeatureList selectedId={selectedId} onSelect={onSelect} />);
  return onSelect;
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("FeatureList", () => {
  it("is a named region, so it is not an anonymous box of buttons", () => {
    load();
    renderList();

    expect(
      screen.getByRole("region", { name: m.label_feature_list() }),
    ).toBeInTheDocument();
  });

  it("lists every feature and every receiver by kind and id", () => {
    load();
    renderList();

    const entries = screen.getAllByRole("button");
    expect(entries.map((entry) => entry.textContent)).toEqual([
      `${m.option_source()}src-1`,
      `${m.option_building()}bld-1`,
      `${m.option_receiver()}rcv-1`,
    ]);
  });

  it("reports the id the page selects by", async () => {
    load();
    const onSelect = renderList();

    await userEvent.click(screen.getAllByRole("button")[1] as HTMLElement);

    expect(onSelect).toHaveBeenCalledWith("bld-1");
  });

  it("reaches every entry by Tab and activates one by Enter", async () => {
    // The whole point of the list. A regression to a clickable `<div>` — or to
    // an entry rendered as a `disabled` button — fails here rather than in a
    // review.
    load();
    const onSelect = renderList();

    await userEvent.tab();
    expect(screen.getAllByRole("button")[0]).toHaveFocus();
    await userEvent.tab();
    await userEvent.tab();
    expect(screen.getAllByRole("button")[2]).toHaveFocus();

    await userEvent.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledWith("rcv-1");
  });

  it("marks the entry the page is editing as the current one", () => {
    load();
    renderList("bld-1");

    const entries = screen.getAllByRole("button");
    expect(entries[1]).toHaveAttribute("aria-current", "true");
    expect(entries[0]).not.toHaveAttribute("aria-current");
  });

  it("says the model is empty rather than showing an empty box", () => {
    renderList();

    expect(screen.getByText(m.msg_feature_list_empty())).toBeVisible();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });
});
