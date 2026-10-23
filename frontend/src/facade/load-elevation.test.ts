import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetComputeModelCacheForTests } from "@/model/compute-crs";
import type { ModelFeature } from "@/model/types";
import type {
  FacadeElevationRequest,
  FacadeElevationResponse,
  TransformRequest,
  TransformResponse,
} from "@/wasm/types";
import { loadFacadeElevation, windowBuildingRef } from "./load-elevation";

const house: ModelFeature = {
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
};

function windowOn(id: string, buildingRef: string): ModelFeature {
  return {
    id,
    kind: "window",
    properties: {
      facade: { building_ref: buildingRef, part: 0, ring: 0, edge: 0 },
      window: { u_m: 1, sill_z_m: 1, width_m: 1, height_m: 1 },
    },
    geometry: { type: "Point", coordinates: [547001.5, 5795000] },
  };
}

const elevation: FacadeElevationResponse = {
  building_ref: "house",
  height_m: 9,
  slab_heights_m: [3, 6, 9],
  slab_height_source: "assumed",
  facades: [],
  detached: [],
};

function fakeBackend() {
  const asked: FacadeElevationRequest[] = [];
  const transforms: TransformRequest[] = [];
  return {
    asked,
    transforms,
    backend: {
      facadeElevation: vi.fn((req: FacadeElevationRequest) => {
        asked.push(req);
        return Promise.resolve(elevation);
      }),
      transformCoordinates: vi.fn(
        (req: TransformRequest): Promise<TransformResponse> => {
          transforms.push(req);
          return Promise.resolve({
            source_crs: req.source_crs,
            target_crs: "EPSG:25832",
            applied: true,
            coordinates: req.coordinates.map((v) => v * 100000),
          });
        },
      ),
    },
  };
}

beforeEach(() => {
  resetComputeModelCacheForTests();
});

describe("loadFacadeElevation", () => {
  it("sends the building and its own windows, in the compute CRS", async () => {
    const { asked, backend } = fakeBackend();
    const road: ModelFeature = {
      id: "road",
      kind: "source",
      sourceType: "line",
      geometry: {
        type: "LineString",
        coordinates: [
          [547000, 5794990],
          [547050, 5794990],
        ],
      },
    };

    const loaded = await loadFacadeElevation(
      backend,
      {
        features: [
          house,
          road,
          windowOn("w1", "house"),
          windowOn("w2", "shed"),
        ],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      },
      "house",
    );

    expect(loaded).toEqual({ elevation, computeCRS: "EPSG:25832" });
    expect(asked).toHaveLength(1);
    const [req] = asked;
    expect(req?.crs).toBe("EPSG:25832");
    expect(req?.building).toBe("house");
    const ids = (req?.model as { features: { id: string }[] }).features.map(
      (f) => f.id,
    );
    expect(ids).toEqual(["house", "w1"]);
  });

  it("sends the buildings that could stand against it, and no farther one", async () => {
    const { asked, backend } = fakeBackend();
    // Metres in, metres out: the reach is measured in the compute CRS.
    backend.transformCoordinates.mockImplementation((req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.source_crs,
        applied: false,
        coordinates: req.coordinates,
      }),
    );
    const box = (id: string, x0: number, x1: number): ModelFeature => ({
      id,
      kind: "building",
      heightM: 6,
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [x0, 5795000],
            [x1, 5795000],
            [x1, 5795008],
            [x0, 5795008],
            [x0, 5795000],
          ],
        ],
      },
    });

    await loadFacadeElevation(
      backend,
      {
        features: [
          box("far", 547015, 547025),
          house,
          box("next", 547010, 547014),
          box("close", 546990, 546999.5),
          windowOn("w-next", "next"),
        ],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      },
      "house",
    );

    const ids = (
      asked[0]?.model as { features: { id: string }[] }
    ).features.map((f) => f.id);
    expect(ids).toEqual(["house", "next", "close"]);
  });

  it("projects a geographic workspace first, as a run does", async () => {
    const { asked, transforms, backend } = fakeBackend();
    const lonLat: ModelFeature = {
      ...house,
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [5.47, 57.95],
            [5.4701, 57.95],
            [5.4701, 57.9501],
            [5.47, 57.95],
          ],
        ],
      },
    };

    const loaded = await loadFacadeElevation(
      backend,
      { features: [lonLat], receivers: [], calcArea: null, crs: "EPSG:4326" },
      "house",
    );

    expect(transforms.length).toBeGreaterThan(0);
    expect(transforms[0]?.target_crs).toBe("auto");
    expect(loaded.computeCRS).toBe("EPSG:25832");
    expect(asked[0]?.crs).toBe("EPSG:25832");
  });

  it("passes a refusal through", async () => {
    const { backend } = fakeBackend();
    backend.facadeElevation.mockRejectedValueOnce(
      new Error('building "house" is not in the model'),
    );

    await expect(
      loadFacadeElevation(
        backend,
        { features: [], receivers: [], calcArea: null, crs: "EPSG:25832" },
        "house",
      ),
    ).rejects.toThrow("not in the model");
  });
});

describe("windowBuildingRef", () => {
  it("reads the building a window's facade key names", () => {
    expect(windowBuildingRef(windowOn("w", "house"))).toBe("house");
    expect(windowBuildingRef({ ...house, properties: {} })).toBeNull();
    expect(
      windowBuildingRef({
        ...house,
        properties: { facade: { building_ref: 3 } },
      }),
    ).toBeNull();
  });
});
