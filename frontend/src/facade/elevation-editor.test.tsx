import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { m } from "@/i18n/messages";
import { getLocale, overwriteGetLocale } from "@/i18n/runtime";
import { resetComputeModelCacheForTests } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import type {
  FacadeElevation,
  PlaceWindowRequest,
  PlaceWindowResponse,
  TransformRequest,
} from "@/wasm/types";
import { ElevationEditor } from "./elevation-editor";

/**
 * The editor only gestures; the placement is the backend's. So the backend
 * here places whatever it is asked to, at a recognisable point, and records
 * every request — which is what these tests read: a gesture must end in
 * exactly the placement it describes, and the store must hold its answer.
 */
const kernel = vi.hoisted(() => {
  const value: {
    placed: PlaceWindowRequest[];
    refuse: string | null;
  } = { placed: [], refuse: null };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    placeWindow: (req: PlaceWindowRequest): Promise<PlaceWindowResponse> => {
      kernel.placed.push(req);
      if (kernel.refuse !== null) {
        return Promise.reject(new Error(kernel.refuse));
      }
      return Promise.resolve({
        feature: {
          type: "Feature",
          id: req.window.id,
          properties: {
            id: req.window.id,
            kind: "window",
            facade: {
              building_ref: req.window.building_ref,
              part: req.window.part,
              ring: req.window.ring,
              edge: req.window.edge,
              from: [547000, 5795000],
              to: [547010, 5795000],
            },
            window: req.window.window,
            sensitive: req.window.sensitive,
            review_status: req.window.review_status ?? "proposed",
          },
          geometry: {
            type: "Point",
            coordinates: [547000 + req.window.window.u_m, 5795000],
          },
        },
      });
    },
    transformCoordinates: (req: TransformRequest) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.source_crs,
        applied: false,
        coordinates: req.coordinates,
      }),
  },
}));

const bedroomFrame = { u_m: 2, sill_z_m: 1, width_m: 1.2, height_m: 1.5 };

const bedroom: ModelFeature = {
  id: "w1",
  kind: "window",
  properties: {
    facade: {
      building_ref: "house",
      part: 0,
      ring: 0,
      edge: 0,
      from: [547000, 5795000],
      to: [547010, 5795000],
    },
    window: bedroomFrame,
    sensitive: true,
    review_status: "confirmed",
    evidence: [{ kind: "plan", source_id: "A-12" }],
  },
  geometry: { type: "Point", coordinates: [547002.6, 5795000] },
};

const south: FacadeElevation = {
  part: 0,
  ring: 0,
  edge: 0,
  from: [547000, 5795000],
  to: [547010, 5795000],
  length_m: 10,
  facing_deg: 180,
  courtyard: false,
  windows: [
    {
      id: "w1",
      window: bedroomFrame,
      sensitive: true,
      review_status: "confirmed",
      findings: [],
    },
  ],
};

// A 10 m × 9 m facade at 32 view units per metre, from (36, 324) up: with
// the drawing laid out at its own size, a client point is a view point.
const x = (u: number) => 36 + 32 * u;
const y = (z: number) => 324 - 32 * z;

const originalGetLocale = getLocale;

beforeEach(() => {
  overwriteGetLocale(() => "en");
  resetComputeModelCacheForTests();
  kernel.placed = [];
  kernel.refuse = null;
  useModelStore.getState().reset();
  useModelStore.getState().loadModel({
    features: [
      {
        id: "house",
        kind: "building",
        heightM: 9,
        geometry: {
          type: "Polygon",
          coordinates: [
            [
              [547000, 5795000],
              [547010, 5795000],
              [547010, 5795008],
              [547000, 5795008],
              [547000, 5795000],
            ],
          ],
        },
      },
      bedroom,
    ],
    receivers: [],
    calcArea: null,
    crs: "EPSG:25832",
  });
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockReturnValue({
    left: 0,
    top: 0,
    right: 640,
    bottom: 360,
    width: 640,
    height: 360,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  });
});

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
  vi.restoreAllMocks();
});

function renderEditor(facade: FacadeElevation = south) {
  return render(
    <ElevationEditor
      buildingId="house"
      facade={facade}
      number={1}
      heightM={9}
      slabHeightsM={[3, 6, 9]}
    />,
  );
}

