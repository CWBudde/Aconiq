import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  browserBackend,
  buildBuildings,
  buildingFootprints,
  buildRoadSources,
  MAX_STORED_RUNS,
  overpassWayToFeature,
  PERSISTED_STATE_VERSION,
  resetBrowserBackendForTests,
} from "./browser-backend";
import * as storage from "./browser-storage";
import { getFeatureBBox } from "@/model/grid-estimate";
import { useModelStore } from "@/model/model-store";
import { normalizeModelGeoJSON } from "@/model/normalize";
import { buildReceiverTableCSV } from "@/model/receiver-csv";
import type { GeoJSONFeatureCollection, ModelFeature } from "@/model/types";
import type { ComputeRequest, TransformRequest } from "@/wasm/types";
import type {
  RasterMetadata,
  ReceiverTable,
  RunSummary,
  StandardDescriptor,
} from "./client";

/**
 * Records every `transform` request `startRun` makes, so a test can ask what
 * CRS the browser asked the kernel to project from.
 */
const transformRequests: TransformRequest[] = [];

/**
 * A descriptor the stubbed kernel can publish. Only `id` carries weight here —
 * `startRun`'s first gate matches on it — but the shape is the real one, so a
 * field this file invents cannot pass for a field the Go encoding emits.
 */
function descriptorFor(id: string): StandardDescriptor {
  return {
    id,
    description: `${id} (stub)`,
    default_version: "2019",
    versions: [],
    // Not optional in practice: `framework.StandardDescriptor.Validate()`
    // refuses a module that declares no tier, so every descriptor a real
    // kernel publishes carries one, and a stub without it would let a
    // regression in the summary's tier pass unnoticed.
    evidence_tier: "normative",
  };
}

/**
 * What `kernel.standards()` answers with, and therefore what `startRun`'s
 * first gate consults.
 *
 * Mutable, and reset per test that touches it, because the gate asks the kernel
 * instead of comparing against a string literal: publishing a standard is the
 * one thing a real kernel changes when it grows a module, so a test about the
 * gates has to be able to change it too.
 */
const RLS19_ROAD_DESCRIPTOR = descriptorFor("rls19-road");
let publishedStandards: StandardDescriptor[] = [RLS19_ROAD_DESCRIPTOR];

/** What the stubbed kernel answers `rls19Road` with, per receiver. */
type Levels = { lr_day: number; lr_night: number };

/**
 * A flat 50/40 for every receiver. Most of this file drives `startRun` to see
 * what it *stores*, and cares only that a number arrived, not which.
 *
 * Mutable for the same reason `publishedStandards` is: the golden test below
 * has an opinion about the levels, because its whole point is to follow known
 * values through the assembly the parity suites step over.
 */
const FLAT_LEVELS = (): Levels => ({ lr_day: 50, lr_night: 40 });
let kernelLevels: (receiverID: string) => Levels = FLAT_LEVELS;

// The persistence tests drive `startRun` end to end, but what the kernel
// computes is the parity suite's business; here it only has to answer.
//
// `transform` is a deliberate no-op: it reports the coordinates unmoved, in the
// CRS they arrived in. These fixtures state their coordinates in the units they
// mean, and a stub that projected them would silently change every extent and
// grid this file asserts on. Whether the *real* projection lands the model
// where the CLI lands it is the `road_geographic` parity fixture's job, against
// the real kernel; what this file can still see is that `startRun` asks for the
// projection at all, and asks with the store's CRS — which is what
// `transformRequests` is for.
vi.mock("@/wasm/kernel", () => ({
  // `runRLS19Road` wires an abort signal to this. Nothing in this file
  // cancels — the suites below drive `startRun` to completion — but a mock
  // factory replaces the whole module, so an export it omits is one the
  // importing module cannot even reference.
  cancelKernel: () => undefined,
  getKernel: () =>
    Promise.resolve({
      rls19Road: (req: ComputeRequest) =>
        Promise.resolve(
          req.receivers.map((receiver) => ({
            Receiver: receiver,
            Indicators: kernelLevels(receiver.id),
          })),
        ),
      transform: (req: TransformRequest) => {
        transformRequests.push(req);
        return Promise.resolve({
          source_crs: req.source_crs,
          target_crs: req.source_crs,
          applied: false,
          coordinates: req.coordinates,
        });
      },
      standards: () => Promise.resolve(publishedStandards),
      defaultConfig: () => Promise.resolve({}),
    }),
}));

describe("browserBackend.getModel", () => {
  it("rejects rather than answering null", () => {
    // A `null` here would be indistinguishable from "the project has no model
    // yet" to a caller that forgot the capability gate, and would leave the
    // map empty over a populated store. Following `httpBackend.createExport`:
    // the method exists so the interface has no mode-specific hole, and says
    // why it cannot answer.
    // No CRS argument: the implementation takes none, because there is
    // nothing to reproject and naming one would suggest otherwise.
    return expect(browserBackend.getModel()).rejects.toThrow(
      /not available in browser mode/,
    );
  });
});

describe("browserBackend.saveModel", () => {
  it("returns no hash: there is no file to be a receipt for", async () => {
    const result = await browserBackend.saveModel({
      crs: "EPSG:4326",
      model: { type: "FeatureCollection", features: [] },
    });

    expect(result.hash).toBeNull();
  });
});

describe("buildRoadSources", () => {
  it("prefers feature-level RLS-19 overrides over run defaults", () => {
    const features: ModelFeature[] = [
      {
        id: "road-a",
        kind: "source",
        sourceType: "line",
        properties: {
          // AB is tabulated in both Tabelle 4a speed bands. Beton at 60 km/h,
          // which this fixture used to carry, is a crossed-out cell the kernel
          // now refuses — the pairing has to be valid for the assertion about
          // override precedence to be about precedence.
          surface_type: "AB",
          road_speed_kph: 60,
          speed_lkw2_kph: 50,
          gradient_percent: 4,
          traffic_day_pkw: 1200,
          traffic_night_pkw: 200,
        },
        geometry: {
          type: "LineString",
          coordinates: [
            [0, 0],
            [10, 0],
          ],
        },
      },
      {
        id: "road-b",
        kind: "source",
        sourceType: "line",
        properties: {
          traffic_day_pkw: 300,
        },
        geometry: {
          type: "LineString",
          coordinates: [
            [0, 10],
            [10, 10],
          ],
        },
      },
    ];

    const sources = buildRoadSources(features, {
      surface_type: "SMA",
      speed_pkw_kph: "100",
      speed_lkw1_kph: "100",
      speed_lkw2_kph: "80",
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
    });

    expect(sources).toHaveLength(2);
    expect(sources[0]?.surface_type).toBe("AB");
    expect(sources[0]?.speeds.pkw_kph).toBe(60);
    expect(sources[0]?.speeds.lkw2_kph).toBe(50);
    expect(sources[0]?.traffic_day.pkw_per_hour).toBe(1200);
    expect(sources[1]?.surface_type).toBe("SMA");
    expect(sources[1]?.speeds.pkw_kph).toBe(100);
    expect(sources[1]?.traffic_day.pkw_per_hour).toBe(300);
  });
});

describe("overpassWayToFeature", () => {
  it("maps highway tags to source acoustics and marks review-needed imports", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 42,
      tags: {
        highway: "primary",
        maxspeed: "50",
        surface: "asphalt",
      },
      geometry: [
        { lon: 7, lat: 50 },
        { lon: 7.1, lat: 50.1 },
      ],
    });

    expect(feature?.properties["kind"]).toBe("source");
    expect(feature?.properties["road_speed_kph"]).toBe(50);
    expect(feature?.properties["surface_type"]).toBe("SMA");
    expect(feature?.properties["road_speed_kph_inferred"]).toBe(true);
  });

  /*
   * Browser mode must answer the height question the way the Go importer does.
   *
   * It did not: it read only the `height` tag and fell back to a bare 9 m, so
   * `building:levels` was ignored even where the way carried it, and nothing
   * recorded that the number was assumed. A building's height reaches the
   * line-of-sight test, so that was a computed-level difference between the two
   * backends for one input.
   */
  const ring = [
    { lon: 7, lat: 50 },
    { lon: 7.001, lat: 50 },
    { lon: 7.001, lat: 50.001 },
    { lon: 7, lat: 50 },
  ];

  it("reads building:levels when the way carries no height tag", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 43,
      tags: { building: "yes", "building:levels": "4" },
      geometry: ring,
    });

    expect(feature?.properties["height_m"]).toBe(12);
    expect(feature?.properties["height_source"]).toBeUndefined();
  });

  it("marks an assumed building height rather than passing it off as read", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 44,
      tags: { building: "yes" },
      geometry: ring,
    });

    expect(feature?.properties["height_m"]).toBe(9);
    expect(feature?.properties["height_source"]).toBe("assumed");
  });

  it("marks an assumed barrier height too", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 45,
      tags: { barrier: "wall" },
      geometry: [
        { lon: 7, lat: 50 },
        { lon: 7.1, lat: 50.1 },
      ],
    });

    expect(feature?.properties["height_m"]).toBe(2);
    expect(feature?.properties["height_source"]).toBe("assumed");
  });

  it("leaves a read height unmarked", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 46,
      tags: { building: "yes", height: "7.5 m" },
      geometry: ring,
    });

    expect(feature?.properties["height_m"]).toBe(7.5);
    expect(feature?.properties["height_source"]).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// Object URL lifetime
// ---------------------------------------------------------------------------

const LEGACY_STORAGE_KEY = "aconiq.browser_backend.v1";
const RUN_ID = "run-0001";
const TABLE_ARTIFACT_ID = "artifact-run-0001-receivers-json";
const SECOND_RUN_ID = "run-0002";
const SECOND_ARTIFACT_ID = "artifact-run-0002-summary";

