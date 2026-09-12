// Browser-CLI parity, layer 2: does browser mode build the same scene out of a
// model that the CLI builds?
//
// kernel-parity.test.ts already settles that the WASM build computes what the
// native build computes, over the whole CI-safe acceptance suite. So a failure
// here can only mean the model → scene translation drifted — which is where the
// defects have actually been. buildBuildings dropped every MultiPolygon
// building and PolygonCentroid ignored polygon holes; both survived a full
// name-level parity suite, and both are encoded in the fixtures below.
//
// Nothing here re-implements browser-backend.ts. Only `getKernel` is mocked, so
// buildRoadSources, buildBarriers, buildBuildings, buildParkingSources and
// startRun itself all run for real against the real kernel.
//
// The fixtures and goldens belong to the Go tree — see
// backend/internal/app/cli/parity_golden_test.go, which writes them through the
// CLI's own extraction. `just update-golden` regenerates them.

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { getNodeKernel, kernelSkipReason } from "@/wasm/kernel-node";

// Hoisted: vi.mock is lifted above the imports, so the factory cannot close
// over anything declared with const/let at module scope.
vi.mock("@/wasm/kernel", async () => {
  const { getNodeKernel: load } = await import("@/wasm/kernel-node");
  return { getKernel: load };
});

import { browserBackend } from "./browser-backend";
import { useModelStore } from "@/model/model-store";
import { normalizeGeoJSON } from "@/model/normalize";
import type {
  GeoJSONFeatureCollection,
  ModelReceiver,
  Position,
} from "@/model/types";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses.
const PARITY_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/app/cli/testdata/parity",
);

const skipReason = kernelSkipReason();

/**
 * parityRunParams in parity_golden_test.go. Every value is stated rather than
 * defaulted on both sides, because a default that differs between the targets is
 * exactly what this test exists to catch.
 */
const RUN_PARAMS: Record<string, string> = {
  surface_type: "SMA",
  speed_pkw_kph: "100",
  speed_lkw1_kph: "80",
  speed_lkw2_kph: "70",
  speed_krad_kph: "100",
  gradient_percent: "0",
  traffic_day_pkw: "900",
  traffic_day_lkw1: "40",
  traffic_day_lkw2: "60",
  traffic_day_krad: "10",
  traffic_night_pkw: "200",
  traffic_night_lkw1: "10",
  traffic_night_lkw2: "20",
  traffic_night_krad: "2",
  segment_length_m: "5",
  min_distance_m: "3",
  receiver_height_m: "4",
  grid_resolution_m: "10",
  grid_padding_m: "50",
};

const FIXTURES = ["road_building_barrier", "parking_building"] as const;

interface ReceiverSnapshot {
  id: string;
  x: number;
  y: number;
  height_m: number;
  lr_day: number;
  lr_night: number;
}

interface ReceiverTable {
  records: {
    id: string;
    values: Record<string, number | undefined>;
  }[];
}

function parityFile(name: string): string {
  return readFileSync(resolve(PARITY_DIR, name), "utf8");
}

/**
 * The frontend normalizer accepts only source/building/barrier — receivers live
 * in their own store rather than in the feature list — so they are lifted out of
 * the same GeoJSON here, in file order. The ids come from the feature-level
 * `id`, which is what the CLI's normalizer also resolves them to, so the two
 * targets name the same points identically.
 */
function receiversFrom(collection: GeoJSONFeatureCollection): ModelReceiver[] {
  const receivers: ModelReceiver[] = [];

  for (const feature of collection.features) {
    if (feature.properties["kind"] !== "receiver") continue;
    if (feature.geometry.type !== "Point") continue;

    receivers.push({
      id: String(feature.id),
      heightM: Number(feature.properties["height_m"]),
      geometry: {
        type: "Point",
        coordinates: feature.geometry.coordinates as Position,
      },
    });
  }

  return receivers;
}

function round6(value: number): number {
  return Math.round(value * 1e6) / 1e6;
}

