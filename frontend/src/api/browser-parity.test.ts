// Browser-CLI parity, layer 2: does browser mode build the same scene out of a
// model that the CLI builds?
//
// kernel-parity.test.ts already settles that the WASM build computes what the
// native build computes, over the whole CI-safe acceptance suite. So a failure
// here can only mean the model → scene translation drifted — which is where the
// defects have actually been. buildBuildings dropped every MultiPolygon
// building and PolygonCentroid ignored polygon holes; both survived a full
// name-level parity suite, and both are encoded in the fixtures below. So did a
// third: browser mode handed lon/lat straight to the kernel, which measures
// distance with math.Hypot, so every propagation distance in a geographic model
// fell under the minimum-distance clamp — tens of dB, with nothing to see. That
// one is `road_geographic`, and the x/y comparison below is what makes it
// visible.
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

import { browserBackend, resetBrowserBackendForTests } from "./browser-backend";
import { clearPersistedState } from "./browser-storage";
import { useModelStore } from "@/model/model-store";
import { normalizeModelGeoJSON } from "@/model/normalize";
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

/**
 * `parityFixtures` in parity_golden_test.go, project CRS and all.
 *
 * The CRS is not decoration. Two of these fixtures are authored in EPSG:25832
 * and one in EPSG:4326, and a browser that assumed 4326 for all three would not
 * refuse the metric ones: `road_building_barrier` holds coordinates like
 * `[-60, 0]` and `[60, 12]`, which are perfectly good lon/lat, so it would
 * project them into UTM zone 31 and compute a scene 10,000 km wide.
 */
const FIXTURES = [
  { name: "road_building_barrier", crs: "EPSG:25832" },
  { name: "parking_building", crs: "EPSG:25832" },
  { name: "road_geographic", crs: "EPSG:4326" },
] as const;

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
    x: number;
    y: number;
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
  beforeEach(async () => {
    await clearPersistedState();
    resetBrowserBackendForTests();
    useModelStore.setState({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:4326",
    });
  });

  it("loads the kernel through the mocked production entry point", async () => {
    const { getKernel } = await import("@/wasm/kernel");
    await expect(getKernel()).resolves.toBe(await getNodeKernel());
  });

  it.each(FIXTURES)("$name", async ({ name, crs }) => {
    const collection = JSON.parse(
      parityFile(`${name}.geojson`),
    ) as GeoJSONFeatureCollection;
    const expected = JSON.parse(parityFile(`${name}.golden.json`)) as {
      receivers: ReceiverSnapshot[];
    };

    const { features, skipped } = normalizeModelGeoJSON(collection);

    // A silently skipped source or building would make the browser compute a
    // smaller scene and the comparison would then be measuring the wrong thing.
    // Receivers are read rather than skipped now, so nothing is exempt.
    expect(skipped).toEqual([]);

    useModelStore.setState({
      features,
      receivers: receiversFrom(collection),
      calcArea: null,
      crs,
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

      // The coordinates, not only the levels.
      //
      // They were not compared before, and that is the second reason a whole
      // class of defect was invisible here: a browser computing in degrees
      // wrote degrees into its receiver table, the CLI wrote metres into its
      // golden, and nothing looked. On a geographic fixture the difference is
      // six orders of magnitude.
      //
      // 1e-6 is the goldens' own rounding, not a tolerance: both targets run
      // the same transform over the same coordinates through the same kernel,
      // so an agreement looser than that would mean they had taken different
      // routes there.
      expect(
        Math.abs(round6(record?.x ?? NaN) - want.x),
        `receiver ${want.id} x`,
      ).toBeLessThanOrEqual(1e-6);
      expect(
        Math.abs(round6(record?.y ?? NaN) - want.y),
        `receiver ${want.id} y`,
      ).toBeLessThanOrEqual(1e-6);

      expect(
        Math.abs(round6(record?.values["LrDay"] ?? NaN) - want.lr_day),
      ).toBeLessThanOrEqual(1e-6);
      expect(
        Math.abs(round6(record?.values["LrNight"] ?? NaN) - want.lr_night),
      ).toBeLessThanOrEqual(1e-6);
    }
  });

  // The geographic fixture is the only one that can see a target computing in
  // degrees, so what it proves has to be stated rather than left implicit in a
  // level comparison: the run really was projected, and the run says so.
  it("projects a geographic model into the zone the CLI would pick", async () => {
    const collection = JSON.parse(
      parityFile("road_geographic.geojson"),
    ) as GeoJSONFeatureCollection;

    useModelStore.setState({
      features: normalizeModelGeoJSON(collection).features,
      receivers: receiversFrom(collection),
      calcArea: null,
      crs: "EPSG:4326",
    });

    const run = await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: RUN_PARAMS,
      receiverMode: "custom",
    });

    const summaryRef = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.summary",
    );
    await expect(
      browserBackend.getArtifactContent(summaryRef?.id ?? ""),
    ).resolves.toMatchObject({
      project_crs: "EPSG:4326",
      compute_crs: "EPSG:25832",
    });

    const tableRef = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.receiver_table_json",
    );
    const table = await browserBackend.getArtifactContent<ReceiverTable>(
      tableRef?.id ?? "",
    );

    // Eastings and northings, not longitudes and latitudes — the same check
    // TestGeographicFixtureIsActuallyProjected makes on the Go side.
    for (const record of table.records) {
      expect(record.x).toBeGreaterThan(400_000);
      expect(record.x).toBeLessThan(700_000);
      expect(record.y).toBeGreaterThan(5_000_000);
      expect(record.y).toBeLessThan(6_500_000);
    }
  });

  // A model already in metres must run through exactly the code it ran through
  // before: projecting one would move every coordinate for nothing.
  it("leaves a model already in a metric CRS where it is", async () => {
    const collection = JSON.parse(
      parityFile("road_building_barrier.geojson"),
    ) as GeoJSONFeatureCollection;

    useModelStore.setState({
      features: normalizeModelGeoJSON(collection).features,
      receivers: receiversFrom(collection),
      calcArea: null,
      crs: "EPSG:25832",
    });

    const run = await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: RUN_PARAMS,
      receiverMode: "custom",
    });

    const summaryRef = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.summary",
    );
    await expect(
      browserBackend.getArtifactContent(summaryRef?.id ?? ""),
    ).resolves.toMatchObject({
      project_crs: "EPSG:25832",
      compute_crs: "EPSG:25832",
    });
  });

  // Browser mode refuses a site the CLI refuses, in the CLI's own words. The
  // refusal is `geo.ComputeCRSForGeographic`'s, carried across verbatim.
  it("refuses a geographic model outside the supported UTM zones", async () => {
    useModelStore.setState({
      features: [
        {
          id: "road",
          kind: "source",
          sourceType: "line",
          properties: {},
          geometry: {
            type: "LineString",
            // Manhattan: UTM zone 18, which the ETRS89 table does not carry.
            coordinates: [
              [-74.0, 40.7],
              [-73.99, 40.7],
            ],
          },
        },
      ],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [-73.995, 40.701] },
        },
      ],
      calcArea: null,
      crs: "EPSG:4326",
    });

    await expect(
      browserBackend.startRun({
        standardId: "rls19-road",
        version: "2019",
        profile: "default",
        params: RUN_PARAMS,
        receiverMode: "custom",
      }),
    ).rejects.toThrow(/zones 31-34/);
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
      features: normalizeModelGeoJSON(collection).features,
      receivers: receiversFrom(collection),
      calcArea: null,
      crs: "EPSG:25832",
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