function seededState({ withSecondRun = false } = {}) {
  const run = {
    id: RUN_ID,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    status: "completed",
    started_at: "2026-01-01T10:00:00.000Z",
    finished_at: "2026-01-01T10:00:01.000Z",
    log_path: `.noise/runs/${RUN_ID}/run.log`,
    artifacts: [
      {
        id: TABLE_ARTIFACT_ID,
        kind: "run.result.receiver_table_json",
        path: `.noise/runs/${RUN_ID}/receivers.json`,
        created_at: "2026-01-01T10:00:01.000Z",
      },
    ],
  };
  return {
    projectId: "demo",
    projectName: "Demo",
    projectPath: "/demo",
    crs: "EPSG:25832",
    runs: [
      ...(withSecondRun
        ? [
            {
              run: {
                ...run,
                id: SECOND_RUN_ID,
                started_at: "2026-01-01T09:00:00.000Z",
                artifacts: [
                  {
                    id: SECOND_ARTIFACT_ID,
                    kind: "run.result.summary",
                    path: `.noise/runs/${SECOND_RUN_ID}/summary.json`,
                    created_at: "2026-01-01T09:00:01.000Z",
                  },
                ],
              },
              log: { run_id: SECOND_RUN_ID, lines: ["run completed"] },
              artifacts: {
                [SECOND_ARTIFACT_ID]: {
                  kind: "run.result.summary",
                  mimeType: "application/json",
                  encoding: "json",
                  value: { run_id: SECOND_RUN_ID },
                },
              },
            },
          ]
        : []),
      {
        run,
        log: { run_id: RUN_ID, lines: ["run completed"] },
        artifacts: {
          [TABLE_ARTIFACT_ID]: {
            kind: "run.result.receiver_table_json",
            mimeType: "application/json",
            encoding: "json",
            value: {
              run_id: RUN_ID,
              standard_id: "rls19-road",
              indicator_order: ["lr_day"],
              unit: "dB(A)",
              records: [
                { id: "R1", x: 0, y: 0, height_m: 4, values: { lr_day: 55 } },
              ],
            },
          },
        },
      },
    ],
  };
}

/**
 * Writes the fixture the way a previous session would have left it, then
 * forgets the in-memory copy so the backend loads it from the store — and
 * loads it, because `getArtifactURL` is synchronous and reads only the
 * loaded state.
 */
async function seedState(options?: { withSecondRun?: boolean }) {
  await storage.savePersistedState({
    version: PERSISTED_STATE_VERSION,
    state: seededState(options),
  });
  resetBrowserBackendForTests();
  await browserBackend.getRuns();
}

async function resetStores() {
  await storage.clearPersistedState();
  window.localStorage.clear();
  resetBrowserBackendForTests();
}

describe("artifact object URLs", () => {
  let created: string[] = [];
  let revoked: string[] = [];

  // The blob URL cache is module state that outlives a single test, so the
  // mock counter must not restart — two tests would otherwise mint the same
  // URL string and the identity assertions below would be meaningless.
  let counter = 0;

  beforeEach(async () => {
    await resetStores();
    created = [];
    revoked = [];
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: () => {
        counter += 1;
        const url = `blob:mock/${String(counter)}`;
        created.push(url);
        return url;
      },
      revokeObjectURL: (url: string) => {
        revoked.push(url);
      },
    });
    await seedState();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("caches one URL per artifact", () => {
    const first = browserBackend.getArtifactURL(TABLE_ARTIFACT_ID);
    const second = browserBackend.getArtifactURL(TABLE_ARTIFACT_ID);
    expect(second).toBe(first);
    expect(created).toHaveLength(1);
  });

  it("keeps URLs for artifacts that still exist when state is written", async () => {
    const url = browserBackend.getArtifactURL(TABLE_ARTIFACT_ID);

    // createExport writes state. The old writeState revoked the whole cache,
    // which invalidated URLs the export page was still rendering — the report
    // iframe and the "open in browser" link went dead mid-view.
    await browserBackend.createExport(RUN_ID);

    expect(revoked).not.toContain(url);
    expect(browserBackend.getArtifactURL(TABLE_ARTIFACT_ID)).toBe(url);
  });

  it("revokes URLs whose artifact has disappeared", async () => {
    await seedState({ withSecondRun: true });
    const kept = browserBackend.getArtifactURL(TABLE_ARTIFACT_ID);
    const dropped = browserBackend.getArtifactURL(SECOND_ARTIFACT_ID);
    expect(dropped).not.toBe(kept);

    // Drop the second run, then write state through the public API. Pruning
    // has to release the blobs of artifacts that are gone — otherwise the fix
    // above would just be a leak.
    await seedState();
    await browserBackend.createExport(RUN_ID);

    expect(revoked).toContain(dropped);
    expect(revoked).not.toContain(kept);
  });
});

// ---------------------------------------------------------------------------
// Persistence: version guard, migration, cap, quota
// ---------------------------------------------------------------------------

const ROAD: ModelFeature = {
  id: "road",
  kind: "source",
  sourceType: "line",
  properties: { traffic_day_pkw: 300 },
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [100, 0],
    ],
  },
};

const RUN_SPEC = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: { surface_type: "SMA" },
  receiverMode: "custom",
} as const;

function runFixture(index: number, startedAt: string) {
  const id = `run-${String(index).padStart(4, "0")}`;
  return {
    run: {
      id,
      scenario_id: "default",
      standard_id: "rls19-road",
      version: "2019",
      status: "completed",
      started_at: startedAt,
      finished_at: startedAt,
      log_path: `${id}/run.log`,
      artifacts: [],
    },
    log: { run_id: id, lines: [] },
    artifacts: {},
  };
}

/**
 * A stored run whose raster bytes really are in the byte store, which is what
 * makes "were they deleted?" a question with an answer.
 */
async function runFixtureWithRaster(index: number, startedAt: string) {
  const fixture = runFixture(index, startedAt);
  const artifactId = `artifact-${fixture.run.id}-raster-bin`;
  await storage.saveArtifactBytes(artifactId, new ArrayBuffer(64));
  return {
    ...fixture,
    run: {
      ...fixture.run,
      artifacts: [
        {
          id: artifactId,
          kind: "run.result.raster_binary",
          path: `${fixture.run.id}/results/rls19-road.bin`,
          created_at: startedAt,
        },
      ],
    },
    artifacts: {
      [artifactId]: {
        kind: "run.result.raster_binary",
        mimeType: "application/octet-stream",
        encoding: "binary",
        value: null,
      },
    },
  };
}

async function persisted(): Promise<{ version: number; state: unknown }> {
  return (await storage.loadPersistedState()) as {
    version: number;
    state: unknown;
  };
}

function runIDs(state: unknown): string[] {
  return (state as { runs: { run: { id: string } }[] }).runs.map(
    (entry) => entry.run.id,
  );
}

function highWaterMark(state: unknown): number | undefined {
  return (state as { runHighWaterMark?: number }).runHighWaterMark;
}

/**
 * The two refusals `startRun` can hand back before it computes anything, and
 * the reason they are two.
 *
 * Browser mode used to compare `spec.standardId` against the literal
 * "rls19-road", which was right only because the kernel published exactly one
 * standard. The gate now asks the kernel, and a standard the kernel publishes
 * but this build cannot extract a model for is a separate, differently worded
 * refusal. Without the second case the split is unobservable, because the one
 * published standard is also the one with an extraction.
 */
describe("browserBackend.startRun standard gates", () => {
  afterEach(() => {
    publishedStandards = [RLS19_ROAD_DESCRIPTOR];
  });

  it("refuses a standard the kernel does not publish", async () => {
    publishedStandards = [RLS19_ROAD_DESCRIPTOR];

    await expect(
      browserBackend.startRun({ ...RUN_SPEC, standardId: "iso9613" }),
    ).rejects.toThrow("Standard iso9613 is not available in browser mode");
  });

  it("refuses a published standard it has no model extraction for", async () => {
    publishedStandards = [RLS19_ROAD_DESCRIPTOR, descriptorFor("iso9613")];

    await expect(
      browserBackend.startRun({ ...RUN_SPEC, standardId: "iso9613" }),
    ).rejects.toThrow(
      "The kernel publishes iso9613, but this build has no model extraction for it",
    );
  });

  it("lets the published standard it can extract through both gates", async () => {
    await resetStores();
    useModelStore.setState({
      features: [ROAD],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [50, 20] },
        },
      ],
      calcArea: null,
      crs: "EPSG:4326",
    });

    const run = await browserBackend.startRun(RUN_SPEC);
    expect(run.standard_id).toBe("rls19-road");
  });
});

/**
 * `segment_length_mode` is an enum, and the CLI refuses a value outside it:
 * `PropagationConfig.Validate` names the two modes. Browser mode used to read
 * anything it did not recognise as `fixed`, so a typo computed one thing while
 * the run's own parameter record — which is stamped into provenance — said
 * another. The two targets have to refuse the same request.
 */
describe("browserBackend.startRun segment_length_mode", () => {
  beforeEach(async () => {
    await resetStores();
    useModelStore.setState({
      features: [ROAD],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [50, 20] },
        },
      ],
      calcArea: null,
      crs: "EPSG:4326",
    });
  });

  it("refuses a mode the kernel does not accept", async () => {
    await expect(
      browserBackend.startRun({
        ...RUN_SPEC,
        params: { ...RUN_SPEC.params, segment_length_mode: "adaptive" },
      }),
    ).rejects.toThrow("segment_length_mode must be one of");
  });

  it("accepts the two modes, and an unset one as fixed", async () => {
    for (const mode of ["fixed", "distance_scaled"]) {
      const run = await browserBackend.startRun({
        ...RUN_SPEC,
        params: { ...RUN_SPEC.params, segment_length_mode: mode },
      });

      expect(run.standard_id).toBe("rls19-road");
    }

    const unset = await browserBackend.startRun(RUN_SPEC);
    expect(unset.standard_id).toBe("rls19-road");
  });
});

