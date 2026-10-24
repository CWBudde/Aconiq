import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
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
  PhotoTransform,
  PlaceWindowRequest,
  PlaceWindowResponse,
  TransformRequest,
} from "@/wasm/types";
import { ElevationEditor } from "./elevation-editor";
import { hashPhoto } from "./photo";

/**
 * The editor only gestures; the placement is the backend's. So the backend
 * here places whatever it is asked to, at a recognisable point, and records
 * every request — which is what these tests read: a gesture must end in
 * exactly the placement it describes, and the store must hold its answer.
 */
const kernel = vi.hoisted(() => {
  const value: {
    placed: PlaceWindowRequest[];
    rectified: PhotoTransform[];
    refuse: string | null;
    /** When set, every answer waits for it: a placement held in flight. */
    gate: Promise<void> | null;
  } = { placed: [], rectified: [], refuse: null, gate: null };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    placeWindow: (req: PlaceWindowRequest): Promise<PlaceWindowResponse> => {
      kernel.placed.push(req);
      if (kernel.refuse !== null) {
        return Promise.reject(new Error(kernel.refuse));
      }
      const answer: PlaceWindowResponse = {
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
            // The Go placer keeps a placement's evidence on the feature.
            ...(req.window.evidence === undefined
              ? {}
              : { evidence: req.window.evidence }),
          },
          geometry: {
            type: "Point",
            coordinates: [547000 + req.window.window.u_m, 5795000],
          },
        },
      };
      return (kernel.gate ?? Promise.resolve()).then(() => answer);
    },
    // A straight-on photo, 100 px to the metre: what is solved is not the
    // editor's business, only what it sends and draws.
    rectifyFacade: (req: PhotoTransform) => {
      kernel.rectified.push(req);
      return Promise.resolve({
        matrix: [0.01, 0, 0, 0, -0.01, 9, 0, 0, 1],
        inverse: [100, 0, 0, 0, -100, 900, 0, 0, 1],
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
  party_walls: [],
  windows: [
    {
      id: "w1",
      window: bedroomFrame,
      sensitive: true,
      review_status: "confirmed",
      on_party_wall: false,
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
  kernel.rectified = [];
  kernel.refuse = null;
  kernel.gate = null;
  // jsdom has no object URLs; the editor only needs one to hand to an <img>.
  URL.createObjectURL = vi.fn(() => "blob:facade-photo");
  URL.revokeObjectURL = vi.fn();
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

/** Holds every placement until the returned function is called. */
function holdPlacements(): () => Promise<void> {
  let release = () => {};
  kernel.gate = new Promise((resolve) => {
    release = resolve;
  });
  return async () => {
    await act(async () => {
      release();
      // A macrotask, so every continuation of the answer has run.
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  };
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

  it("stops a dragged window at the top of the facade", async () => {
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
      clientX: x(2.5),
      clientY: y(20),
    });
    fireEvent.pointerUp(drawing(), { pointerId: 1 });

    await waitFor(() => {
      expect(kernel.placed).toHaveLength(1);
    });
    // 9 m of facade, 1.5 m of window.
    expect(kernel.placed[0]?.window.window.sill_z_m).toBe(7.5);
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

  it("drops a placement an undo overtook", async () => {
    renderEditor();
    const w1 = screen.getByRole("button", { name: /^Window w1/ });

    fireEvent.keyDown(w1, { key: "ArrowRight" });
    await waitFor(() => {
      expect(stored("w1")?.properties?.window).toEqual({
        ...bedroomFrame,
        u_m: 2.1,
      });
    });

    const release = holdPlacements();
    fireEvent.keyDown(w1, { key: "ArrowRight" });
    await waitFor(() => {
      expect(kernel.placed).toHaveLength(2);
    });
    useModelStore.getState().undo();
    await release();

    // The answer was for the window at 2.1 m; the undo put it back at 2 m.
    expect(stored("w1")?.properties?.window).toEqual(bedroomFrame);
    expect(stored("w1")?.geometry).toEqual(bedroom.geometry);
  });

  it("drops a placement measured on a building since reshaped", async () => {
    renderEditor();
    const w1 = screen.getByRole("button", { name: /^Window w1/ });

    const release = holdPlacements();
    fireEvent.keyDown(w1, { key: "ArrowRight" });
    await waitFor(() => {
      expect(kernel.placed).toHaveLength(1);
    });
    useModelStore.getState().updateFeatureGeometry("house", {
      type: "Polygon",
      coordinates: [
        [
          [547000, 5795000],
          [547012, 5795000],
          [547012, 5795008],
          [547000, 5795008],
          [547000, 5795000],
        ],
      ],
    });
    await release();

    expect(stored("w1")?.properties?.window).toEqual(bedroomFrame);
  });

  it("drops a new window whose id an undo gave back", async () => {
    useModelStore.getState().addFeature({ ...bedroom, id: "win-1" });
    useModelStore.getState().removeFeature("win-1");
    renderEditor();

    const release = holdPlacements();
    fireEvent.click(
      screen.getByRole("button", { name: m.action_add_window() }),
    );
    await waitFor(() => {
      expect(kernel.placed[0]?.window.id).toBe("win-1");
    });
    useModelStore.getState().undo();
    await release();

    expect(
      useModelStore.getState().features.filter((f) => f.id === "win-1"),
    ).toHaveLength(1);
  });

  it("keeps a review field set while a placement was in flight", async () => {
    renderEditor();
    const w1 = screen.getByRole("button", { name: /^Window w1/ });
    fireEvent.focus(w1);
    const form = screen.getByRole("region", {
      name: m.heading_window_edit({ id: "w1" }),
    });

    const release = holdPlacements();
    fireEvent.keyDown(w1, { key: "ArrowRight" });
    await waitFor(() => {
      expect(kernel.placed).toHaveLength(1);
    });
    fireEvent.change(within(form).getByLabelText(m.label_window_review()), {
      target: { value: "rejected" },
    });
    await release();

    expect(stored("w1")?.properties?.window).toEqual({
      ...bedroomFrame,
      u_m: 2.1,
    });
    expect(stored("w1")?.properties?.review_status).toBe("rejected");
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

  it("says in the form when the window stands on a party wall", () => {
    const window = south.windows[0];
    if (window === undefined) throw new Error("fixture has no window");
    renderEditor({
      ...south,
      party_walls: [{ from_um: 0, to_um: 4 }],
      windows: [{ ...window, on_party_wall: true }],
    });

    fireEvent.focus(screen.getByRole("button", { name: /^Window w1/ }));
    const form = screen.getByRole("region", {
      name: m.heading_window_edit({ id: "w1" }),
    });
    expect(within(form).getByText(m.msg_window_on_party_wall())).toHaveClass(
      "text-destructive",
    );
    expect(drawing().querySelectorAll("[data-party-wall]")).toHaveLength(1);
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

  describe("with a photo of the facade", () => {
    const photoBytes = "not really a jpeg";
    const corners: [number, number][] = [
      [0, 900],
      [1000, 900],
      [1000, 0],
      [0, 0],
    ];

    function choosePhoto() {
      fireEvent.change(screen.getByLabelText(m.label_photo_file()), {
        target: { files: [new File([photoBytes], "IMG_0042.jpg")] },
      });
    }

    async function clickCorners() {
      await screen.findByRole("region", { name: m.heading_photo_corners() });
      const labels = [
        m.label_corner_bottom_left(),
        m.label_corner_bottom_right(),
        m.label_corner_top_right(),
        m.label_corner_top_left(),
      ];
      corners.forEach(([px, py], i) => {
        const corner = labels[i] ?? "";
        fireEvent.change(
          screen.getByLabelText(m.label_photo_corner_x({ corner })),
          { target: { value: String(px) } },
        );
        fireEvent.change(
          screen.getByLabelText(m.label_photo_corner_y({ corner })),
          { target: { value: String(py) } },
        );
      });
      fireEvent.change(screen.getByLabelText(m.label_photo_reference()), {
        target: { value: "store/0042" },
      });
      fireEvent.click(
        screen.getByRole("button", { name: m.action_rectify_photo() }),
      );
      await waitFor(() => {
        expect(document.querySelector("img[data-facade-photo]")).not.toBeNull();
      });
    }

    it("rectifies it from four corners and records it on a window drawn over it, never the image", async () => {
      renderEditor();
      choosePhoto();
      await clickCorners();

      // Only the corners were sent, against the facade's frame corners.
      expect(kernel.rectified).toEqual([
        {
          image_points: corners,
          facade_points: [
            [0, 0],
            [10, 0],
            [10, 9],
            [0, 9],
          ],
        },
      ]);
      expect(
        document.querySelector<HTMLImageElement>("img[data-facade-photo]")
          ?.style.transform,
      ).toMatch(/^matrix3d\(/);

      fireEvent.click(
        screen.getByRole("button", { name: m.action_add_window() }),
      );
      await waitFor(() => {
        expect(stored("win-1")).toBeDefined();
      });

      const sha256 = await hashPhoto(new Blob([photoBytes]));
      expect(stored("win-1")?.properties?.evidence).toEqual([
        {
          kind: "site-photo",
          source_id: "store/0042",
          content_sha256: sha256,
          licence: "own",
          transform: kernel.rectified[0],
        },
      ]);
      const model = JSON.stringify(useModelStore.getState().features);
      expect(model).not.toContain("blob:");
      expect(model).not.toContain("data:");
      expect(model).not.toContain("IMG_0042");
    });

    it("adds the photo to a window moved over it, keeping its other evidence", async () => {
      renderEditor();
      choosePhoto();
      await clickCorners();

      const window = within(drawing()).getByRole("button", {
        name: /^Window w1/,
      });
      fireEvent.focus(window);
      fireEvent.keyDown(window, { key: "ArrowRight" });

      await waitFor(() => {
        expect(stored("w1")?.properties?.evidence).toHaveLength(2);
      });
      const evidence = stored("w1")?.properties?.evidence as Record<
        string,
        unknown
      >[];
      expect(evidence[0]).toEqual({ kind: "plan", source_id: "A-12" });
      expect(evidence[1]).toMatchObject({
        kind: "site-photo",
        transform: kernel.rectified[0],
      });
    });

    it("draws the same photo again from the corners a window recorded, without asking for them", async () => {
      const sha256 = await hashPhoto(new Blob([photoBytes]));
      const transform = {
        image_points: corners,
        facade_points: [
          [0, 0],
          [10, 0],
          [10, 9],
          [0, 9],
        ],
      };
      useModelStore.getState().updateFeature({
        ...bedroom,
        properties: {
          ...bedroom.properties,
          evidence: [
            {
              kind: "site-photo",
              source_id: "store/0042",
              content_sha256: sha256,
              licence: "own",
              transform,
            },
          ],
        },
      });
      renderEditor();
      choosePhoto();

      await waitFor(() => {
        expect(document.querySelector("img[data-facade-photo]")).not.toBeNull();
      });
      expect(
        screen.queryByRole("region", { name: m.heading_photo_corners() }),
      ).toBeNull();
      expect(kernel.rectified).toEqual([transform]);
      expect(screen.getByText(new RegExp(m.msg_photo_reused()))).toBeTruthy();
    });

    it("asks for the corners again when they were recorded against a facade of another size", async () => {
      const sha256 = await hashPhoto(new Blob([photoBytes]));
      useModelStore.getState().updateFeature({
        ...bedroom,
        properties: {
          ...bedroom.properties,
          evidence: [
            {
              kind: "site-photo",
              source_id: "store/0042",
              content_sha256: sha256,
              licence: "own",
              transform: {
                image_points: corners,
                // The building was 8 m high then; it is 9 m now.
                facade_points: [
                  [0, 0],
                  [10, 0],
                  [10, 8],
                  [0, 8],
                ],
              },
            },
          ],
        },
      });
      renderEditor();
      choosePhoto();

      await screen.findByRole("region", { name: m.heading_photo_corners() });
      expect(kernel.rectified).toEqual([]);
    });
  });
});
