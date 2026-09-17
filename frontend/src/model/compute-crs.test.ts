import { describe, expect, it, vi } from "vitest";

import {
  projectGeometry,
  projectWorkspace,
  resolveComputeModel,
} from "./compute-crs";
import type { Workspace } from "./compute-crs";
import type { Geometry, ModelFeature } from "./types";
import type { TransformRequest, TransformResponse } from "@/wasm/types";

/**
 * A kernel stand-in that records what it was asked and answers with whatever
 * the test wants back. The real projection is the parity suite's business
 * (`api/browser-parity.test.ts`, against the real WASM kernel); what this file
 * pins is the traversal — which coordinates are collected, in which order, and
 * where the answer is put back.
 */
function fakeKernel(respond: (req: TransformRequest) => TransformResponse): {
  transform: (req: TransformRequest) => Promise<TransformResponse>;
  requests: TransformRequest[];
} {
  const requests: TransformRequest[] = [];
  return {
    requests,
    transform: (req) => {
      requests.push(req);
      return Promise.resolve(respond(req));
    },
  };
}

/** Answers by adding a fixed offset, so a moved coordinate is recognisable. */
function shiftBy(dx: number, dy: number) {
  return (req: TransformRequest): TransformResponse => ({
    source_crs: req.source_crs,
    target_crs: "EPSG:25832",
    applied: true,
    coordinates: req.coordinates.map((value, index) =>
      index % 2 === 0 ? value + dx : value + dy,
    ),
  });
}

function unmoved(req: TransformRequest): TransformResponse {
  return {
    source_crs: req.source_crs,
    target_crs: req.source_crs,
    applied: false,
    coordinates: req.coordinates,
  };
}

const LINE: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  geometry: {
    type: "LineString",
    coordinates: [
      [1, 2],
      [3, 4],
    ],
  },
};

const POLYGON: ModelFeature = {
  id: "block",
  kind: "building",
  heightM: 10,
  geometry: {
    type: "MultiPolygon",
    coordinates: [
      [
        [
          [10, 11],
          [12, 13],
          [14, 15],
          [10, 11],
        ],
      ],
    ],
  },
};

function workspace(overrides: Partial<Workspace> = {}): Workspace {
  return {
    features: [LINE, POLYGON],
    receivers: [
      {
        id: "R1",
        heightM: 4,
        geometry: { type: "Point", coordinates: [5, 6] },
      },
    ],
    calcArea: {
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [20, 21],
            [22, 23],
            [24, 25],
            [20, 21],
          ],
        ],
      },
    },
    crs: "EPSG:4326",
    ...overrides,
  };
}