describe("persisted state", () => {
  let warn: ReturnType<typeof vi.spyOn>;

  beforeEach(async () => {
    await resetStores();
    useModelStore.setState({
      features: [ROAD],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [50, 20] },
        },
      ],
      calcArea: null,
      // Restated per test: the model store is module state, so a case that
      // declares a metric CRS would otherwise leak it into the next one.
      crs: "EPSG:4326",
    });
    transformRequests.length = 0;
    warn = vi.spyOn(console, "warn").mockImplementation(() => undefined);
    // jsdom has no object URLs, and every persist prunes the URL cache.
    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: () => "blob:mock/persisted",
      revokeObjectURL: () => undefined,
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("starts fresh when nothing is stored", async () => {
    await expect(browserBackend.getRuns()).resolves.toEqual([]);
    const status = await browserBackend.getProjectStatus();
    expect(status.project_id).toBe("browser-project");
    expect(warn).not.toHaveBeenCalled();
  });

  it("loads a version 1 document field by field", async () => {
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: {
        // projectId absent, a `crs` no build reads any more, one run entry
        // that is not a run: an older build could have written any of these.
        projectName: "Old",
        crs: 42,
        runs: [
          runFixture(1, "2026-01-01T10:00:00.000Z"),
          { run: "not a run" },
          null,
        ],
      },
    });

    const runs = await browserBackend.getRuns();
    expect(runs.map((run) => run.id)).toEqual(["run-0001"]);
    const status = await browserBackend.getProjectStatus();
    expect(status.project_id).toBe("browser-project");
    expect(status.name).toMatch(/^Old/);
    // The CRS comes from the model store, not from the stored document: in
    // browser mode the store *is* the project, and the document's old `crs`
    // field held the display label "WGS84 / web map", which named no CRS and
    // could not have — the store holds whatever the last import brought.
    expect(status.crs).toBe("EPSG:4326");
  });

  it.each([
    ["no encoding", { mimeType: "application/json", kind: "k", value: {} }],
    [
      "an unknown encoding",
      { mimeType: "application/json", kind: "k", encoding: "blob", value: 1 },
    ],
    ["no value", { mimeType: "text/plain", kind: "k", encoding: "text" }],
    ["no mime type", { kind: "k", encoding: "text", value: "x" }],
    ["a non-object artifact", "just text"],
  ])("drops a run whose artifact has %s", async (_name, artifact) => {
    const broken = runFixture(2, "2026-01-01T11:00:00.000Z");
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: {
        runs: [
          { ...broken, artifacts: { "artifact-run-0002-x": artifact } },
          runFixture(1, "2026-01-01T10:00:00.000Z"),
        ],
      },
    });

    const runs = await browserBackend.getRuns();
    expect(runs.map((run) => run.id)).toEqual(["run-0001"]);
  });

  /*
   * `run.result.raster_binary` used to be stored as `text` whose value was the
   * run's SHA-256 hex string. Those documents are still at version 1 — the
   * version deliberately did not move, because a bump costs the reader its
   * whole run list where this costs one raster that was never really stored.
   * What the missing bump must not cost is a caller asking for an ArrayBuffer
   * and being handed 64 characters of hex that decode as a raster of nothing.
   */
  it("refuses a pre-binary raster artifact instead of serving its digest", async () => {
    const legacy = runFixture(1, "2026-01-01T10:00:00.000Z");
    const artifactID = "artifact-run-0001-raster-bin";
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: {
        runs: [
          {
            ...legacy,
            artifacts: {
              [artifactID]: {
                kind: "run.result.raster_binary",
                mimeType: "application/octet-stream",
                encoding: "text",
                value: "a".repeat(64),
              },
            },
          },
        ],
      },
    });

    // The run itself survives: its receiver table, sidecar and summary are
    // still exactly what it wrote.
    await expect(browserBackend.getRuns()).resolves.toHaveLength(1);
    await expect(browserBackend.getArtifactContent(artifactID)).rejects.toThrow(
      /bytes are not stored/,
    );
  });

  it.each([
    ["an unknown version", { version: 2, state: { runs: [] } }],
    ["a missing version", { state: { runs: [] } }],
    ["a non-object document", "just a string"],
    ["a non-object state", { version: PERSISTED_STATE_VERSION, state: [] }],
  ])(
    "starts fresh on %s and leaves the document alone until the next write",
    async (_name, document) => {
      await storage.savePersistedState(document);

      await expect(browserBackend.getRuns()).resolves.toEqual([]);
      expect(warn).toHaveBeenCalledOnce();
      // Reading must not have replaced what it could not understand.
      await expect(storage.loadPersistedState()).resolves.toEqual(document);

      const run = await browserBackend.startRun(RUN_SPEC);
      expect((await persisted()).version).toBe(PERSISTED_STATE_VERSION);
      expect(runIDs((await persisted()).state)).toEqual([run.id]);
    },
  );

  it("migrates the localStorage document once, then removes it", async () => {
    window.localStorage.setItem(
      LEGACY_STORAGE_KEY,
      JSON.stringify(seededState({ withSecondRun: true })),
    );

    const runs = await browserBackend.getRuns();
    expect(runs.map((run) => run.id)).toEqual([RUN_ID, SECOND_RUN_ID]);
    expect(window.localStorage.getItem(LEGACY_STORAGE_KEY)).toBeNull();
    const document = await persisted();
    expect(document.version).toBe(PERSISTED_STATE_VERSION);
    // Copied as it was, not re-sorted: the list order is `getRuns`' concern.
    expect(runIDs(document.state).sort()).toEqual([RUN_ID, SECOND_RUN_ID]);
    // The artifact came across with the run, not just its summary line.
    await expect(
      browserBackend.getArtifactContent(TABLE_ARTIFACT_ID),
    ).resolves.toMatchObject({ run_id: RUN_ID });
  });

  it("does not migrate once IndexedDB holds a document", async () => {
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: { runs: [runFixture(7, "2026-02-01T10:00:00.000Z")] },
    });
    window.localStorage.setItem(
      LEGACY_STORAGE_KEY,
      JSON.stringify(seededState()),
    );

    const runs = await browserBackend.getRuns();
    expect(runs.map((run) => run.id)).toEqual(["run-0007"]);
    expect(window.localStorage.getItem(LEGACY_STORAGE_KEY)).not.toBeNull();
  });

  it("starts fresh on an unreadable localStorage document", async () => {
    window.localStorage.setItem(LEGACY_STORAGE_KEY, "{not json");

    await expect(browserBackend.getRuns()).resolves.toEqual([]);
    expect(warn).toHaveBeenCalledOnce();
    await expect(storage.loadPersistedState()).resolves.toBeNull();
  });

  // The whole scene has to reach the kernel in metres. `startRun` therefore
  // projects the workspace once, before any builder reads a coordinate, and it
  // has to ask with the CRS the store says its coordinates are in — not with a
  // constant, which is what `DEFAULT_CRS = "WGS84 / web map"` amounted to.
  it("asks the kernel to project the model out of the store's CRS", async () => {
    useModelStore.setState({ crs: "EPSG:25832" });

    await browserBackend.startRun(RUN_SPEC);

    expect(transformRequests).toHaveLength(1);
    const request = transformRequests[0];
    expect(request).toBeDefined();
    expect(request?.source_crs).toBe("EPSG:25832");
    // The kernel decides the zone, not the browser: the same
    // `geo.ComputeCRSForGeographic` the CLI resolves through.
    expect(request?.target_crs).toBe("auto");
    // Flat and interleaved, and it carries the receiver as well as the road:
    // a run whose sources moved and whose receivers did not would compute
    // distances across two coordinate systems.
    expect((request?.coordinates.length ?? 1) % 2).toBe(0);
    expect(request?.coordinates).toContain(50);
  });

  it("records the CRS it computed in, on the summary and in the log", async () => {
    useModelStore.setState({ crs: "EPSG:25832" });

    const run = await browserBackend.startRun(RUN_SPEC);
    const summary = run.artifacts.find(
      (entry) => entry.kind === "run.result.summary",
    );

    await expect(
      browserBackend.getArtifactContent(summary?.id ?? ""),
    ).resolves.toMatchObject({
      // The CLI's own provenance key names, so a consumer reads one spelling.
      project_crs: "EPSG:25832",
      compute_crs: "EPSG:25832",
    });

    const log = await browserBackend.getRunLog(run.id);
    expect(
      log.lines.some((line) => line.includes("compute_crs=EPSG:25832")),
    ).toBe(true);
  });

  it("stamps the run summary with the tier the kernel declares", async () => {
    // AGENTS.md requires the evidence tier to travel with the result, so that
    // "a consumer that never reads the docs still sees it". Browser-mode
    // summaries carried none, which left the map's evidence badge blank in one
    // of the two shipped modes while API mode showed it.
    const run = await browserBackend.startRun(RUN_SPEC);
    const summary = run.artifacts.find(
      (entry) => entry.kind === "run.result.summary",
    );

    await expect(
      browserBackend.getArtifactContent(summary?.id ?? ""),
    ).resolves.toMatchObject({ evidence_tier: "normative" });
  });

  it("refuses a model whose property geometry it cannot project", async () => {
    useModelStore.setState({
      features: [
        {
          ...ROAD,
          properties: {
            ...ROAD.properties,
            // Coordinates in the project CRS that live in a property rather
            // than in `geometry`. `geo/modelgeojson/reproject.go` moves these;
            // browser mode cannot, so it must refuse rather than leave them in
            // degrees inside a model that is otherwise in metres.
            rls19_directional_sources: [],
          },
        },
      ],
    });

    await expect(browserBackend.startRun(RUN_SPEC)).rejects.toThrow(
      /rls19_directional_sources/,
    );
  });

  it("keeps a run's results readable after a reload", async () => {
    const run = await browserBackend.startRun(RUN_SPEC);
    const artifact = run.artifacts.find(
      (entry) => entry.kind === "run.result.summary",
    );

    resetBrowserBackendForTests();

    const runs = await browserBackend.getRuns();
    expect(runs.map((entry) => entry.id)).toEqual([run.id]);
    await expect(
      browserBackend.getArtifactContent(artifact?.id ?? ""),
    ).resolves.toMatchObject({ run_id: run.id, receiver_count: 1 });
  });

  it("stores the receivers CSV the shared builder produces", async () => {
    const run = await browserBackend.startRun(RUN_SPEC);
    const idOf = (kind: string) =>
      run.artifacts.find((entry) => entry.kind === kind)?.id ?? "";

    const table = await browserBackend.getArtifactContent<
      Parameters<typeof buildReceiverTableCSV>[0]
    >(idOf("run.result.receiver_table_json"));
    const csv = await browserBackend.getArtifactContent<string>(
      idOf("run.result.receiver_table_csv"),
    );

    // Not a restatement of the bytes — those are the builder's contract, pinned
    // against the CLI in model/receiver-csv.parity.test.ts. What is asserted
    // here is that the stored artifact went through that builder at all, so a
    // second inline copy of the escaping cannot creep back in.
    expect(csv).toBe(buildReceiverTableCSV(table));
    expect(csv).toBe("id,x,y,height_m,LrDay,LrNight\nR1,50,20,4,50,40\n");
  });

  it("refuses artifact URLs before the state is loaded", () => {
    expect(() => browserBackend.getArtifactURL("artifact-x")).toThrow(
      /before the browser backend loaded/,
    );
  });

  it("keeps at most MAX_STORED_RUNS runs, dropping the oldest", async () => {
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: {
        runs: Array.from({ length: MAX_STORED_RUNS }, (_, index) =>
          runFixture(
            index + 1,
            `2026-01-01T${String(index).padStart(2, "0")}:00:00.000Z`,
          ),
        ),
      },
    });

    const run = await browserBackend.startRun(RUN_SPEC);

    const runs = await browserBackend.getRuns();
    expect(runs).toHaveLength(MAX_STORED_RUNS);
    expect(runs[0]?.id).toBe(run.id);
    expect(runs.map((entry) => entry.id)).not.toContain("run-0001");
    expect(runIDs((await persisted()).state)).toHaveLength(MAX_STORED_RUNS);
  });

  it("mints run ids past the highest stored id, not from the list length", async () => {
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: {
        // A list that has already been capped once: run-0001 is gone.
        runs: [
          runFixture(3, "2026-01-01T03:00:00.000Z"),
          runFixture(2, "2026-01-01T02:00:00.000Z"),
        ],
      },
    });

    const run = await browserBackend.startRun(RUN_SPEC);
    expect(run.id).toBe("run-0004");
  });

  // The store is shared by every tab of the origin. "Another tab" is a
  // direct write to the store after this tab has loaded its cache.
  it("a run completed in another tab survives this tab's next write", async () => {
    await browserBackend.getRuns();
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: { runs: [runFixture(1, "2026-01-01T01:00:00.000Z")] },
    });

    const run = await browserBackend.startRun(RUN_SPEC);

    expect(run.id).not.toBe("run-0001");
    expect(runIDs((await persisted()).state)).toEqual([run.id, "run-0001"]);
    const runs = await browserBackend.getRuns();
    expect(runs.map((entry) => entry.id)).toEqual([run.id, "run-0001"]);
  });

  it("a run id is allocated after the other tab's run is seen", async () => {
    await browserBackend.getRuns();
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: { runs: [runFixture(3, "2026-01-01T03:00:00.000Z")] },
    });

    const run = await browserBackend.startRun(RUN_SPEC);
    expect(run.id).toBe("run-0004");
  });

  it("writes are serialised through navigator.locks when available", async () => {
    // jsdom has no Web Locks; the stub grants every request at once and
    // only records what was asked for.
    const request = vi.fn((_name: string, callback: () => Promise<unknown>) =>
      callback(),
    );
    Object.defineProperty(navigator, "locks", {
      value: { request },
      configurable: true,
    });
    try {
      await seedState();

      const run = await browserBackend.createExport(RUN_ID);

      expect(request).toHaveBeenCalledWith(
        "aconiq-browser-backend",
        expect.any(Function),
      );
      expect(
        run.artifacts.some((entry) => entry.kind === "export.bundle"),
      ).toBe(true);
      expect(runIDs((await persisted()).state)).toEqual([RUN_ID]);
    } finally {
      Reflect.deleteProperty(navigator, "locks");
    }
  });

  describe("run ids", () => {
    it("derives the mark from a document written before it existed", async () => {
      // The field is additive, so an older document simply lacks it — and the
      // highest stored id is exactly right there, because nothing could delete
      // a run before this mark existed.
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            runFixture(7, "2026-01-01T07:00:00.000Z"),
            runFixture(3, "2026-01-01T03:00:00.000Z"),
          ],
        },
      });
      resetBrowserBackendForTests();

      const run = await browserBackend.startRun(RUN_SPEC);

      expect(run.id).toBe("run-0008");
      expect(highWaterMark((await persisted()).state)).toBe(8);
    });

    it("keeps the runs a document written before the mark holds", async () => {
      // The alternative — bumping PERSISTED_STATE_VERSION — makes an older
      // build read this document as corrupt and discard every run in it.
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: { runs: [runFixture(2, "2026-01-01T02:00:00.000Z")] },
      });
      resetBrowserBackendForTests();

      await expect(
        browserBackend.getRuns().then((runs) => runs.map((r) => r.id)),
      ).resolves.toEqual(["run-0002"]);
    });

    it("does not reuse the id of a run the cap evicted", async () => {
      const state = {
        runs: Array.from({ length: MAX_STORED_RUNS }, (_, i) =>
          runFixture(
            i + 1,
            `2026-01-0${String((i % 9) + 1)}T0${String(i % 10)}:00:00.000Z`,
          ),
        ),
        runHighWaterMark: MAX_STORED_RUNS,
      };
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state,
      });
      resetBrowserBackendForTests();

      const first = await browserBackend.startRun(RUN_SPEC);
      const second = await browserBackend.startRun(RUN_SPEC);

      expect(first.id).toBe(
        `run-${String(MAX_STORED_RUNS + 1).padStart(4, "0")}`,
      );
      expect(second.id).toBe(
        `run-${String(MAX_STORED_RUNS + 2).padStart(4, "0")}`,
      );
      expect(first.id).not.toBe(second.id);
    });

    it("does not lower the mark when a run fails to reach storage", async () => {
      // The mark is raised in `setRun`, which is the moment an id becomes
      // real — never at mint time, where a quota failure would strand it.
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: { runs: [], runHighWaterMark: 41 },
      });
      resetBrowserBackendForTests();

      const run = await browserBackend.startRun(RUN_SPEC);

      expect(run.id).toBe("run-0042");
      expect(highWaterMark((await persisted()).state)).toBe(42);
    });
  });

  describe("deleteRun", () => {
    it("removes the run and everything stored under it", async () => {
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            runFixture(2, "2026-01-01T02:00:00.000Z"),
            runFixture(1, "2026-01-01T01:00:00.000Z"),
          ],
          runHighWaterMark: 2,
        },
      });
      resetBrowserBackendForTests();

      const result = await browserBackend.deleteRun("run-0002");

      expect(result).toEqual({ runId: "run-0002", retainedPaths: [] });
      expect(runIDs((await persisted()).state)).toEqual(["run-0001"]);
    });

    /*
     * "Everything stored under it" has to include the bytes beside the
     * document. Deleting only the document entry reclaims almost nothing, and
     * run-then-delete, repeated, fills the origin's quota with rasters that no
     * run names and only `clearPersistedState` would ever sweep.
     */
    it("removes the raster bytes the document stopped naming", async () => {
      const stored = await runFixtureWithRaster(2, "2026-01-01T02:00:00.000Z");
      const bytesID = stored.run.artifacts[0]?.id ?? "";
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: { runs: [stored], runHighWaterMark: 2 },
      });
      resetBrowserBackendForTests();

      await browserBackend.deleteRun("run-0002");

      await vi.waitFor(async () => {
        expect(await storage.loadArtifactBytes(bytesID)).toBeNull();
      });
    });

    it("does not free the id of the run it deleted", async () => {
      // The hazard the high-water mark exists for: without it the next run
      // would be `run-0002` again, `setRun` would replace a run that is
      // already gone, and artifact ids would name two payloads.
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [runFixture(2, "2026-01-01T02:00:00.000Z")],
          runHighWaterMark: 2,
        },
      });
      resetBrowserBackendForTests();

      await browserBackend.deleteRun("run-0002");
      const next = await browserBackend.startRun(RUN_SPEC);

      expect(next.id).toBe("run-0003");
    });

    it("refuses a run that has not finished", async () => {
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            {
              ...runFixture(1, "2026-01-01T01:00:00.000Z"),
              run: {
                ...runFixture(1, "2026-01-01T01:00:00.000Z").run,
                status: "running",
              },
            },
          ],
        },
      });
      resetBrowserBackendForTests();

      await expect(browserBackend.deleteRun("run-0001")).rejects.toThrow(
        /still running/,
      );
      expect(runIDs((await persisted()).state)).toEqual(["run-0001"]);
    });

    it("rejects an id it does not hold", async () => {
      await expect(browserBackend.deleteRun("run-9999")).rejects.toThrow(
        /not found/,
      );
    });

    it("keeps no export bundle, and says so", async () => {
      // Browser mode stores export artifacts inside the run record, so there
      // is nothing for `retainedPaths` to name — which is why the capability
      // flag exists and the confirmation reads differently here.
      await seedState();
      await browserBackend.createExport(RUN_ID);

      const result = await browserBackend.deleteRun(RUN_ID);

      expect(result.retainedPaths).toEqual([]);
      expect(runIDs((await persisted()).state)).toEqual([]);
    });
  });

  describe("when the store is full", () => {
    const quota = () =>
      new storage.BrowserStorageError("quota", "quota exhausted");

    it("evicts the oldest run and retries once", async () => {
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            runFixture(2, "2026-01-01T02:00:00.000Z"),
            runFixture(1, "2026-01-01T01:00:00.000Z"),
          ],
        },
      });
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "savePersistedStateForgetting")
        .mockRejectedValueOnce(quota());

      const run = await browserBackend.startRun(RUN_SPEC);

      expect(save).toHaveBeenCalledTimes(2);
      const runs = await browserBackend.getRuns();
      expect(runs.map((entry) => entry.id)).toEqual([run.id, "run-0002"]);
      expect(runIDs((await persisted()).state)).toEqual([run.id, "run-0002"]);
    });

    it("keeps the run in memory and says so when the retry fails too", async () => {
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            runFixture(2, "2026-01-01T02:00:00.000Z"),
            runFixture(1, "2026-01-01T01:00:00.000Z"),
          ],
        },
      });
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "savePersistedStateForgetting")
        .mockRejectedValue(quota());

      const failure = await browserBackend
        .startRun(RUN_SPEC)
        .catch((error: unknown) => error);

      expect(save).toHaveBeenCalledTimes(2);
      expect(storage.isBrowserStorageError(failure, "quota")).toBe(true);
      expect((failure as Error).message).toMatch(
        /completed but could not be stored/,
      );
      expect((failure as Error).message).toMatch(/older runs/);

      // The computation succeeded, so its results stay viewable this
      // session, and the failed eviction is not shown either.
      const runs = await browserBackend.getRuns();
      expect(runs.map((entry) => entry.id)).toEqual([
        "run-0003",
        "run-0002",
        "run-0001",
      ]);
      const summary = runs[0]?.artifacts.find(
        (entry) => entry.kind === "run.result.summary",
      );
      await expect(
        browserBackend.getArtifactContent(summary?.id ?? ""),
      ).resolves.toMatchObject({ run_id: "run-0003" });
      // The store still holds what it held before.
      expect(runIDs((await persisted()).state)).toEqual([
        "run-0002",
        "run-0001",
      ]);
    });

    it("reports an export the same way", async () => {
      await seedState();
      vi.spyOn(storage, "savePersistedStateForgetting").mockRejectedValue(
        quota(),
      );

      const failure = await browserBackend
        .createExport(RUN_ID)
        .catch((error: unknown) => error);

      expect(storage.isBrowserStorageError(failure, "quota")).toBe(true);
      expect((failure as Error).message).toMatch(/export completed/);
      const runs = await browserBackend.getRuns();
      expect(
        runs[0]?.artifacts.some((entry) => entry.kind === "export.bundle"),
      ).toBe(true);
    });

    it("restores the un-evicted list when the retry fails for another reason", async () => {
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [
            runFixture(2, "2026-01-01T02:00:00.000Z"),
            runFixture(1, "2026-01-01T01:00:00.000Z"),
          ],
        },
      });
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "savePersistedStateForgetting")
        .mockRejectedValueOnce(quota())
        .mockRejectedValueOnce(
          new storage.BrowserStorageError("unavailable", "gone away"),
        );

      const failure = await browserBackend
        .startRun(RUN_SPEC)
        .catch((error: unknown) => error);

      expect(save).toHaveBeenCalledTimes(2);
      expect(storage.isBrowserStorageError(failure, "unavailable")).toBe(true);
      // The eviction never reached the store, so memory must not show it.
      const runs = await browserBackend.getRuns();
      expect(runs.map((entry) => entry.id)).toEqual([
        "run-0003",
        "run-0002",
        "run-0001",
      ]);
      expect(runIDs((await persisted()).state)).toEqual([
        "run-0002",
        "run-0001",
      ]);
    });

    /*
     * The byte records are keyed outside the document, so deleting them is
     * only safe once the document that stopped naming them is on disk. This
     * is the path that used to lose data: the eviction deleted the victim's
     * raster up front, the retry then failed, `persistRun` put the un-evicted
     * list back — and the run it had just restored could no longer read its
     * own raster.
     */
    it("keeps an evicted run's raster bytes when the retry fails too", async () => {
      const victim = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
      const victimBytesID = victim.run.artifacts[0]?.id ?? "";
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [runFixture(2, "2026-01-01T02:00:00.000Z"), victim],
        },
      });
      await browserBackend.getRuns();
      vi.spyOn(storage, "savePersistedStateForgetting").mockRejectedValue(
        quota(),
      );

      await browserBackend.startRun(RUN_SPEC).catch(() => undefined);

      // The store still holds the run, so it must still hold its raster.
      expect(runIDs((await persisted()).state)).toContain("run-0001");
      expect(await storage.loadArtifactBytes(victimBytesID)).not.toBeNull();
    });

    it("deletes the evicted run's raster bytes once the eviction is stored", async () => {
      const victim = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
      const victimBytesID = victim.run.artifacts[0]?.id ?? "";
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [runFixture(2, "2026-01-01T02:00:00.000Z"), victim],
        },
      });
      await browserBackend.getRuns();
      vi.spyOn(storage, "savePersistedStateForgetting").mockRejectedValueOnce(
        quota(),
      );

      await browserBackend.startRun(RUN_SPEC);

      expect(runIDs((await persisted()).state)).not.toContain("run-0001");
      // Eviction exists to free quota, and the raster is the largest thing a
      // run owns. The delete is deliberately not awaited by the writer.
      await vi.waitFor(async () => {
        expect(await storage.loadArtifactBytes(victimBytesID)).toBeNull();
      });
    });

    it("surfaces a non-quota storage failure without evicting", async () => {
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "savePersistedStateForgetting")
        .mockRejectedValue(
          new storage.BrowserStorageError("unavailable", "no IndexedDB"),
        );

      const failure = await browserBackend
        .startRun(RUN_SPEC)
        .catch((error: unknown) => error);

      expect(save).toHaveBeenCalledOnce();
      expect(storage.isBrowserStorageError(failure, "unavailable")).toBe(true);
      expect((failure as Error).message).toMatch(/could not be stored/);
      expect(await browserBackend.getRuns()).toHaveLength(1);
    });

    /*
     * The raster bytes are written before the document, so the retry has to
     * run the other way round: evict first — a *document* write, which frees
     * the victim's bytes in the same transaction — then write the bytes into
     * the space that freed, then the document naming them. In between, the
     * bytes are stored under a document that does not yet name them; that is
     * the orphan the session sweep reclaims, which is why the two ship
     * together.
     */
    it("evicts the oldest run and retries the raster write", async () => {
      const victim = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
      const victimBytesID = victim.run.artifacts[0]?.id ?? "";
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [runFixture(2, "2026-01-01T02:00:00.000Z"), victim],
        },
      });
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "saveArtifactBytes")
        .mockRejectedValueOnce(quota());

      const run = await browserBackend.startRun(RUN_SPEC);

      expect(save).toHaveBeenCalledTimes(2);
      // The eviction is a document write, so it is visible in the store, and
      // the victim's bytes went with it in the same transaction.
      expect(runIDs((await persisted()).state)).toEqual([run.id, "run-0002"]);
      expect(await storage.loadArtifactBytes(victimBytesID)).toBeNull();
      // The whole point of the retry: the raster the run just computed is
      // readable, where it used to be a reported failure.
      const binary = run.artifacts.find(
        (entry) => entry.kind === "run.result.raster_binary",
      );
      await expect(
        browserBackend.getArtifactBytes(binary?.id ?? ""),
      ).resolves.toBeDefined();
    });

    /*
     * Eviction needs a victim. A first run in an empty store that cannot write
     * its raster has nothing to drop, so there is no second attempt to make —
     * and the failure must still read like every other storage failure,
     * because the dialogs render `error.message` and a raw DOMException text
     * is not what they are written for.
     */
    it("reports a quota failure on the raster write when there is nothing to evict", async () => {
      const save = vi
        .spyOn(storage, "saveArtifactBytes")
        .mockRejectedValue(quota());

      const failure = await browserBackend
        .startRun({
          ...RUN_SPEC,
          receiverMode: "auto-grid",
          params: {
            ...RUN_SPEC.params,
            grid_resolution_m: "50",
            grid_padding_m: "0",
          },
        })
        .catch((error: unknown) => error);

      expect(save).toHaveBeenCalledOnce();
      expect(storage.isBrowserStorageError(failure, "quota")).toBe(true);
      expect((failure as Error).message).toMatch(
        /run completed but could not be stored/,
      );
    });

    it("reports the failure when the raster write fails again after evicting", async () => {
      const victim = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: {
          runs: [runFixture(2, "2026-01-01T02:00:00.000Z"), victim],
        },
      });
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "saveArtifactBytes")
        .mockRejectedValue(quota());

      const failure = await browserBackend
        .startRun(RUN_SPEC)
        .catch((error: unknown) => error);

      expect(save).toHaveBeenCalledTimes(2);
      expect(storage.isBrowserStorageError(failure, "quota")).toBe(true);
      expect((failure as Error).message).toMatch(
        /run completed but could not be stored/,
      );
      // The eviction committed before the retry, and a committed document
      // write cannot be taken back once the victim's bytes are gone with it.
      // The run list the user sees has to agree with the store.
      expect(runIDs((await persisted()).state)).toEqual(["run-0002"]);
      const runs = await browserBackend.getRuns();
      expect(runs.map((entry) => entry.id)).not.toContain("run-0001");
    });

    /*
     * Bytes written for a run whose document never landed are unreachable:
     * `forgetArtifactBytes` walks the runs the document holds, and the next
     * `reloadState` drops the failed run from memory too. Left behind they
     * cost a raster's worth of quota per failed run, permanently — and on the
     * quota path that is the resource that just ran out.
     */
    it("reclaims the raster bytes of a run that could not be stored", async () => {
      vi.spyOn(storage, "savePersistedStateForgetting").mockRejectedValue(
        new storage.BrowserStorageError("unavailable", "no IndexedDB"),
      );

      const failure = await browserBackend
        .startRun({
          ...RUN_SPEC,
          receiverMode: "auto-grid",
          params: {
            ...RUN_SPEC.params,
            grid_resolution_m: "50",
            grid_padding_m: "0",
          },
        })
        .catch((error: unknown) => error);

      expect(failure).toBeInstanceOf(Error);
      const runID = `run-${String(1).padStart(4, "0")}`;
      await vi.waitFor(async () => {
        expect(
          await storage.loadArtifactBytes(`artifact-${runID}-raster-bin`),
        ).toBeNull();
      });
    });
  });

  /*
   * Every deliberate path deletes the byte records it orphans — the run cap,
   * eviction, `deleteRun`, and `startRun` when its own persist fails. Each of
   * those is a caller that knows which ids it dropped. A tab closed between
   * the byte write and the document write leaves a record no caller ever knew
   * about, and until this sweep only `clearPersistedState` removed it.
   */
  describe("orphaned raster bytes", () => {
    const ORPHAN_ID = "artifact-run-0099-raster-bin";

    /*
     * The sweep only runs where the Web Locks API does, so every test that
     * expects it to happen has to supply one. jsdom has none, and that is not
     * an accident of the harness being thin — it is the same condition an old
     * Safari presents, and the code refuses to sweep in it deliberately: the
     * per-module latch says nothing about another tab, so an unlocked sweep
     * can delete the bytes a second tab has written and not yet named.
     *
     * The stub grants immediately and serialises nothing, which is all these
     * tests need; `sweeps while holding the store lock` below supplies a
     * stricter one that tracks whether the lock is actually held.
     */
    beforeEach(() => {
      Object.defineProperty(navigator, "locks", {
        value: {
          request: (_name: string, callback: () => Promise<unknown>) =>
            callback(),
        },
        configurable: true,
      });
    });

    afterEach(() => {
      Reflect.deleteProperty(navigator, "locks");
    });

    /** One stored run whose raster really is in the byte store. */
    async function seedOneRunWithRaster(): Promise<string> {
      const kept = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: { runs: [kept] },
      });
      resetBrowserBackendForTests();
      return kept.run.artifacts[0]?.id ?? "";
    }

    it("reclaims a byte record no stored document names", async () => {
      const keptBytesID = await seedOneRunWithRaster();
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));

      await browserBackend.startRun(RUN_SPEC);

      expect(await storage.loadArtifactBytes(ORPHAN_ID)).toBeNull();
      expect(await storage.loadArtifactBytes(keptBytesID)).not.toBeNull();
    });

    it("does not sweep at all where the Web Locks API is missing", async () => {
      // The condition the sweep refuses to run in, asserted rather than left
      // to the absence of an assertion. `sweptOrphanedBytes` is per module,
      // so it cannot serialise anything across tabs: without an origin-wide
      // lock, this tab could list the bytes another tab has just written and
      // not yet named, and delete a raster out from under a run that is
      // about to reference it. An orphan left alive costs quota; this would
      // cost the run.
      Reflect.deleteProperty(navigator, "locks");
      const keptBytesID = await seedOneRunWithRaster();
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));

      await browserBackend.startRun(RUN_SPEC);

      expect(await storage.loadArtifactBytes(ORPHAN_ID)).not.toBeNull();
      expect(await storage.loadArtifactBytes(keptBytesID)).not.toBeNull();
    });

    it("sweeps once per session, not on every write", async () => {
      await seedOneRunWithRaster();
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));

      await browserBackend.startRun(RUN_SPEC);
      expect(await storage.loadArtifactBytes(ORPHAN_ID)).toBeNull();

      // A record that appears after the sweep waits for the next session.
      // That is the cost of the constraint, not an oversight: `reloadState()`
      // runs on every write, and a sweep hung off it would be deleting the
      // bytes of the very run that write is storing.
      const later = "artifact-run-0098-raster-bin";
      await storage.saveArtifactBytes(later, new ArrayBuffer(8));
      await browserBackend.startRun(RUN_SPEC);

      expect(await storage.loadArtifactBytes(later)).not.toBeNull();
    });

    it("does not sweep when the stored document was unreadable", async () => {
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION + 1,
        state: {},
      });
      resetBrowserBackendForTests();

      await browserBackend.startRun(RUN_SPEC);

      // An unreadable document is left in place on purpose, so a later build
      // can still read it. What it names is therefore unknown, and a record
      // this session cannot account for is not a record it may delete.
      expect(await storage.loadArtifactBytes(ORPHAN_ID)).not.toBeNull();
    });

    it("does not sweep when the store was unavailable", async () => {
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));
      resetBrowserBackendForTests();
      vi.spyOn(storage, "loadPersistedState").mockRejectedValue(
        new storage.BrowserStorageError("unavailable", "no IndexedDB"),
      );

      await browserBackend.startRun(RUN_SPEC).catch(() => undefined);
      vi.restoreAllMocks();

      // Same reason: a store that could not be read said nothing about what
      // the document names, and "nothing named it" is not the same answer.
      expect(await storage.loadArtifactBytes(ORPHAN_ID)).not.toBeNull();
    });

    /*
     * `decodeState` silently drops a run entry that fails `isStoredRun`, so
     * its bytes look like orphans here. Sweeping them is the deliberate
     * choice: the entry is gone from every decoded state, and the next write
     * removes it from the document too, so nothing will ever name them again.
     */
    it("reclaims the bytes of a run entry the decoder rejected", async () => {
      // Index 5, not 1: a rejected entry raises no high-water mark, so the run
      // below is minted as `run-0001` and would write its own raster to the
      // very key this asserts about.
      const broken = await runFixtureWithRaster(5, "2026-01-01T05:00:00.000Z");
      const brokenBytesID = broken.run.artifacts[0]?.id ?? "";
      await storage.savePersistedState({
        version: PERSISTED_STATE_VERSION,
        state: { runs: [{ ...broken, log: "not a log" }] },
      });
      resetBrowserBackendForTests();

      await browserBackend.startRun(RUN_SPEC);

      expect(await storage.loadArtifactBytes(brokenBytesID)).toBeNull();
    });

    it("sweeps while holding the store lock", async () => {
      const keptBytesID = await seedOneRunWithRaster();
      await storage.saveArtifactBytes(ORPHAN_ID, new ArrayBuffer(8));

      let held = false;
      let heldDuringSweep: boolean | null = null;
      const request = vi.fn(
        async (_name: string, callback: () => Promise<unknown>) => {
          held = true;
          try {
            return await callback();
          } finally {
            held = false;
          }
        },
      );
      Object.defineProperty(navigator, "locks", {
        value: { request },
        configurable: true,
      });
      const swept: string[][] = [];
      // Records rather than deletes: what this test is about is *when* the
      // sweep runs, and a real delete would make the records it asserts on
      // disappear.
      vi.spyOn(storage, "deleteArtifactBytes").mockImplementation(
        (artifactIds: readonly string[]) => {
          heldDuringSweep = held;
          swept.push([...artifactIds]);
          return Promise.resolve();
        },
      );

      try {
        await browserBackend.startRun(RUN_SPEC);
      } finally {
        Reflect.deleteProperty(navigator, "locks");
      }

      // `startRun` holds this lock across its byte write and its document
      // write, so a sweep outside it could delete the bytes of a run another
      // tab is sitting between the two of.
      expect(request).toHaveBeenCalledWith(
        "aconiq-browser-backend",
        expect.any(Function),
      );
      expect(swept).toEqual([[ORPHAN_ID]]);
      expect(heldDuringSweep).toBe(true);
      expect(await storage.loadArtifactBytes(keptBytesID)).not.toBeNull();
    });
  });
});