describe.skipIf(skipReason !== null)("browser run path vs. CLI goldens", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useModelStore.setState({ features: [], receivers: [], calcArea: null });
  });

  it("loads the kernel through the mocked production entry point", async () => {
    const { getKernel } = await import("@/wasm/kernel");
    await expect(getKernel()).resolves.toBe(await getNodeKernel());
  });

  it.each(FIXTURES)("%s", async (name) => {
    const collection = JSON.parse(
      parityFile(`${name}.geojson`),
    ) as GeoJSONFeatureCollection;
    const expected = JSON.parse(parityFile(`${name}.golden.json`)) as {
      receivers: ReceiverSnapshot[];
    };

    const { features, skipped } = normalizeGeoJSON(collection);

    // A silently skipped source or building would make the browser compute a
    // smaller scene and the comparison would then be measuring the wrong thing.
    const unexpected = skipped.filter(
      (entry) => !entry.reason.includes('unknown kind "receiver"'),
    );
    expect(unexpected).toEqual([]);

    useModelStore.setState({
      features,
      receivers: receiversFrom(collection),
      calcArea: null,
    });

    const run = await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: RUN_PARAMS,
      receiverMode: "custom",
    });

    expect(run.status).toBe("completed");

    const tableArtifact = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.receiver_table_json",
    );
    expect(tableArtifact).toBeDefined();

    const table = await browserBackend.getArtifactContent<ReceiverTable>(
      tableArtifact?.id ?? "",
    );

    expect(table.records).toHaveLength(expected.receivers.length);

    // Matched by id, not by row: the two targets order the receiver table
    // differently — see the ordering test below. The levels are the claim here.
    const byID = new Map(table.records.map((record) => [record.id, record]));
    expect([...byID.keys()].sort()).toEqual(
      expected.receivers.map((receiver) => receiver.id).sort(),
    );

    for (const want of expected.receivers) {
      const record = byID.get(want.id);
      expect(record, `no browser record for receiver ${want.id}`).toBeDefined();
      expect(
        Math.abs(round6(record?.values["LrDay"] ?? NaN) - want.lr_day),
      ).toBeLessThanOrEqual(1e-6);
      expect(
        Math.abs(round6(record?.values["LrNight"] ?? NaN) - want.lr_night),
      ).toBeLessThanOrEqual(1e-6);
    }
  });

  // Found by the comparison above, and left standing deliberately rather than
  // papered over: in `custom` receiver mode the CLI keeps the receivers in model
  // order (extractExplicitReceivers walks model.Features) while browser mode
  // sorts them by id. The levels agree receiver for receiver, so this is not a
  // computation defect — but the receiver table's row order and therefore the
  // run's output_hash differ between the two targets for the same model.
  //
  // Picking a winner changes the hash of every browser run that has already been
  // made, so it is a decision rather than a fix. This test records the current
  // behaviour so the difference cannot quietly change shape in the meantime, and
  // PLAN.md carries the open item.
  it("orders the receiver table by id, where the CLI keeps model order", async () => {
    const collection = JSON.parse(
      parityFile("road_building_barrier.geojson"),
    ) as GeoJSONFeatureCollection;
    const golden = JSON.parse(
      parityFile("road_building_barrier.golden.json"),
    ) as { receivers: ReceiverSnapshot[] };

    useModelStore.setState({
      features: normalizeGeoJSON(collection).features,
      receivers: receiversFrom(collection),
      calcArea: null,
    });

    const run = await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: RUN_PARAMS,
      receiverMode: "custom",
    });

    const artifact = run.artifacts.find(
      (entry) => entry.kind === "run.result.receiver_table_json",
    );
    const table = await browserBackend.getArtifactContent<ReceiverTable>(
      artifact?.id ?? "",
    );

    const browserOrder = table.records.map((record) => record.id);
    const cliOrder = golden.receivers.map((receiver) => receiver.id);

    expect(browserOrder).toEqual(
      [...cliOrder].sort((a, b) => a.localeCompare(b)),
    );
    // The fixture is chosen so the two orders genuinely differ; if they ever
    // coincide, this test stops proving anything.
    expect(browserOrder).not.toEqual(cliOrder);
  });
});

if (skipReason !== null) {
  describe("browser run path vs. CLI goldens", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the runner output.
    });
  });
}
