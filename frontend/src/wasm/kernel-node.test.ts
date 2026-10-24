// Health check for the Node-side kernel loader.
//
// It is deliberately separate from kernel-parity.test.ts: when the loader
// breaks, this is one obvious failure rather than thirty-six confusing ones.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "./kernel-node";

const skipReason = kernelSkipReason();

/** Read a file from the repository, relative to this test's directory. */
function repoFile(relative: string): string {
  return readFileSync(
    fileURLToPath(new URL(relative, import.meta.url)),
    "utf8",
  );
}

/**
 * The scalars `road.DefaultPropagationConfig` returns, parsed out of the Go
 * source. The kernel's `defaultConfig()` is the browser's only view of them, so
 * a change on the Go side that never reaches the WASM build shows up here.
 */
function goDefaultConfig(): Record<string, number> {
  const source = repoFile(
    "../../../backend/internal/standards/rls19/road/propagation.go",
  );

  const body = /func DefaultPropagationConfig\(\)[^{]*\{[^}]*\{([^}]*)\}/.exec(
    source,
  );
  const fields = body?.[1];
  if (fields === undefined) {
    throw new Error("DefaultPropagationConfig not found in propagation.go");
  }

  const defaults: Record<string, number> = {};
  for (const match of fields.matchAll(/(\w+):\s*([0-9]+(?:\.[0-9]+)?)\s*,/g)) {
    const [, name, value] = match;
    if (name !== undefined && value !== undefined) {
      defaults[name] = Number(value);
    }
  }

  return defaults;
}