describe("buildingFootprints", () => {
  it("keeps the courtyards buildBuildings drops, and splits a MultiPolygon", () => {
    const exterior: [number, number][] = [
      [0, 0],
      [10, 0],
      [10, 10],
      [0, 10],
      [0, 0],
    ];
    const courtyard: [number, number][] = [
      [3, 3],
      [7, 3],
      [7, 7],
      [3, 7],
      [3, 3],
    ];
    const features: ModelFeature[] = [
      {
        id: "block",
        kind: "building",
        heightM: 12,
        properties: {},
        geometry: {
          type: "MultiPolygon",
          coordinates: [[exterior, courtyard], [exterior]],
        },
      },
      {
        id: "screen",
        kind: "barrier",
        heightM: 3,
        properties: {},
        geometry: {
          type: "LineString",
          coordinates: [
            [0, 0],
            [1, 1],
          ],
        },
      },
    ];

    // A courtyard masked as building would blank open air, so the mask needs
    // the holes the kernel's extruded Building has no use for.
    expect(buildingFootprints(features)).toEqual([
      [exterior, courtyard],
      [exterior],
    ]);
    expect(buildBuildings(features).map((b) => b.footprint.length)).toEqual([
      5, 5,
    ]);
  });
});

describe("buildBuildings", () => {
  const footprint: ModelFeature = {
    id: "block",
    kind: "building",
    heightM: 12,
    properties: {},
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [0, 0],
          [10, 0],
          [10, 5],
          [0, 5],
          [0, 0],
        ],
      ],
    },
  };

  it("falls back to the Tabelle 8 facade row", () => {
    const buildings = buildBuildings([footprint]);

    expect(buildings).toHaveLength(1);
    expect(buildings[0]?.height_m).toBe(12);
    expect(buildings[0]?.reflection_loss_db).toBe(0.5);
  });

  it("honours an explicit reflection loss", () => {
    const buildings = buildBuildings([
      { ...footprint, properties: { reflection_loss_db: 3 } },
    ]);

    expect(buildings[0]?.reflection_loss_db).toBe(3);
  });

  // A dropped building is a receiver computed as though nothing stood there.
  // The CLI expands each part into its own building, so this must too.
  it("expands a MultiPolygon into one building per part, as the CLI does", () => {
    const buildings = buildBuildings([
      {
        ...footprint,
        geometry: {
          type: "MultiPolygon",
          coordinates: [
            [
              [
                [0, 0],
                [10, 0],
                [10, 5],
                [0, 5],
                [0, 0],
              ],
            ],
            [
              [
                [20, 0],
                [30, 0],
                [30, 5],
                [20, 5],
                [20, 0],
              ],
            ],
          ],
        },
      },
    ]);

    expect(buildings.map((b) => b.id)).toEqual(["block-01", "block-02"]);
    expect(buildings[1]?.footprint[0]).toEqual({ x: 20, y: 0 });
  });

  // PropagationConfig.Validate does not inspect buildings, so an unchecked
  // zero height would compute happily and shield nothing. Building.Validate
  // refuses it on the CLI side; this mirrors that.
  it("refuses a building with no stated height", () => {
    const withoutHeight: ModelFeature = {
      id: footprint.id,
      kind: footprint.kind,
      properties: {},
      geometry: footprint.geometry,
    };

    expect(() => buildBuildings([withoutHeight])).toThrow(/height_m/);
  });

  it.each([
    ["a zero height", { heightM: 0 }, /height_m/],
    [
      "a negative reflection loss",
      { properties: { reflection_loss_db: -1 } },
      /reflection_loss_db/,
    ],
  ])("refuses %s", (_name, patch, expected) => {
    expect(() => buildBuildings([{ ...footprint, ...patch }])).toThrow(
      expected,
    );
  });
});

