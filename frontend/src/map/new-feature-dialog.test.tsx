import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  it("reads a drawn shape's coordinates back in the CRS it landed in", () => {
    // The dialog used to take a geometry and never show it, so the only
    // visible consequence of `use-draw-projection.ts` was a shape's position
    // on a basemap.
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderDialog(point);

    expect(
      screen.getByText(`${m.label_coordinates()} (EPSG:25832)`),
    ).toBeVisible();
    expect(screen.getByText("10.00, 51.00")).toBeVisible();
  });
});

/**
 * The keyboard path: the dialog is opened with no geometry and asks for the
 * coordinates instead of being handed them.
 *
 * **The CRS constraint these pin down.** The typed numbers are read as the
 * store's own CRS and written into the store unchanged. Nothing here calls
 * `backend.transformCoordinates`, so this is not a second coordinate writer
 * beside `use-draw-projection.ts` — that inverse WGS 84 → store transform
 * stays the single deliberate exception to "the map is a projection *of* the
 * model, never a source *for* it". The label assertions are what would fail if
 * someone later relabelled these fields as longitude/latitude, which is the
 * shape the mistake would take.
 */
function renderTypingDialog(onClose = vi.fn()) {
  render(<NewFeatureDialog open={true} geometry={null} onClose={onClose} />);
  return onClose;
}

function typeInto(label: string | RegExp, value: string) {
  fireEvent.change(screen.getByLabelText(label), { target: { value } });
}

describe("NewFeatureDialog coordinate entry", () => {
  it("labels every coordinate field with the CRS the model is stored in", () => {
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderTypingDialog();

    expect(
      screen.getByLabelText(m.label_coordinate_x({ crs: "EPSG:25832" })),
    ).toBeVisible();
    expect(
      screen.getByLabelText(m.label_coordinate_y({ crs: "EPSG:25832" })),
    ).toBeVisible();
    expect(
      screen.getByText(m.msg_coordinate_entry_crs({ crs: "EPSG:25832" })),
    ).toBeVisible();
  });

  it("writes the typed numbers into the store untouched", async () => {
    // A metric store, and an easting/northing pair that could not be mistaken
    // for degrees: if anything ever projects this path, these numbers move.
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderTypingDialog();

    typeInto(m.label_coordinate_x({ crs: "EPSG:25832" }), "566000.5");
    typeInto(m.label_coordinate_y({ crs: "EPSG:25832" }), "5651000.25");
    await userEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    const state = useModelStore.getState();
    expect(state.features).toHaveLength(1);
    expect(state.features[0]?.geometry).toEqual({
      type: "Point",
      coordinates: [566000.5, 5651000.25],
    });
    expect(state.crs).toBe("EPSG:25832");
  });

  it("refuses an incomplete geometry without leaving the focus order", async () => {
    // `aria-disabled` plus a swallowed click, never `disabled`: the reason is
    // beside the button, and a `disabled` one is skipped by Tab and carries
    // `pointer-events-none`, so the state cannot be inspected where it holds.
    renderTypingDialog();

    const add = screen.getByRole("button", { name: m.action_add_feature() });
    expect(add).toHaveAttribute("aria-disabled", "true");
    expect(add).not.toBeDisabled();
    expect(screen.getByText(m.msg_coordinate_entry_incomplete())).toBeVisible();

    await userEvent.click(add);

    expect(useModelStore.getState().features).toHaveLength(0);
  });

  it("grows the vertex list to what the chosen kind needs", () => {
    const crs = "EPSG:4326";
    renderTypingDialog();

    // A source point asks for one pair.
    expect(
      screen.getAllByLabelText(m.label_coordinate_x({ crs })),
    ).toHaveLength(1);

    selectKind(m.option_building());

    // A building is a polygon, so three.
    expect(
      screen.getAllByLabelText(m.label_coordinate_x({ crs })),
    ).toHaveLength(3);
  });

  it("closes a typed polygon ring itself", async () => {
    const crs = "EPSG:4326";
    renderTypingDialog();
    selectKind(m.option_building());

    const xs = screen.getAllByLabelText(m.label_coordinate_x({ crs }));
    const ys = screen.getAllByLabelText(m.label_coordinate_y({ crs }));
    const ring = [
      [0, 0],
      [1, 0],
      [1, 1],
    ];
    ring.forEach(([x, y], i) => {
      fireEvent.change(xs[i] as HTMLElement, { target: { value: String(x) } });
      fireEvent.change(ys[i] as HTMLElement, { target: { value: String(y) } });
    });

    await userEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    expect(useModelStore.getState().features[0]?.geometry).toEqual({
      type: "Polygon",
      coordinates: [
        [
          [0, 0],
          [1, 0],
          [1, 1],
          [0, 0],
        ],
      ],
    });
  });

  it("refuses to remove a vertex the geometry still needs", async () => {
    const crs = "EPSG:4326";
    renderTypingDialog();
    selectKind(m.option_barrier());

    const remove = screen.getByRole("button", {
      name: m.action_remove_vertex({ index: 1 }),
    });
    expect(remove).toHaveAttribute("aria-disabled", "true");
    expect(remove).not.toBeDisabled();
    expect(screen.getByText(m.msg_vertex_minimum({ count: 2 }))).toBeVisible();

    await userEvent.click(remove);

    expect(
      screen.getAllByLabelText(m.label_coordinate_x({ crs })),
    ).toHaveLength(2);
  });

  it("adds and removes a vertex once the minimum is cleared", async () => {
    const crs = "EPSG:4326";
    renderTypingDialog();
    selectKind(m.option_barrier());

    await userEvent.click(
      screen.getByRole("button", { name: m.action_add_vertex() }),
    );
    expect(
      screen.getAllByLabelText(m.label_coordinate_x({ crs })),
    ).toHaveLength(3);

    await userEvent.click(
      screen.getByRole("button", {
        name: m.action_remove_vertex({ index: 3 }),
      }),
    );
    expect(
      screen.getAllByLabelText(m.label_coordinate_x({ crs })),
    ).toHaveLength(2);
  });

  it("adds a receiver from typed coordinates", () => {
    renderTypingDialog();
    selectKind(m.option_receiver());

    typeInto(m.label_coordinate_x({ crs: "EPSG:4326" }), "10");
    typeInto(m.label_coordinate_y({ crs: "EPSG:4326" }), "51");
    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_feature() }),
    );

    const state = useModelStore.getState();
    expect(state.receivers).toHaveLength(1);
    expect(state.receivers[0]?.geometry).toEqual(point);
  });
});