function stored(id: string) {
  return useModelStore.getState().getFeatureById(id);
}

function drawing() {
  return screen.getByRole("group", { name: /^Facade 1/ });
}

describe("ElevationEditor", () => {
  it("adds a window proposed and not sensitive, as one undo step", async () => {
    renderEditor();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_window() }),
    );

    await waitFor(() => {
      expect(stored("win-1")).toBeDefined();
    });
    expect(kernel.placed[0]?.window).toEqual({
      id: "win-1",
      building_ref: "house",
      part: 0,
      ring: 0,
      edge: 0,
      window: { u_m: 4.4, sill_z_m: 0.9, width_m: 1.2, height_m: 1.5 },
      sensitive: false,
    });
    expect(stored("win-1")?.properties?.review_status).toBe("proposed");

    useModelStore.getState().undo();
    expect(stored("win-1")).toBeUndefined();
  });

  it("names a new window past a receiver's id", async () => {
    useModelStore.getState().addReceiver({
      id: "win-1",
      heightM: 4,
      geometry: { type: "Point", coordinates: [547005, 5795020] },
    });
    renderEditor();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_window() }),
    );

    await waitFor(() => {
      expect(stored("win-2")).toBeDefined();
    });
    expect(stored("win-1")).toBeUndefined();
  });

  it("draws a window by dragging on the wall", async () => {
    renderEditor();
    const wall = drawing().querySelector("[data-wall]");
    expect(wall).not.toBeNull();

    fireEvent.pointerDown(wall as Element, {
      button: 0,
      pointerId: 1,
      clientX: x(1),
      clientY: y(1),
    });
    fireEvent.pointerMove(drawing(), {
      pointerId: 1,
      clientX: x(3),
      clientY: y(2.5),
    });
    fireEvent.pointerUp(drawing(), { pointerId: 1 });

    await waitFor(() => {
      expect(stored("win-1")).toBeDefined();
    });
    expect(kernel.placed[0]?.window.window).toEqual({
      u_m: 1,
      sill_z_m: 1,
      width_m: 2,
      height_m: 1.5,
    });
  });

  it("moves a window by dragging it, keeping what was said about it", async () => {
    renderEditor();
    const rect = drawing().querySelector('[data-window-id="w1"]');

    fireEvent.pointerDown(rect as Element, {
      button: 0,
      pointerId: 1,
      clientX: x(2.5),
      clientY: y(1.5),
    });
    fireEvent.pointerMove(drawing(), {
      pointerId: 1,
      clientX: x(3.5),
      clientY: y(2),
    });
    fireEvent.pointerUp(drawing(), { pointerId: 1 });

    await waitFor(() => {
      expect(stored("w1")?.properties?.window).toEqual({
        ...bedroomFrame,
        u_m: 3,
        sill_z_m: 1.5,
      });
    });
    expect(kernel.placed).toHaveLength(1);
    expect(kernel.placed[0]?.window).toMatchObject({
      id: "w1",
      sensitive: true,
      review_status: "confirmed",
      evidence: [{ kind: "plan", source_id: "A-12" }],
    });
    const w1 = stored("w1");
    expect(w1?.properties?.evidence).toEqual([
      { kind: "plan", source_id: "A-12" },
    ]);
    expect(w1?.geometry.coordinates).toEqual([547003, 5795000]);
  });

  it("does not place a window a click left where it was", () => {
    renderEditor();
    const rect = drawing().querySelector('[data-window-id="w1"]');

    fireEvent.pointerDown(rect as Element, {
      button: 0,
      pointerId: 1,
      clientX: x(2.5),
      clientY: y(1.5),
    });
    fireEvent.pointerUp(drawing(), { pointerId: 1 });

    expect(kernel.placed).toHaveLength(0);
  });

  it("places nothing when the pointer is cancelled", () => {
    renderEditor();
    const rect = drawing().querySelector('[data-window-id="w1"]');

    fireEvent.pointerDown(rect as Element, {
      button: 0,
      pointerId: 1,
      clientX: x(2.5),
      clientY: y(1.5),
    });
    fireEvent.pointerMove(drawing(), {
      pointerId: 1,
      clientX: x(4),
      clientY: y(1.5),
    });
    fireEvent.pointerCancel(drawing(), { pointerId: 1 });

    expect(kernel.placed).toHaveLength(0);
    expect(stored("w1")?.properties?.window).toEqual(bedroomFrame);
  });

  it("places one window at a time, so two quick adds make one window", async () => {
    renderEditor();
    const add = screen.getByRole("button", { name: m.action_add_window() });

    fireEvent.click(add);
    fireEvent.click(add);

    await waitFor(() => {
      expect(stored("win-1")).toBeDefined();
    });
    expect(kernel.placed).toHaveLength(1);
    expect(
      useModelStore.getState().features.filter((f) => f.id === "win-1"),
    ).toHaveLength(1);
  });

  it("moves the selected window with the arrow keys", async () => {
    renderEditor();
    const w1 = screen.getByRole("button", { name: /^Window w1/ });

    w1.focus();
    fireEvent.keyDown(w1, { key: "ArrowRight" });

    await waitFor(() => {
      expect(stored("w1")?.properties?.window).toEqual({
        ...bedroomFrame,
        u_m: 2.1,
      });
    });

    fireEvent.keyDown(w1, { key: "ArrowUp", shiftKey: true });
    await waitFor(() => {
      expect(kernel.placed.at(-1)?.window.window.sill_z_m).toBe(2);
    });
  });

  it("shows a refusal and leaves the model as it was", async () => {
    kernel.refuse = 'window "w1": u_m 9.9 + width_m 1.2 runs past the end';
    renderEditor();
    const w1 = screen.getByRole("button", { name: /^Window w1/ });

    w1.focus();
    fireEvent.keyDown(w1, { key: "ArrowRight" });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "runs past the end",
    );
    expect(stored("w1")?.properties?.window).toEqual(bedroomFrame);
  });

  it("edits the selected window from its form", async () => {
    renderEditor();
    // jsdom does not move focus onto an SVG element; the event is what the
    // editor listens to.
    fireEvent.focus(screen.getByRole("button", { name: /^Window w1/ }));
    const form = screen.getByRole("region", {
      name: m.heading_window_edit({ id: "w1" }),
    });

    const width = within(form).getByLabelText(m.label_window_width());
    fireEvent.change(width, { target: { value: "2" } });
    fireEvent.blur(width);
    await waitFor(() => {
      expect(stored("w1")?.properties?.window).toEqual({
        ...bedroomFrame,
        width_m: 2,
      });
    });

    fireEvent.click(within(form).getByLabelText(m.label_window_sensitive()));
    expect(stored("w1")?.properties?.sensitive).toBe(false);

    fireEvent.change(within(form).getByLabelText(m.label_window_review()), {
      target: { value: "rejected" },
    });
    expect(stored("w1")?.properties?.review_status).toBe("rejected");
  });

  it("puts a refused frame value back in its field", async () => {
    kernel.refuse = 'window "w1": u_m 2 + width_m 50 runs past the end';
    renderEditor();
    fireEvent.focus(screen.getByRole("button", { name: /^Window w1/ }));
    const width = within(
      screen.getByRole("region", { name: m.heading_window_edit({ id: "w1" }) }),
    ).getByLabelText(m.label_window_width());

    fireEvent.change(width, { target: { value: "50" } });
    fireEvent.keyDown(width, { key: "Enter" });
    fireEvent.blur(width);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "runs past the end",
    );
    expect(kernel.placed).toHaveLength(1);
    await waitFor(() => {
      expect(width).toHaveValue(1.2);
    });
  });

  it("deletes a window after asking", async () => {
    renderEditor();
    // jsdom does not move focus onto an SVG element; the event is what the
    // editor listens to.
    fireEvent.focus(screen.getByRole("button", { name: /^Window w1/ }));

    fireEvent.click(
      within(
        screen.getByRole("region", {
          name: m.heading_window_edit({ id: "w1" }),
        }),
      ).getByRole("button", { name: m.action_delete_window() }),
    );
    const confirm = await screen.findByRole("alertdialog");
    fireEvent.click(
      within(confirm).getByRole("button", { name: m.action_delete_window() }),
    );

    expect(stored("w1")).toBeUndefined();
    useModelStore.getState().undo();
    expect(stored("w1")).toBeDefined();
  });
});