describe.skipIf(skipReason !== null)("Node WASM kernel loader", () => {
  it("returns the same defaults the Go source declares", async () => {
    const kernel = await getNodeKernel();
    const defaults = await kernel.defaultConfig();
    const want = goDefaultConfig();

    expect(Object.keys(want).length).toBeGreaterThan(0);
    for (const [name, value] of Object.entries(want)) {
      expect(defaults[name as keyof typeof defaults]).toBe(value);
    }
  });

  it("computes a level for a well-formed request", async () => {
    const kernel = await getNodeKernel();

    const outputs = await kernel.rls19Road({
      receivers: [{ id: "r1", point: { x: 0, y: 50 }, height_m: 4 }],
      sources: [
        {
          id: "road",
          centerline: [
            { x: -50, y: 0 },
            { x: 50, y: 0 },
          ],
          surface_type: "SMA",
          speeds: {
            pkw_kph: 100,
            lkw1_kph: 80,
            lkw2_kph: 70,
            krad_kph: 100,
          },
          traffic_day: {
            pkw_per_hour: 900,
            lkw1_per_hour: 40,
            lkw2_per_hour: 60,
            krad_per_hour: 10,
          },
          traffic_night: {
            pkw_per_hour: 200,
            lkw1_per_hour: 10,
            lkw2_per_hour: 20,
            krad_per_hour: 2,
          },
        },
      ],
      barriers: [],
      config: { SegmentLengthM: 5, MinDistanceM: 3, ReceiverHeightM: 4 },
    });

    expect(outputs).toHaveLength(1);
    expect(outputs[0]?.Indicators.lr_day).toBeGreaterThan(
      outputs[0]?.Indicators.lr_night ?? 0,
    );
  });

  it("estimates a run of the same request without computing it", async () => {
    const kernel = await getNodeKernel();

    const estimate = await kernel.estimate(
      {
        receivers: [
          { id: "r1", point: { x: 0, y: 50 }, height_m: 4 },
          { id: "r2", point: { x: 20, y: 80 }, height_m: 4 },
        ],
        sources: [
          {
            id: "road",
            centerline: [
              { x: -50, y: 0 },
              { x: 50, y: 0 },
            ],
            surface_type: "SMA",
            speeds: { pkw_kph: 100, lkw1_kph: 80, lkw2_kph: 70, krad_kph: 100 },
            traffic_day: {
              pkw_per_hour: 900,
              lkw1_per_hour: 40,
              lkw2_per_hour: 60,
              krad_per_hour: 10,
            },
            traffic_night: {
              pkw_per_hour: 200,
              lkw1_per_hour: 10,
              lkw2_per_hour: 20,
              krad_per_hour: 2,
            },
          },
        ],
        barriers: [],
        config: { SegmentLengthM: 5, MinDistanceM: 3, ReceiverHeightM: 4 },
      },
      { workers: 2, budgetMs: 100 },
    );

    // The same JSON `aconiq run --estimate --json` prints, from the same Go.
    expect(estimate.available).toBe(true);
    expect(estimate.standard_id).toBe("rls19-road");
    expect(estimate.inputs).toMatchObject({ receivers: 2, workers: 2 });
    // 100 m at 5 m: twenty Teilstücke for each of the two receivers.
    expect(estimate.inputs?.units).toBe(40);
    expect(estimate.wall_s?.high).toBeGreaterThan(0);
    // Fixed mode, so the distance-scaled run is costed beside it.
    expect(estimate.alternatives?.[0]?.params.segment_length_mode).toBe(
      "distance_scaled",
    );
  });

  // A synthetic 10 m × 8 m house of three storeys in EPSG:25832, addressed so
  // the request can name it the way a user types it.
  const facadeModel = {
    type: "FeatureCollection",
    features: [
      {
        type: "Feature",
        id: "house",
        properties: {
          kind: "building",
          height_m: 9,
          "building:levels": "3",
          "addr:street": "Musterstraße",
          "addr:housenumber": "1",
        },
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
  };

  it("places facade receivers through the kernel, keyed to their building", async () => {
    const kernel = await getNodeKernel();

    const placed = await kernel.facadeReceivers({
      crs: "EPSG:25832",
      model: facadeModel,
      standard_id: "rls19-road",
      buildings: ["musterstraße 1"],
      spacing_m: 4,
      allow_unconfirmed: true,
    });

    expect(placed.receivers.length).toBeGreaterThan(0);
    expect(placed.proposed_count).toBe(placed.receivers.length);
    expect(placed.metadata["allow_unconfirmed"]).toBe("true");
    for (const receiver of placed.receivers) {
      expect(receiver.placement_rule).toBe("rls19-facade");
      expect(receiver.facade.building_ref).toBe("house");
      expect(receiver.facade.from).toHaveLength(2);
      expect(receiver.storey).toBeGreaterThanOrEqual(0);
    }
  });

  it("unfolds a building into its facades and storeys through the kernel", async () => {
    const kernel = await getNodeKernel();

    const elevation = await kernel.facadeElevation({
      crs: "EPSG:25832",
      model: facadeModel,
      building: "house",
    });

    expect(elevation.building_ref).toBe("house");
    expect(elevation.slab_heights_m).toEqual([3, 6, 9]);
    expect(elevation.facades.map((f) => f.length_m)).toEqual([10, 8, 10, 8]);
    expect(elevation.facades[0]?.facing_deg).toBe(180);
    expect(elevation.detached).toEqual([]);
    await expect(
      kernel.facadeElevation({
        crs: "EPSG:25832",
        model: facadeModel,
        building: "shed",
      }),
    ).rejects.toThrow("not in the model");
  });

  it("places a window drawn in the elevation view through the kernel", async () => {
    const kernel = await getNodeKernel();

    const { feature } = await kernel.placeWindow({
      crs: "EPSG:25832",
      model: facadeModel,
      window: {
        id: "drawn",
        building_ref: "house",
        part: 0,
        ring: 0,
        edge: 0,
        window: { u_m: 2, sill_z_m: 3.9, width_m: 1.2, height_m: 1.5 },
        sensitive: false,
      },
    });

    expect(feature.id).toBe("drawn");
    expect(feature.properties.kind).toBe("window");
    expect(feature.properties.review_status).toBe("proposed");
    expect(feature.properties.facade).toMatchObject({
      building_ref: "house",
      from: [547000, 5795000],
      to: [547010, 5795000],
    });
    expect(feature.geometry.coordinates[0]).toBeCloseTo(547002.6, 9);
    await expect(
      kernel.placeWindow({
        crs: "EPSG:25832",
        model: facadeModel,
        window: {
          id: "drawn",
          building_ref: "house",
          part: 0,
          ring: 0,
          edge: 0,
          window: { u_m: 9.5, sill_z_m: 1, width_m: 1.2, height_m: 1.5 },
          sensitive: false,
        },
      }),
    ).rejects.toThrow("runs past the end");
  });

  it("places a sensitive window's point in place of its facade's stations", async () => {
    const kernel = await getNodeKernel();

    const placed = await kernel.facadeReceivers({
      crs: "EPSG:25832",
      model: {
        ...facadeModel,
        features: [
          ...facadeModel.features,
          {
            type: "Feature",
            id: "bedroom",
            properties: {
              kind: "window",
              facade: {
                building_ref: "house",
                part: 0,
                ring: 0,
                edge: 0,
                from: [547000, 5795000],
                to: [547010, 5795000],
              },
              window: { u_m: 2, sill_z_m: 3.9, width_m: 1.2, height_m: 1.5 },
              sensitive: true,
              review_status: "confirmed",
            },
            geometry: { type: "Point", coordinates: [547002.6, 5795000] },
          },
        ],
      },
      standard_id: "rls19-road",
      buildings: ["house"],
      spacing_m: 4,
      allow_unconfirmed: true,
    });

    const south = placed.receivers.filter(
      (receiver) => receiver.facade.edge === 0,
    );
    expect(south).toHaveLength(1);
    expect(south[0]).toMatchObject({
      id: "facade/house/p0r0e0/w/bedroom",
      window_id: "bedroom",
      height_basis: "schall03-window-head",
      review_status: "confirmed",
    });
    expect(south[0]?.height_m).toBeCloseTo(5.6, 9);
    expect(placed.metadata["facade_window_count"]).toBe("1");
  });

  it("refuses an unconfirmed normative facade run in the run's words", async () => {
    const kernel = await getNodeKernel();

    await expect(
      kernel.facadeReceivers({
        crs: "EPSG:25832",
        model: facadeModel,
        standard_id: "rls19-road",
        buildings: ["house"],
        spacing_m: 4,
      }),
    ).rejects.toThrow(/--allow-unconfirmed/);
  });

  // Hand-placed facade points, as a person keeps them after reviewing what the
  // kernel placed: the confirmed one is computed with its table attributes, the
  // rejected one never enters the run, and a proposed one refuses a normative
  // run without the opt-in.
  it("decides which custom receivers a run keeps, as `aconiq run` does", async () => {
    const kernel = await getNodeKernel();
    const placed = await kernel.facadeReceivers({
      crs: "EPSG:25832",
      model: facadeModel,
      standard_id: "rls19-road",
      buildings: ["house"],
      spacing_m: 4,
      allow_unconfirmed: true,
    });
    const [first, second] = placed.receivers;
    if (first === undefined || second === undefined) {
      throw new Error("expected at least two facade receivers");
    }

    const asFeature = (receiver: typeof first, status: string) => ({
      type: "Feature",
      id: receiver.id,
      properties: {
        kind: "receiver",
        height_m: receiver.height_m,
        placement_rule: receiver.placement_rule,
        facade: receiver.facade,
        storey: receiver.storey,
        slab_height_source: receiver.slab_height_source,
        review_status: status,
      },
      geometry: {
        type: "Point",
        coordinates: [receiver.point.x, receiver.point.y],
      },
    });
    const reviewed = (firstStatus: string) => ({
      type: "FeatureCollection",
      features: [
        ...facadeModel.features,
        asFeature(first, firstStatus),
        asFeature(second, "rejected"),
      ],
    });
    const receivers = [first, second].map(({ id, point, height_m }) => ({
      id,
      point,
      height_m,
    }));

    const kept = await kernel.customReceivers({
      crs: "EPSG:25832",
      model: reviewed("confirmed"),
      standard_id: "rls19-road",
      receivers,
    });

    expect(kept.receivers.map((receiver) => receiver.id)).toEqual([first.id]);
    expect(kept.rejected_count).toBe(1);
    expect(kept.proposed_count).toBe(0);
    expect(kept.receivers[0]?.facade).toEqual(first.facade);
    expect(kept.receivers[0]?.placement_rule).toBe("rls19-facade");

    await expect(
      kernel.customReceivers({
        crs: "EPSG:25832",
        model: reviewed("proposed"),
        standard_id: "rls19-road",
        receivers,
      }),
    ).rejects.toThrow(/--allow-unconfirmed/);
  });

  it("rejects a malformed request rather than answering it", async () => {
    const kernel = await getNodeKernel();

    // A source with no centerline cannot be validated into a scene; the kernel
    // must say so instead of returning a level for it.
    await expect(
      kernel.rls19Road({
        receivers: [{ id: "r1", point: { x: 0, y: 50 }, height_m: 4 }],
        sources: [{ id: "bad" } as never],
        barriers: [],
        config: { SegmentLengthM: 1, MinDistanceM: 3, ReceiverHeightM: 4 },
      }),
    ).rejects.toBeTruthy();
  });
});

if (skipReason !== null) {
  describe("Node WASM kernel loader", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the vitest output.
    });
  });
}
