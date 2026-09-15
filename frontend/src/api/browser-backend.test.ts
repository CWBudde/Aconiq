import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  browserBackend,
  buildBuildings,
  buildRoadSources,
  getFeatureBBox,
  MAX_STORED_RUNS,
  overpassWayToFeature,
  PERSISTED_STATE_VERSION,
  resetBrowserBackendForTests,
} from "./browser-backend";
import * as storage from "./browser-storage";
import { useModelStore } from "@/model/model-store";
import { buildReceiverTableCSV } from "@/model/receiver-csv";
import type { ModelFeature } from "@/model/types";
import type { ComputeRequest } from "@/wasm/types";

// The persistence tests drive `startRun` end to end, but what the kernel
// computes is the parity suite's business; here it only has to answer.
vi.mock("@/wasm/kernel", () => ({
  getKernel: () =>
    Promise.resolve({
      rls19Road: (req: ComputeRequest) =>
        Promise.resolve(
          req.receivers.map((receiver) => ({
            Receiver: receiver,
            Indicators: { lr_day: 50, lr_night: 40 },
          })),
        ),
      defaultConfig: () => ({}),
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
    });
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
        // projectId absent, crs of the wrong type, one run entry that is not
        // a run: an older build could have written any of these.
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
    expect(status.crs).toBe("WGS84 / web map");
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
        .spyOn(storage, "savePersistedState")
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
        .spyOn(storage, "savePersistedState")
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
      vi.spyOn(storage, "savePersistedState").mockRejectedValue(quota());

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
        .spyOn(storage, "savePersistedState")
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

    it("surfaces a non-quota storage failure without evicting", async () => {
      await browserBackend.getRuns();
      const save = vi
        .spyOn(storage, "savePersistedState")
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
