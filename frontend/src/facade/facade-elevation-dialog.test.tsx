import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { m } from "@/i18n/messages";
import { getLocale, overwriteGetLocale } from "@/i18n/runtime";
import { resetComputeModelCacheForTests } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import type {
  ElevationWindow,
  FacadeElevationRequest,
  FacadeElevationResponse,
  TransformRequest,
} from "@/wasm/types";
import FacadeElevationDialog from "./facade-elevation-dialog";
import { compassLabel } from "./elevation-labels";

/**
 * The dialog draws what the backend measured and nothing else, so the backend
 * is a stand-in that answers one elevation: a 10 m × 8 m house, a confirmed
 * sensitive window on its south facade, one above the roof, and one whose
 * key no longer names an edge.
 */
const kernel = vi.hoisted(() => {
  const value: {
    asked: FacadeElevationRequest[];
    answer: () => Promise<FacadeElevationResponse>;
  } = { asked: [], answer: () => Promise.reject(new Error("unset")) };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    facadeElevation: (req: FacadeElevationRequest) => {
      kernel.asked.push(req);
      return kernel.answer();
    },
    // `auto` leaves a projected workspace where it is, as the kernel does.
    transformCoordinates: (req: TransformRequest) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.source_crs,
        applied: false,
        coordinates: req.coordinates,
      }),
  },
}));

function win(id: string, over: Partial<ElevationWindow> = {}): ElevationWindow {
  return {
    id,
    window: { u_m: 2, sill_z_m: 3.9, width_m: 1.2, height_m: 1.5 },
    sensitive: false,
    review_status: "proposed",
    findings: [],
    ...over,
  };
}

const house: FacadeElevationResponse = {
  building_ref: "house",
  height_m: 9,
  levels: 3,
  slab_heights_m: [3, 6, 9],
  slab_height_source: "measured",
  facades: [
    {
      part: 0,
      ring: 0,
      edge: 0,
      from: [547000, 5795000],
      to: [547010, 5795000],
      length_m: 10,
      facing_deg: 180,
      courtyard: false,
      windows: [
        win("a-bed", { sensitive: true, review_status: "confirmed" }),
        win("attic", {
          window: { u_m: 5, sill_z_m: 8.5, width_m: 1.2, height_m: 1.5 },
          findings: [
            {
              level: "warning",
              code: "window.frame.above_building",
              feature_id: "attic",
              message: "rises above the building",
            },
          ],
        }),
      ],
    },
    {
      part: 0,
      ring: 0,
      edge: 1,
      from: [547010, 5795000],
      to: [547010, 5795008],
      length_m: 8,
      facing_deg: 90,
      courtyard: false,
      windows: [],
    },
  ],
  detached: [
    win("moved", {
      findings: [
        {
          level: "warning",
          code: "window.facade.moved",
          feature_id: "moved",
          message: "its facade moved",
        },
      ],
    }),
  ],
};

const originalGetLocale = getLocale;

beforeEach(() => {
  overwriteGetLocale(() => "en");
  resetComputeModelCacheForTests();
  kernel.asked = [];
  kernel.answer = () => Promise.resolve(house);
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
    ],
    receivers: [],
    calcArea: null,
    crs: "EPSG:25832",
  });
});

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
});

function renderDialog() {
  return render(
    <FacadeElevationDialog buildingId="house" open onOpenChange={() => {}} />,
  );
}

