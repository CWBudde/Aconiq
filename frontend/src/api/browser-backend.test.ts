import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  browserBackend,
  buildRoadSources,
  overpassWayToFeature,
} from "./browser-backend";
import type { ModelFeature } from "@/model/types";

describe("buildRoadSources", () => {
  it("prefers feature-level RLS-19 overrides over run defaults", () => {
    const features: ModelFeature[] = [
      {
        id: "road-a",
        kind: "source",
        sourceType: "line",
        properties: {
          surface_type: "Beton",
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
    expect(sources[0]?.surface_type).toBe("Beton");
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

const STORAGE_KEY = "aconiq.browser_backend.v1";
const RUN_ID = "run-0001";
const TABLE_ARTIFACT_ID = "artifact-run-0001-receivers-json";
const SECOND_RUN_ID = "run-0002";
const SECOND_ARTIFACT_ID = "artifact-run-0002-summary";

function seedState({ withSecondRun = false } = {}) {
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
  window.localStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({
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
    }),
  );
}

describe("artifact object URLs", () => {
  let created: string[] = [];
  let revoked: string[] = [];

  // The blob URL cache is module state that outlives a single test, so the
  // mock counter must not restart — two tests would otherwise mint the same
  // URL string and the identity assertions below would be meaningless.
  let counter = 0;

  beforeEach(() => {
    window.localStorage.clear();
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
    seedState();
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
    seedState({ withSecondRun: true });
    const kept = browserBackend.getArtifactURL(TABLE_ARTIFACT_ID);
    const dropped = browserBackend.getArtifactURL(SECOND_ARTIFACT_ID);
    expect(dropped).not.toBe(kept);

    // Drop the second run, then write state through the public API. Pruning
    // has to release the blobs of artifacts that are gone — otherwise the fix
    // above would just be a leak.
    seedState();
    await browserBackend.createExport(RUN_ID);

    expect(revoked).toContain(dropped);
    expect(revoked).not.toContain(kept);
  });
});
