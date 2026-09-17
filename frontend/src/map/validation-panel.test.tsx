import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { ValidationPanel } from "./validation-panel";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The panel reads `useModelValidation` and renders it; no map, no WebGL. It is
 * driven here through the real store and the real validator rather than a
 * stubbed report, because the three states it switches on — empty, valid,
 * issues — are exactly the distinction that hook exists to make, and a stub
 * would let the panel and the hook disagree about which is which.
 *
 * `pages/map.test.tsx` mocks this component out, so none of it had a test.
 */

/** Valid on its own: a line source with matching geometry. */
const road: ModelFeature = {
  id: "road-1",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [10, 51],
      [10.01, 51.01],
    ],
  },
};

/** One warning: imported acoustics flagged for review before an RLS-19 run. */
const roadNeedingReview: ModelFeature = {
  ...road,
  properties: { source_acoustics_review_required: true },
};

/** One error: a building must carry a positive height. */
const buildingWithoutHeight: ModelFeature = {
  id: "bld-1",
  kind: "building",
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [10, 51],
        [10.01, 51],
        [10.01, 51.01],
        [10, 51],
      ],
    ],
  },
};

const receiver: ModelReceiver = {
  id: "rcv-1",
  heightM: 4,
  geometry: { type: "Point", coordinates: [10.02, 51.02] },
};

function renderPanel() {
  const onSelectFeature = vi.fn<(featureId: string) => void>();
  render(<ValidationPanel onSelectFeature={onSelectFeature} />);
  return onSelectFeature;
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("ValidationPanel on an empty model", () => {
  it("says there is nothing to check yet, not that there is an error", () => {
    // The validator answers an empty model with a synthetic `model.empty`
    // error whose message is hardcoded English. Showing it would greet a fresh
    // project with "1 error" — in English, whatever the UI language — and the
    // map is now mounted from the start, so this is the first thing seen.
    renderPanel();

    expect(screen.getByText(m.msg_validation_nothing_yet())).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });

  it("counts a receiver-only model as something to check", () => {
    // The hook validates receivers too; a model whose only content is a
    // receiver is not empty and must not read as "nothing yet".
    useModelStore.getState().addReceiver(receiver);
    renderPanel();

    expect(
      screen.queryByText(m.msg_validation_nothing_yet()),
    ).not.toBeInTheDocument();
  });
});

describe("ValidationPanel on a clean model", () => {
  it("says so rather than showing an empty list", () => {
    useModelStore.getState().addFeature(road);
    renderPanel();

    expect(screen.getByText(m.msg_model_valid())).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});

describe("ValidationPanel on a model with findings", () => {
  it("lists one row per finding, with its code", () => {
    // The code is what a user quotes when asking about a finding, and it is
    // the only part of the row that is stable across message changes.
    useModelStore.getState().addFeature(buildingWithoutHeight);
    renderPanel();

    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveTextContent("building.height.required");
  });

  it("counts one error in the singular", () => {
    useModelStore.getState().addFeature(buildingWithoutHeight);
    renderPanel();

    expect(
      screen.getByText(m.msg_validation_error_count_one({ count: 1 })),
    ).toBeInTheDocument();
  });

  it("counts several errors in the plural", () => {
    // English pluralizes; German does not. Picking the form by count rather
    // than by appending an "s" is what keeps both catalogues honest.
    useModelStore.getState().addFeature(buildingWithoutHeight);
    useModelStore.getState().addFeature({
      ...buildingWithoutHeight,
      id: "bld-2",
    });
    renderPanel();

    expect(
      screen.getByText(m.msg_validation_error_count_other({ count: 2 })),
    ).toBeInTheDocument();
  });

  it("names warnings separately from errors", () => {
    useModelStore.getState().addFeature(roadNeedingReview);
    renderPanel();

    expect(
      screen.getByText(m.msg_validation_warning_count_one({ count: 1 })),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
  });

  it("joins the two counts when a model has both", () => {
    // Without the separator the header reads "1 error1 warning".
    useModelStore.getState().addFeature(buildingWithoutHeight);
    useModelStore.getState().addFeature(roadNeedingReview);
    renderPanel();

    const header = `${m.msg_validation_error_count_one({ count: 1 })}, ${m.msg_validation_warning_count_one({ count: 1 })}`;
    expect(screen.getByText(header)).toBeInTheDocument();
  });

  it("lists errors above warnings", () => {
    // Errors block a run and warnings do not, so a list that interleaved them
    // would bury the blocking finding.
    useModelStore.getState().addFeature(roadNeedingReview);
    useModelStore.getState().addFeature(buildingWithoutHeight);
    renderPanel();

    const rows = screen.getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent("building.height.required");
    expect(rows[1]).toHaveTextContent("source.rls19.review_required");
  });

  it("hands the offending feature's id back when the row is followed", () => {
    // The panel is the only route from a finding to the feature that caused
    // it; passing the wrong id sends the user to some other object.
    useModelStore.getState().addFeature(buildingWithoutHeight);
    const onSelectFeature = renderPanel();

    fireEvent.click(screen.getByRole("button", { name: m.action_go_to() }));

    expect(onSelectFeature).toHaveBeenCalledWith("bld-1");
  });

  it("follows a warning to its feature as well as an error", () => {
    useModelStore.getState().addFeature(roadNeedingReview);
    const onSelectFeature = renderPanel();

    fireEvent.click(screen.getByRole("button", { name: m.action_go_to() }));

    expect(onSelectFeature).toHaveBeenCalledWith("road-1");
  });

  it("drops back to the clean message once the model is fixed", () => {
    // The panel stays mounted while the model is edited, so it has to follow
    // the store out of the issues state, not only into it.
    useModelStore.getState().addFeature(buildingWithoutHeight);
    renderPanel();
    expect(screen.getAllByRole("listitem")).toHaveLength(1);

    act(() => {
      useModelStore
        .getState()
        .updateFeature({ ...buildingWithoutHeight, heightM: 8 });
    });

    expect(screen.getByText(m.msg_model_valid())).toBeInTheDocument();
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