describe("FacadeElevationDialog", () => {
  it("asks for the building in the CRS the workspace computes in", async () => {
    renderDialog();

    await screen.findByText("a-bed");
    expect(kernel.asked).toHaveLength(1);
    expect(kernel.asked[0]?.building).toBe("house");
    expect(kernel.asked[0]?.crs).toBe("EPSG:25832");
  });

  it("draws the first facade with its storeys and windows", async () => {
    renderDialog();

    expect(
      await screen.findByText(
        m.msg_facade_elevation_summary({
          height: "9.0",
          storeys: 3,
          source: m.label_slab_source_measured(),
        }),
      ),
    ).toBeInTheDocument();

    const drawing = screen.getByRole("group", {
      name: m.label_facade_elevation_svg({
        n: 1,
        length: "10.0",
        height: "9.0",
        count: 2,
      }),
    });
    expect(drawing.querySelectorAll("[data-window-id]")).toHaveLength(2);
    expect(within(drawing).getAllByText(/^[0-2]$/)).toHaveLength(3);

    const facades = screen.getByLabelText(m.label_facade_select());
    expect(
      within(facades).getByRole("option", {
        name: m.label_facade_option({
          n: 1,
          facing: m.label_compass_s(),
          length: "10.0",
        }),
      }),
    ).toBeInTheDocument();

    const attic = screen.getByText("attic").closest("tr");
    expect(attic).not.toBeNull();
    expect(
      within(attic as HTMLElement).getByText("window.frame.above_building"),
    ).toBeInTheDocument();
  });

  it("switches facades from the picker", async () => {
    renderDialog();
    const facades = await screen.findByLabelText(m.label_facade_select());

    await userEvent.selectOptions(facades, "1");

    expect(
      screen.getByRole("group", {
        name: m.label_facade_elevation_svg({
          n: 2,
          length: "8.0",
          height: "9.0",
          count: 0,
        }),
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(m.msg_facade_no_windows())).toBeInTheDocument();
  });

  it("lists windows on no facade apart", async () => {
    renderDialog();

    await screen.findByText(m.heading_facade_detached());
    expect(screen.getByText("moved")).toBeInTheDocument();
    expect(screen.getByText("window.facade.moved")).toBeInTheDocument();
  });

  it("words a window's findings in the reader's language", async () => {
    // The kernel's finding carries English prose built by the Go validator;
    // the dialog shows the code's sentence from the catalogue instead, and a
    // code it has no sentence for as a plain finding, never the prose.
    overwriteGetLocale(() => "de");
    kernel.answer = () =>
      Promise.resolve({
        ...house,
        detached: [
          win("moved", {
            findings: [
              {
                level: "warning",
                code: "window.facade.moved",
                feature_id: "moved",
                message: "its facade moved",
              },
              {
                level: "error",
                code: "window.invalid",
                feature_id: "moved",
                message:
                  'window "moved": sensitive "yes" must be true or false',
              },
            ],
          }),
        ],
      });
    renderDialog();

    await screen.findByText(m.heading_facade_detached());
    const row = screen.getByText("moved").closest("tr") as HTMLElement;
    expect(
      within(row).getByText(m.msg_validation_window_facade_moved()),
    ).toBeInTheDocument();
    expect(
      within(row).getByText(m.msg_facade_finding_unnamed()),
    ).toBeInTheDocument();
    expect(within(row).getByText("window.invalid")).toBeInTheDocument();
    expect(row.textContent).not.toContain("its facade moved");
    expect(row.textContent).not.toContain("must be true or false");

    const attic = screen.getByText("attic").closest("tr") as HTMLElement;
    expect(
      within(attic).getByText(m.msg_validation_window_frame_above_building()),
    ).toBeInTheDocument();
  });

  it("says why the facades could not be measured", async () => {
    kernel.answer = () =>
      Promise.reject(new Error('building "house" is not in the model'));
    renderDialog();

    await waitFor(() => {
      expect(
        screen.getByText(
          m.msg_facade_elevation_failed({
            error: 'building "house" is not in the model',
          }),
        ),
      ).toBeInTheDocument();
    });
  });
});

describe("compassLabel", () => {
  it("names the eighth of the compass a bearing falls in", () => {
    expect(compassLabel(0)).toBe(m.label_compass_n());
    expect(compassLabel(359)).toBe(m.label_compass_n());
    expect(compassLabel(90)).toBe(m.label_compass_e());
    expect(compassLabel(135)).toBe(m.label_compass_se());
    expect(compassLabel(180)).toBe(m.label_compass_s());
    expect(compassLabel(292)).toBe(m.label_compass_w());
    expect(compassLabel(300)).toBe(m.label_compass_nw());
  });
});