describe("receiver grid extent", () => {
  // The CLI had to be taught this explicitly: padding a grid around a lot's
  // centroid alone puts the whole grid inside the source.
  it("covers every vertex of an area source, not just its middle", () => {
    const bbox = getFeatureBBox([
      {
        id: "lot",
        kind: "source",
        sourceType: "area",
        properties: {},
        geometry: {
          type: "Polygon",
          coordinates: [
            [
              [0, 0],
              [200, 0],
              [200, 100],
              [0, 100],
              [0, 0],
            ],
          ],
        },
      },
    ]);

    expect(bbox).toEqual({ minX: 0, minY: 0, maxX: 200, maxY: 100 });
  });
});

describe("browser-mode raster artifacts", () => {
  beforeEach(async () => {
    await resetStores();
    useModelStore.setState({
      features: [ROAD],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
  });

  /*
   * This slot held the run's SHA-256 hex string — 64 characters where a
   * float64 array belongs — because `StoredArtifactContent` could not
   * represent bytes at all. Nothing could read back the raster a browser run
   * had just computed, which is what blocked drawing it on the map.
   */
  it("stores the raster binary as bytes, not as a digest of it", async () => {
    const run = await browserBackend.startRun({
      ...RUN_SPEC,
      receiverMode: "auto-grid",
      params: {
        ...RUN_SPEC.params,
        grid_resolution_m: "25",
        grid_padding_m: "0",
      },
    });

    const binary = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.raster_binary",
    );
    expect(binary).toBeDefined();

    const bytes = await browserBackend.getArtifactContent<ArrayBuffer>(
      binary?.id ?? "",
    );
    // Not `toBeInstanceOf`: fake-indexeddb clones across a realm boundary, so
    // the buffer that comes back has every internal slot and still fails
    // `instanceof` — the same reason `browser-storage` checks the tag.
    expect(Object.prototype.toString.call(bytes)).toBe("[object ArrayBuffer]");

    const meta = await browserBackend.getArtifactContent<RasterMetadata>(
      run.artifacts.find((a) => a.kind === "run.result.raster_metadata")?.id ??
        "",
    );
    // Exactly what `results.SaveRaster` writes: one float64 per cell per band.
    expect(bytes.byteLength).toBe(meta.width * meta.height * meta.bands * 8);
  });

  /*
   * The georeference is what lets anything place the raster. It has to be the
   * padded grid's south-west corner — the centre of cell (0,0) — which is the
   * same convention `buildReceiversFromPoints` records on the CLI side.
   */
  it("records where the grid sits, in the compute CRS", async () => {
    const run = await browserBackend.startRun({
      ...RUN_SPEC,
      receiverMode: "auto-grid",
      params: {
        ...RUN_SPEC.params,
        grid_resolution_m: "25",
        grid_padding_m: "50",
      },
    });

    const meta = await browserBackend.getArtifactContent<RasterMetadata>(
      run.artifacts.find((a) => a.kind === "run.result.raster_metadata")?.id ??
        "",
    );

    expect(meta.crs).toBe("EPSG:25832");
    expect(meta.georeference).toEqual({
      // ROAD spans x 0..100, y 0..0; padding 50 puts the origin at (-50, -50).
      origin_x: -50,
      origin_y: -50,
      pixel_size_m: 25,
      row_order: "south-up",
    });

    const table = await browserBackend.getArtifactContent<ReceiverTable>(
      run.artifacts.find((a) => a.kind === "run.result.receiver_table_json")
        ?.id ?? "",
    );
    // The origin is the first receiver, on both targets.
    expect([table.records[0]?.x, table.records[0]?.y]).toEqual([-50, -50]);
  });

  /*
   * Explicit receivers are points the user placed. No cell size describes
   * them, and an invented georeference would be read by every GIS consumer
   * without a second opinion.
   */
  it("records no georeference for explicit receivers", async () => {
    useModelStore.setState({
      features: [ROAD],
      receivers: [
        {
          id: "R1",
          heightM: 4,
          geometry: { type: "Point", coordinates: [50, 20] },
        },
      ],
      calcArea: null,
      crs: "EPSG:25832",
    });

    const run = await browserBackend.startRun(RUN_SPEC);
    const meta = await browserBackend.getArtifactContent<RasterMetadata>(
      run.artifacts.find((a) => a.kind === "run.result.raster_metadata")?.id ??
        "",
    );

    expect(meta.georeference).toBeUndefined();
    expect(meta.crs).toBe("EPSG:25832");
  });

  /*
   * `getArtifactURL` is synchronous because pages put its result straight into
   * `<iframe src>`. Binary content is not in the document, so it cannot answer
   * — and a blob minted from `null` would read "null" to whoever opened it.
   */
  it("refuses to mint a synchronous URL for binary content", async () => {
    const run = await browserBackend.startRun({
      ...RUN_SPEC,
      receiverMode: "auto-grid",
      params: {
        ...RUN_SPEC.params,
        grid_resolution_m: "50",
        grid_padding_m: "0",
      },
    });
    const binaryId =
      run.artifacts.find((a) => a.kind === "run.result.raster_binary")?.id ??
      "";

    expect(() => browserBackend.getArtifactURL(binaryId)).toThrow(
      "holds binary content",
    );
  });
});

describe("browserBackend.getArtifactBytes", () => {
  beforeEach(async () => {
    await resetStores();
  });

  /**
   * Seeds one stored run whose raster artifact is declared in the document,
   * and puts its bytes in the byte store only when asked — which is what
   * makes "the two disagree" a state a test can create.
   */
  async function seedRasterRun(options: { withBytes: boolean }) {
    const fixture = await runFixtureWithRaster(1, "2026-01-01T01:00:00.000Z");
    const artifactId = fixture.run.artifacts[0]?.id ?? "";
    await storage.savePersistedState({
      version: PERSISTED_STATE_VERSION,
      state: { runs: [fixture] },
    });
    // The document keeps naming the artifact; only the bytes go. That is the
    // shape a partial quota eviction leaves behind.
    if (!options.withBytes) await storage.deleteArtifactBytes([artifactId]);
    resetBrowserBackendForTests();
    return artifactId;
  }

  it("returns the bytes stored beside the document", async () => {
    const artifactId = await seedRasterRun({ withBytes: true });

    const bytes = await browserBackend.getArtifactBytes(artifactId);

    // Not `toBeInstanceOf`: fake-indexeddb clones across a realm boundary, so
    // the buffer that comes back has every internal slot and still fails
    // `instanceof` — the same reason `browser-storage` checks the tag.
    expect(Object.prototype.toString.call(bytes)).toBe("[object ArrayBuffer]");
    expect(bytes.byteLength).toBe(64);
  });

  it("says so when the document names bytes the store does not hold", async () => {
    const artifactId = await seedRasterRun({ withBytes: false });

    // An empty buffer read as a raster is a grid of zeroes, which looks like
    // a result. The refusal is the whole point of the branch.
    await expect(browserBackend.getArtifactBytes(artifactId)).rejects.toThrow(
      "its bytes are not stored",
    );
  });

  it("refuses an artifact whose content is not binary", async () => {
    await seedState();

    await expect(
      browserBackend.getArtifactBytes(TABLE_ARTIFACT_ID),
    ).rejects.toThrow("not bytes");
  });
});

/**
 * The run assembly, with known levels going in.
 *
 * Two suites already drive `startRun`, and neither can see this layer. The
 * parity suites (`browser-parity.test.ts`, `kernel-parity.test.ts`) run the
 * real kernel and compare the numbers, so a defect in what surrounds the
 * numbers — a mislabelled band, a receiver table whose rows drifted out of the
 * golden's order, a summary field silently dropped — reads there as a passing
 * comparison. The rest of this file drives `startRun` for its persistence and
 * feeds it a flat 50/40, which every column agrees with, so a swap of two
 * columns is invisible.
 *
 * So: the CLI's own golden goes in through the stub, keyed by receiver id, and
 * everything the run *writes around* those values is asserted against it. The
 * dB values prove nothing here — they came from the stub — but they are
 * distinguishable per receiver and per indicator, which is what makes the
 * plumbing legible.
 *
 * The fixture is `backend/internal/app/cli/testdata/parity/`, written by
 * `parity_golden_test.go` through the CLI's own extraction; `just update-golden`
 * regenerates it.
 */
describe("browserBackend.startRun result assembly", () => {
  const PARITY_DIR = resolve(
    dirname(fileURLToPath(import.meta.url)),
    "../../../backend/internal/app/cli/testdata/parity",
  );

  interface GoldenReceiver {
    id: string;
    x: number;
    y: number;
    height_m: number;
    lr_day: number;
    lr_night: number;
  }

  const golden = JSON.parse(
    readFileSync(
      resolve(PARITY_DIR, "road_building_barrier.golden.json"),
      "utf8",
    ),
  ) as { receivers: GoldenReceiver[] };

  const collection = JSON.parse(
    readFileSync(resolve(PARITY_DIR, "road_building_barrier.geojson"), "utf8"),
  ) as GeoJSONFeatureCollection;

  /** The golden's levels, by receiver id — what the stubbed kernel reports. */
  const goldenLevels = new Map(
    golden.receivers.map((receiver) => [
      receiver.id,
      { lr_day: receiver.lr_day, lr_night: receiver.lr_night },
    ]),
  );

  let run: RunSummary;

  beforeEach(async () => {
    await resetStores();

    const { features, skipped } = normalizeModelGeoJSON(collection);
    // A silently skipped source or building would make the run smaller than
    // the golden and every count below would then be measuring the wrong
    // scene.
    expect(skipped).toEqual([]);

    useModelStore.setState({
      features,
      receivers: golden.receivers.map((receiver) => ({
        id: receiver.id,
        heightM: receiver.height_m,
        geometry: {
          type: "Point" as const,
          coordinates: [receiver.x, receiver.y] as [number, number],
        },
      })),
      calcArea: null,
      // The fixture is authored in metres. The stubbed `transform` reports
      // coordinates unmoved, so stating 4326 here would leave the numbers
      // right and the CRS fields wrong.
      crs: "EPSG:25832",
    });

    kernelLevels = (receiverID) =>
      goldenLevels.get(receiverID) ?? { lr_day: NaN, lr_night: NaN };

    vi.stubGlobal("URL", {
      ...URL,
      createObjectURL: () => "blob:mock/assembly",
      revokeObjectURL: () => undefined,
    });

    run = await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: {},
      receiverMode: "custom",
    });
  });

  afterEach(() => {
    kernelLevels = FLAT_LEVELS;
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  async function artifactOf<T>(kind: string): Promise<T> {
    const ref = run.artifacts.find((artifact) => artifact.kind === kind);
    expect(ref, `no artifact of kind ${kind}`).toBeDefined();
    return browserBackend.getArtifactContent<T>(ref?.id ?? "");
  }

  it("writes every receiver's levels into the table, in model order", async () => {
    const table = await artifactOf<ReceiverTable>(
      "run.result.receiver_table_json",
    );

    expect(table.records.map((record) => record.id)).toEqual(
      golden.receivers.map((receiver) => receiver.id),
    );
    // Column for column. A stub reporting one value for both indicators would
    // pass this with the two swapped; the golden's do not agree.
    for (const want of golden.receivers) {
      const record = table.records.find((entry) => entry.id === want.id);
      expect(record?.values["LrDay"], `${want.id} LrDay`).toBe(want.lr_day);
      expect(record?.values["LrNight"], `${want.id} LrNight`).toBe(
        want.lr_night,
      );
    }
    // The CLI's own indicator names, which is why the golden's snake_case
    // field names do not appear here: `parity_golden_test.go` writes its
    // snapshot in its own shape, and the container keeps the names a reader of
    // `receivers.csv` sees.
    expect(table.indicator_order).toEqual(["LrDay", "LrNight"]);
  });

  it("carries a unit per indicator, not one for the table", async () => {
    const table = await artifactOf<ReceiverTable>(
      "run.result.receiver_table_json",
    );

    expect(table.units).toEqual({ LrDay: "dB(A)", LrNight: "dB(A)" });
  });

  it("renders the CSV from the same table", async () => {
    const table = await artifactOf<ReceiverTable>(
      "run.result.receiver_table_json",
    );
    const csv = await artifactOf<string>("run.result.receiver_table_csv");

    // The canonical writer, not a second rendering: the CSV a browser run
    // stores has to be the bytes `receiver-csv.parity.test.ts` pins against
    // the Go writer.
    expect(csv).toBe(buildReceiverTableCSV(table));
    expect(csv.split("\n")[0]).toBe("id,x,y,height_m,LrDay,LrNight");
  });

  it("names the raster after the standard and labels both bands", async () => {
    const meta = await artifactOf<RasterMetadata>("run.result.raster_metadata");
    const ref = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.raster_metadata",
    );

    expect(ref?.path).toMatch(/\/results\/rls19-road\.json$/);
    expect(meta.bands).toBe(2);
    expect(meta.band_names).toEqual(["LrDay", "LrNight"]);
    expect(meta.nodata).toBe(-9999);
    expect(meta.crs).toBe("EPSG:25832");
    // Explicit receivers are not a grid, and no cell size describes them.
    expect(meta.georeference).toBeUndefined();
  });

  it("writes the raster bytes band by band, day before night", async () => {
    const ref = run.artifacts.find(
      (artifact) => artifact.kind === "run.result.raster_binary",
    );
    const bytes = await browserBackend.getArtifactBytes(ref?.id ?? "");
    const values = Array.from(new Float64Array(bytes));

    expect(ref?.path).toMatch(/\/results\/rls19-road\.bin$/);
    expect(values).toEqual([
      ...golden.receivers.map((receiver) => receiver.lr_day),
      ...golden.receivers.map((receiver) => receiver.lr_night),
    ]);
  });

  it("summarises the run with the counts and both CRS", async () => {
    const summary =
      await artifactOf<Record<string, unknown>>("run.result.summary");

    expect(summary).toMatchObject({
      run_id: run.id,
      status: "completed",
      receiver_count: golden.receivers.length,
      grid_width: 1,
      grid_height: golden.receivers.length,
      reporting_precision_db: 0.1,
      project_crs: "EPSG:25832",
      compute_crs: "EPSG:25832",
      // AGENTS.md requires the tier to travel with the result, and it is read
      // off the kernel's descriptor rather than named in browser-backend.ts.
      evidence_tier: "normative",
    });
    expect(summary["source_count"]).toBeGreaterThan(0);
  });

  it("produces exactly the five result artifacts", () => {
    expect(run.artifacts.map((artifact) => artifact.kind)).toEqual([
      "run.result.receiver_table_json",
      "run.result.receiver_table_csv",
      "run.result.raster_metadata",
      "run.result.raster_binary",
      "run.result.summary",
    ]);
  });

  it("logs the output hash of the receiver ids and their indicators", async () => {
    const log = await browserBackend.getRunLog(run.id);
    const line = log.lines.find((entry) => entry.includes("output_hash="));

    // Node's digest rather than the module's own `sha256Hex`, which is not
    // exported: a hash checked with the function that produced it agrees with
    // itself whatever either of them does.
    const expected = createHash("sha256")
      .update(
        JSON.stringify(
          golden.receivers.map((receiver) => ({
            receiver_id: receiver.id,
            indicators: {
              lr_day: receiver.lr_day,
              lr_night: receiver.lr_night,
            },
          })),
        ),
      )
      .digest("hex");

    // Recomputed here rather than snapshotted: a snapshot would be updated
    // along with whatever changed the hash, which is the one thing a run
    // digest must not let happen quietly.
    expect(line).toContain(`output_hash=${expected}`);
  });

  it("writes the raster bytes before the document that names them", async () => {
    // The order is the recovery story: a document naming bytes that are not
    // there reads as a corrupted run, while bytes no document names are merely
    // orphaned and get swept. Asserting it needs a second run, because the
    // first one already happened in beforeEach.
    const order: string[] = [];
    const realSaveBytes = storage.saveArtifactBytes;
    vi.spyOn(storage, "saveArtifactBytes").mockImplementation(
      async (artifactId, bytes) => {
        order.push("bytes");
        await realSaveBytes(artifactId, bytes);
      },
    );
    const realForgetting = storage.savePersistedStateForgetting;
    vi.spyOn(storage, "savePersistedStateForgetting").mockImplementation(
      async (value, forget) => {
        order.push("document");
        await realForgetting(value, forget);
      },
    );

    await browserBackend.startRun({
      standardId: "rls19-road",
      version: "2019",
      profile: "default",
      params: {},
      receiverMode: "custom",
    });

    // The whole sequence, not the two indices: `indexOf` on a missing entry
    // is -1, so "bytes first" would read as satisfied by a run that never
    // wrote any.
    expect(order).toEqual(["bytes", "document"]);
  });
});