describe("resolveComputeModel", () => {
  it("sends every coordinate once, flat and interleaved, in model order", async () => {
    const kernel = fakeKernel(unmoved);

    await resolveComputeModel(kernel, workspace());

    // Features in array order, then receivers, then the calculation area's
    // rings — the same order `geo/modelgeojson/reproject.go` walks.
    expect(kernel.requests).toHaveLength(1);
    expect(kernel.requests[0]?.coordinates).toEqual([
      1, 2, 3, 4, 10, 11, 12, 13, 14, 15, 10, 11, 5, 6, 20, 21, 22, 23, 24, 25,
      20, 21,
    ]);
    expect(kernel.requests[0]?.source_crs).toBe("EPSG:4326");
    expect(kernel.requests[0]?.target_crs).toBe("auto");
  });

  it("puts the answer back where it came from", async () => {
    const kernel = fakeKernel(shiftBy(1000, 2000));

    const model = await resolveComputeModel(kernel, workspace());

    expect(model.features[0]?.geometry.coordinates).toEqual([
      [1001, 2002],
      [1003, 2004],
    ]);
    expect(model.features[1]?.geometry.coordinates).toEqual([
      [
        [
          [1010, 2011],
          [1012, 2013],
          [1014, 2015],
          [1010, 2011],
        ],
      ],
    ]);
    expect(model.receivers[0]?.geometry.coordinates).toEqual([1005, 2006]);
    expect(model.calcArea?.geometry.coordinates).toEqual([
      [
        [1020, 2021],
        [1022, 2023],
        [1024, 2025],
        [1020, 2021],
      ],
    ]);
    expect(model.projection).toEqual({
      projectCRS: "EPSG:4326",
      computeCRS: "EPSG:25832",
      applied: true,
    });
  });

  it("returns the workspace untouched when nothing moved", async () => {
    const kernel = fakeKernel(unmoved);
    const input = workspace({ crs: "EPSG:25832" });

    const model = await resolveComputeModel(kernel, input);

    // Identity, not a deep copy that happens to be equal: a model already in a
    // projected CRS runs through exactly the code it ran through before.
    expect(model.features).toBe(input.features);
    expect(model.receivers).toBe(input.receivers);
    expect(model.calcArea).toBe(input.calcArea);
    expect(model.projection.applied).toBe(false);
    expect(model.projection.computeCRS).toBe("EPSG:25832");
  });

  it("carries a third ordinate through untouched", async () => {
    const kernel = fakeKernel(shiftBy(100, 200));

    const model = await resolveComputeModel(
      kernel,
      workspace({
        features: [
          {
            ...LINE,
            geometry: {
              type: "LineString",
              // An elevation in metres, which no horizontal transform moves.
              coordinates: [[1, 2, 55] as unknown as [number, number]],
            },
          },
        ],
        receivers: [],
        calcArea: null,
      }),
    );

    expect(model.features[0]?.geometry.coordinates).toEqual([[101, 202, 55]]);
  });

  it("refuses a feature whose coordinates live in a property", async () => {
    const kernel = fakeKernel(unmoved);
    const transform = vi.spyOn(kernel, "transform");

    await expect(
      resolveComputeModel(
        kernel,
        workspace({
          features: [
            {
              ...LINE,
              // `geo/modelgeojson/reproject.go` moves these alongside the
              // geometry; browser mode cannot reach them, and leaving them in
              // degrees inside a metric model puts a directional source
              // millions of metres from its own receivers.
              properties: { rls19_directional_sources: [] },
            },
          ],
        }),
      ),
    ).rejects.toThrow(/rls19_directional_sources/);

    // Refused before anything was projected, so there is no half-moved model.
    expect(transform).not.toHaveBeenCalled();
  });

  it("refuses a schall03 track feature for the same reason", async () => {
    const kernel = fakeKernel(unmoved);

    await expect(
      resolveComputeModel(
        kernel,
        workspace({
          features: [
            { ...LINE, properties: { schall03_track_features: [{ x: 1 }] } },
          ],
        }),
      ),
    ).rejects.toThrow(/schall03_track_features/);
  });

  it("refuses an answer that is not the batch it sent", async () => {
    // A short answer would otherwise scatter NaN into the model and compute a
    // level over it.
    const kernel = fakeKernel((req) => ({
      source_crs: req.source_crs,
      target_crs: "EPSG:25832",
      applied: true,
      coordinates: req.coordinates.slice(0, 2),
    }));

    await expect(resolveComputeModel(kernel, workspace())).rejects.toThrow(
      /was not projected/,
    );
  });

  it("projects an empty workspace without asking for trouble", async () => {
    const kernel = fakeKernel(unmoved);

    const model = await resolveComputeModel(
      kernel,
      workspace({ features: [], receivers: [], calcArea: null }),
    );

    expect(kernel.requests[0]?.coordinates).toEqual([]);
    expect(model.features).toEqual([]);
  });
});

