import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetComputeModelCacheForTests } from "@/model/compute-crs";
import type { ModelFeature } from "@/model/types";
import type {
  PlaceWindowRequest,
  PlaceWindowResponse,
  TransformRequest,
  TransformResponse,
  WindowPlacement,
} from "@/wasm/types";
import { nextWindowId, placeWindowFeature } from "./place-window";

function house(coordinates: [number, number][]): ModelFeature {
  return {
    id: "house",
    kind: "building",
    heightM: 9,
    geometry: { type: "Polygon", coordinates: [coordinates] },
  };
}

const placement: WindowPlacement = {
  id: "w1",
  building_ref: "house",
  part: 0,
  ring: 0,
  edge: 0,
  window: { u_m: 2, sill_z_m: 1, width_m: 1.2, height_m: 1.5 },
  sensitive: false,
};

/**
 * A kernel that places at (1000, 2000) with a key from (0, 0) to (10, 0),
 * and a projection that multiplies by 1000 going out and divides coming
 * back, so a move in either direction is recognisable on sight.
 */
function fakeBackend() {
  const placed: PlaceWindowRequest[] = [];
  const transforms: TransformRequest[] = [];
  return {
    placed,
    transforms,
    backend: {
      placeWindow: vi.fn(
        (req: PlaceWindowRequest): Promise<PlaceWindowResponse> => {
          placed.push(req);
          return Promise.resolve({
            feature: {
              type: "Feature",
              id: req.window.id,
              properties: {
                id: req.window.id,
                kind: "window",
                facade: {
                  building_ref: "house",
                  part: 0,
                  ring: 0,
                  edge: 0,
                  from: [0, 0],
                  to: [10, 0],
                },
                window: req.window.window,
                sensitive: false,
                review_status: "proposed",
              },
              geometry: { type: "Point", coordinates: [1000, 2000] },
            },
          });
        },
      ),
      transformCoordinates: vi.fn(
        (req: TransformRequest): Promise<TransformResponse> => {
          transforms.push(req);
          const geographic = req.source_crs === "EPSG:4326";
          return Promise.resolve({
            source_crs: req.source_crs,
            target_crs: geographic ? "EPSG:25832" : req.target_crs,
            applied: true,
            coordinates: req.coordinates.map((v) =>
              geographic ? v * 1000 : v / 1000,
            ),
          });
        },
      ),
    },
  };
}

beforeEach(() => {
  resetComputeModelCacheForTests();
});

describe("placeWindowFeature", () => {
  it("places in the compute CRS and stores the window as the model holds one", async () => {
    const { placed, backend } = fakeBackend();
    backend.transformCoordinates.mockImplementation((req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.source_crs,
        applied: false,
        coordinates: req.coordinates,
      }),
    );

    const feature = await placeWindowFeature(
      backend,
      {
        features: [
          house([
            [0, 0],
            [10, 0],
            [10, 8],
            [0, 0],
          ]),
          {
            id: "other",
            kind: "building",
            heightM: 3,
            geometry: { type: "Polygon", coordinates: [] },
          },
        ],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      },
      placement,
    );

    expect(placed).toHaveLength(1);
    expect(placed[0]?.crs).toBe("EPSG:25832");
    expect(placed[0]?.window).toEqual(placement);
    expect(
      (placed[0]?.model as { features: { id: string }[] }).features.map(
        (f) => f.id,
      ),
    ).toEqual(["house"]);
    expect(feature).toEqual({
      id: "w1",
      kind: "window",
      properties: {
        facade: {
          building_ref: "house",
          part: 0,
          ring: 0,
          edge: 0,
          from: [0, 0],
          to: [10, 0],
        },
        window: placement.window,
        sensitive: false,
        review_status: "proposed",
      },
      geometry: { type: "Point", coordinates: [1000, 2000] },
    });
  });

  it("moves the answer back into a geographic workspace, key and point together", async () => {
    const { transforms, backend } = fakeBackend();

    const feature = await placeWindowFeature(
      backend,
      {
        features: [
          house([
            [9.7, 52.3],
            [9.71, 52.3],
            [9.7, 52.31],
            [9.7, 52.3],
          ]),
        ],
        receivers: [],
        calcArea: null,
        crs: "EPSG:4326",
      },
      placement,
    );

    expect(transforms.at(-1)).toMatchObject({
      source_crs: "EPSG:25832",
      target_crs: "EPSG:4326",
    });
    expect(feature.geometry.coordinates).toEqual([1, 2]);
    const facade = feature.properties?.facade as {
      from: number[];
      to: number[];
    };
    expect(facade.from).toEqual([0, 0]);
    expect(facade.to).toEqual([0.01, 0]);
  });

  it("passes a refusal through and places nothing", async () => {
    const { backend } = fakeBackend();
    backend.placeWindow.mockRejectedValueOnce(
      new Error('window "w1": u_m 9 + width_m 1.2 runs past the end'),
    );

    await expect(
      placeWindowFeature(
        backend,
        { features: [], receivers: [], calcArea: null, crs: "EPSG:25832" },
        placement,
      ),
    ).rejects.toThrow("runs past the end");
  });
});

describe("nextWindowId", () => {
  it("takes the first win-<n> no feature carries", () => {
    const f = (id: string): ModelFeature => ({
      id,
      kind: "window",
      geometry: { type: "Point", coordinates: [0, 0] },
    });
    expect(nextWindowId([])).toBe("win-1");
    expect(nextWindowId([f("win-1"), f("win-3")])).toBe("win-2");
  });
});
