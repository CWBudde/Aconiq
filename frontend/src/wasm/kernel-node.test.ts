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
