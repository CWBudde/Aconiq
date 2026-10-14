// The receiver grid descriptor, against the real kernel.
//
// A grid run names its receivers twice: the kernel expands the descriptor in
// Go (`wasmkernel.ReceiverGrid.Receivers`), and the browser names the same
// receivers in TypeScript (`@/model/receiver-grid`) for the run's table and
// hash. This holds the two to each other through the real WASM build, over a
// grid past the 10 000th receiver, where the ids grow a fifth digit.

import { describe, expect, it } from "vitest";

import { materializeGrid } from "@/model/receiver-grid";
import { getNodeKernel, kernelSkipReason } from "./kernel-node";
import type {
  ComputeRequest,
  GridComputeRequest,
  ReceiverGrid,
  RoadSource,
} from "./types";

const skipReason = kernelSkipReason();

// 101 × 100 = 10 100 receivers, so R10000 to R10100 are in it. One short road
// in one Teilstück keeps the walk cheap; the subject is the receivers, not the
// levels. The origin and the resolution are inexact, so every coordinate past
// cell (0,0) is a rounded product.
const grid: ReceiverGrid = {
  origin_x: 412_345.678,
  origin_y: 5_800_000.25,
  resolution_m: 7.7,
  width: 101,
  height: 100,
  height_m: 4,
};

const road: RoadSource = {
  id: "road",
  surface_type: "SMA",
  centerline: [
    { x: 412_700, y: 5_800_380 },
    { x: 412_720, y: 5_800_380 },
  ],
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
};

const scene = {
  sources: [road],
  barriers: [],
  config: { SegmentLengthM: 20, MinDistanceM: 3, ReceiverHeightM: 4 },
};

describe.skipIf(skipReason !== null)("receiver grid descriptor", () => {
  it("expands in the kernel to the receivers the browser names", async () => {
    const kernel = await getNodeKernel();

    // rls19Road echoes every receiver, which is what makes it the reference:
    // sent the descriptor, it answers with the kernel's own expansion.
    const echoed = await kernel.rls19Road({
      ...scene,
      grid,
    } as unknown as ComputeRequest);

    const named = materializeGrid(grid);
    expect(echoed.map((output) => output.Receiver)).toEqual(named);
    expect(named[9999]?.id).toBe("R10000");
    expect(named.at(-1)?.id).toBe("R10100");
  });

  it("answers rls19RoadGrid with the indicators of the receiver-list run", async () => {
    const kernel = await getNodeKernel();

    const listed = await kernel.rls19Road({
      ...scene,
      receivers: materializeGrid(grid),
    });
    const described = await kernel.rls19RoadGrid({
      ...scene,
      grid,
    } as GridComputeRequest);

    expect(described).toEqual(listed.map((output) => output.Indicators));
  });
});