describe("projectWorkspace", () => {
  it("asks for the target it was given, not for auto", async () => {
    // `wasmkernel.resolveTarget` transforms unconditionally on an explicit
    // target and only consults the coordinates for "auto" — which is what
    // makes the inverse direction, the one the map needs, free.
    const kernel = fakeKernel(unmoved);

    await projectWorkspace(
      kernel.transform,
      workspace({ crs: "EPSG:25832" }),
      "EPSG:4326",
    );

    expect(kernel.requests).toHaveLength(1);
    expect(kernel.requests[0]?.source_crs).toBe("EPSG:25832");
    expect(kernel.requests[0]?.target_crs).toBe("EPSG:4326");
  });

  it("leaves the workspace it was handed untouched", async () => {
    // The map projects the store's own arrays. Mutating them there would make
    // the map a source for the model instead of a projection of it.
    const kernel = fakeKernel(shiftBy(1000, 2000));
    const input = workspace();
    const before = structuredClone(input);

    await projectWorkspace(kernel.transform, input, "EPSG:4326");

    expect(input).toEqual(before);
    expect(input.features[0]).toBe(LINE);
  });

  it("does not refuse geometry carried in a property", async () => {
    // The refusal belongs to `resolveComputeModel`, because it is about
    // computing. `ModelLayers` never draws `rls19_directional_sources`, and
    // throwing here would blank the map for a model whose geometry projects
    // perfectly well.
    const kernel = fakeKernel(shiftBy(1, 1));

    const model = await projectWorkspace(
      kernel.transform,
      workspace({
        features: [{ ...LINE, properties: { rls19_directional_sources: [] } }],
      }),
      "EPSG:4326",
    );

    expect(model.features[0]?.geometry.coordinates).toEqual([
      [2, 3],
      [4, 5],
    ]);
  });
});

describe("projectGeometry", () => {
  const RING: Geometry = {
    type: "Polygon",
    coordinates: [
      [
        [1, 2],
        [3, 4],
        [5, 6],
        [1, 2],
      ],
    ],
  };

  it("walks one geometry's coordinate tree in the order a workspace walk does", async () => {
    // The same collect/scatter, so the map's inverse direction cannot come to
    // disagree with the forward one about where a coordinate belongs.
    const kernel = fakeKernel(shiftBy(100, 200));

    const projected = await projectGeometry(
      kernel.transform,
      RING,
      "EPSG:4326",
      "EPSG:25832",
    );

    expect(kernel.requests[0]?.coordinates).toEqual([1, 2, 3, 4, 5, 6, 1, 2]);
    expect(projected.geometry.coordinates).toEqual([
      [
        [101, 202],
        [103, 204],
        [105, 206],
        [101, 202],
      ],
    ]);
  });

  it("asks for the target it was given, in the direction it was given", async () => {
    const kernel = fakeKernel(shiftBy(0, 0));

    await projectGeometry(kernel.transform, RING, "EPSG:4326", "EPSG:25832");

    expect(kernel.requests).toHaveLength(1);
    expect(kernel.requests[0]?.source_crs).toBe("EPSG:4326");
    expect(kernel.requests[0]?.target_crs).toBe("EPSG:25832");
  });

  it("leaves the geometry it was handed untouched", async () => {
    const kernel = fakeKernel(shiftBy(7, 8));
    const before = structuredClone(RING);

    const projected = await projectGeometry(
      kernel.transform,
      RING,
      "EPSG:4326",
      "EPSG:25832",
    );

    expect(RING).toEqual(before);
    expect(projected.geometry).not.toBe(RING);
  });

  it("hands back the very same geometry when nothing moved", async () => {
    // `applied: false` means the coordinates came back verbatim, and rebuilding
    // the tree from them would allocate a copy that is equal but not identical.
    const kernel = fakeKernel(unmoved);

    const projected = await projectGeometry(
      kernel.transform,
      RING,
      "EPSG:4326",
      "EPSG:4326",
    );

    expect(projected.geometry).toBe(RING);
    expect(projected.projection.applied).toBe(false);
  });

  it("refuses an answer of the wrong length rather than scattering NaN", async () => {
    const kernel = fakeKernel((req) => ({
      source_crs: req.source_crs,
      target_crs: "EPSG:25832",
      applied: true,
      coordinates: req.coordinates.slice(0, 2),
    }));

    await expect(
      projectGeometry(kernel.transform, RING, "EPSG:4326", "EPSG:25832"),
    ).rejects.toThrow("was not projected");
  });

  it("carries a third ordinate through untouched", async () => {
    // An absolute elevation in metres, which no horizontal projection moves.
    const kernel = fakeKernel(shiftBy(10, 20));

    const projected = await projectGeometry(
      kernel.transform,
      { type: "Point", coordinates: [1, 2, 33] } as unknown as Geometry,
      "EPSG:4326",
      "EPSG:25832",
    );

    expect(projected.geometry.coordinates).toEqual([11, 22, 33]);
  });
});
