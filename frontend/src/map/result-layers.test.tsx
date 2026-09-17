import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { ReceiverTable, RunSummary } from "@/api/client";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { m } from "@/i18n/messages";
import { MapContext } from "./use-map";
import { useMapStore } from "./map-store";
import { LAYER_IDS, RESULT_RECEIVERS_GROUP_ID, SOURCE_IDS } from "./layers";
import { ResultLayers } from "./result-layers";

/**
 * What the map does with a completed run's receiver table.
 *
 * Two things are worth pinning beyond "it draws": the run it picks, because
 * the run list arrives in backend order and neither end of it is the newest;
 * and what it does when it cannot project, because an empty source is the only
 * honest answer and a stale one would put a previous run's levels under this
 * run's name.
 */
const state = vi.hoisted(() => {
  const value: {
    canReprojectForDisplay: boolean;
    runs: RunSummary[];
    table: ReceiverTable | undefined;
    summary: unknown;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    runs: [],
    table: undefined,
    summary: { compute_crs: "EPSG:25832", project_crs: "EPSG:25832" },
    requests: [],
    // A stand-in for the kernel: the numbers only have to be distinguishable
    // from the input, because what is asserted is which CRS was asked for and
    // that the answer reaches the source, never the projection's arithmetic —
    // that is `crstransform`'s, and it has its own parity tests.
    respond: (req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.target_crs,
        applied: true,
        coordinates: req.coordinates.map((value) => value / 100000),
      }),
  };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.canReprojectForDisplay ? "browser" : "http",
        canExport: false,
        runsAgainstSavedModel: false,
        runsChangeExternally: false,
        exportsOutliveRunDelete: false,
        canReprojectForDisplay: state.canReprojectForDisplay,
      };
    },
    transformCoordinates: (req: TransformRequest) => {
      state.requests.push(req);
      return state.respond(req);
    },
  },
}));

vi.mock("@/api/hooks", () => ({
  useRuns: () => ({ data: state.runs }),
  useReceiverTable: (artifactId: string | null) => ({
    data: artifactId === null ? undefined : state.table,
  }),
  useArtifactContent: (artifactId: string | null) => ({
    data: artifactId === null ? undefined : state.summary,
  }),
}));

/** The MapLibre surface `ResultLayers` touches, and only that. */
class FakeMap {
  readonly sources = new Map<string, { data: unknown }>();
  readonly layers = new Map<string, { layout?: { visibility?: string } }>();

  getStyle() {
    return { sources: {} };
  }

  getSource(id: string) {
    const source = this.sources.get(id);
    if (!source) return undefined;
    return {
      setData: (data: unknown) => {
        source.data = data;
      },
    };
  }

  addSource(id: string, source: { data: unknown }) {
    this.sources.set(id, { data: source.data });
  }

  getLayer(id: string) {
    return this.layers.get(id);
  }

  addLayer(layer: { id: string; layout?: { visibility?: string } }) {
    this.layers.set(layer.id, layer);
  }
}

function renderLayers(map: FakeMap) {
  render(
    <MapContext value={map as unknown as MapLibreMap | null}>
      <ResultLayers />
    </MapContext>,
  );
}

function drawn(map: FakeMap): GeoJSON.FeatureCollection {
  const source = map.sources.get(SOURCE_IDS.resultReceivers);
  if (!source) throw new Error("the receiver source was never added");
  return source.data as GeoJSON.FeatureCollection;
}

function levelsOf(collection: GeoJSON.FeatureCollection): unknown[] {
  return collection.features.map((feature) => {
    const properties = feature.properties as Record<string, unknown> | null;
    return properties?.["value"];
  });
}

function completedRun(id: string, finishedAt: string): RunSummary {
  return {
    id,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    status: "completed",
    started_at: "2026-01-01T10:00:00Z",
    finished_at: finishedAt,
    log_path: `runs/${id}/run.log`,
    artifacts: [
      {
        id: `${id}-receivers`,
        kind: "run.result.receiver_table_json",
        path: `runs/${id}/results/receivers.json`,
        created_at: finishedAt,
      },
      {
        id: `${id}-summary`,
        kind: "run.result.summary",
        path: `runs/${id}/results/run-summary.json`,
        created_at: finishedAt,
      },
    ],
  };
}

const TABLE: ReceiverTable = {
  indicator_order: ["LrDay", "LrNight"],
  unit: "dB(A)",
  records: [
    {
      id: "R1",
      x: 6660000,
      y: 564400000,
      height_m: 4,
      values: { LrDay: 62.4, LrNight: 55.1 },
    },
    // No LrNight: the table is allowed to omit an indicator for a receiver,
    // and a missing value must drop the point rather than paint it at zero.
    {
      id: "R2",
      x: 6661000,
      y: 564401000,
      height_m: 4,
      values: { LrDay: 71.2 },
    },
  ],
};

beforeEach(() => {
  state.canReprojectForDisplay = true;
  state.runs = [completedRun("run-1", "2026-01-01T10:00:05Z")];
  state.table = TABLE;
  state.summary = { compute_crs: "EPSG:25832", project_crs: "EPSG:25832" };
  state.requests = [];
  useMapStore.setState({ basemap: "light", layerVisibility: {} });
});

describe("ResultLayers", () => {
  it("projects the receivers out of the CRS the run summary names", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(drawn(map).features).toHaveLength(2);
    });

    expect(state.requests).toEqual([
      {
        source_crs: "EPSG:25832",
        target_crs: "EPSG:4326",
        coordinates: [6660000, 564400000, 6661000, 564401000],
      },
    ]);
    expect(drawn(map).features[0]?.geometry).toEqual({
      type: "Point",
      coordinates: [66.6, 5644],
    });
    expect(map.getLayer(LAYER_IDS.resultReceiverLevel)).toBeDefined();
  });

  it("draws the newest completed run, not the first or the last in the list", async () => {
    // The list arrives in backend order, so neither end of it is the newest —
    // the same reason `useRunFromRoute` refuses to fall back to `runs[0]`.
    // Ordering by id would be worse still: `run-10` precedes `run-2`.
    state.runs = [
      completedRun("run-2", "2026-01-02T10:00:00Z"),
      completedRun("run-1", "2026-01-01T10:00:00Z"),
      { ...completedRun("run-3", "2026-01-03T10:00:00Z"), status: "failed" },
    ];

    const map = new FakeMap();
    renderLayers(map);

    expect(await screen.findByText("run-2")).toBeInTheDocument();
  });

  it("paints the indicator the picker names, and only that one", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(levelsOf(drawn(map))).toEqual([62.4, 71.2]);
    });

    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    // R2 carries no LrNight, so it leaves the map rather than dropping to the
    // bottom of the ramp, which is a level somebody would read off it.
    await waitFor(() => {
      expect(levelsOf(drawn(map))).toEqual([55.1]);
    });

    // Switching band re-feeds the source; it must not re-project the geometry.
    expect(state.requests).toHaveLength(1);
  });

  it("empties the source rather than leaving a run's levels behind", async () => {
    const map = new FakeMap();
    const { rerender } = render(
      <MapContext value={map as unknown as MapLibreMap | null}>
        <ResultLayers />
      </MapContext>,
    );

    await waitFor(() => {
      expect(drawn(map).features).toHaveLength(2);
    });

    // A run written before either target recorded its CRS. The x/y are metres
    // in an unnamed projection, and drawing them as degrees would put them in
    // the Gulf of Guinea.
    state.summary = { project_crs: "EPSG:25832" };
    state.runs = [completedRun("run-9", "2026-02-01T10:00:00Z")];
    rerender(
      <MapContext value={map as unknown as MapLibreMap | null}>
        <ResultLayers />
      </MapContext>,
    );

    await waitFor(() => {
      expect(drawn(map).features).toEqual([]);
    });
  });

  it("says so, and draws nothing, when this mode cannot project", async () => {
    state.canReprojectForDisplay = false;

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(
        m.msg_result_levels_unprojectable({ crs: "EPSG:25832" }),
      ),
    ).toBeInTheDocument();
    expect(drawn(map).features).toEqual([]);
    expect(state.requests).toEqual([]);
  });

  it("refuses a short transform answer instead of shifting the levels", async () => {
    // The positions stay aligned with the records by index alone, so a
    // truncated batch would move every level after the gap onto the wrong
    // receiver — a wrong number under a real receiver id, which is worse than
    // no map at all.
    state.respond = (req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.target_crs,
        applied: true,
        coordinates: req.coordinates.slice(0, 2),
      });

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.msg_result_levels_failed()),
    ).toBeInTheDocument();
    expect(drawn(map).features).toEqual([]);

    state.respond = (req) =>
      Promise.resolve({
        source_crs: req.source_crs,
        target_crs: req.target_crs,
        applied: true,
        coordinates: req.coordinates.map((value) => value / 100000),
      });
  });

  it("adds the layer hidden when the control was switched off first", async () => {
    // The layer is added long after the layer control is clickable — it exists
    // only once a run has been projected — so a group switched off on an empty
    // map would come back on by itself the moment a run arrived.
    useMapStore.setState({
      layerVisibility: { [RESULT_RECEIVERS_GROUP_ID]: false },
    });

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultReceiverLevel)).toBeDefined();
    });
    expect(map.getLayer(LAYER_IDS.resultReceiverLevel)?.layout).toEqual({
      visibility: "none",
    });
  });

  it("shows nothing at all until a run has completed", () => {
    state.runs = [];

    const map = new FakeMap();
    renderLayers(map);

    expect(
      screen.queryByRole("region", { name: m.label_result_levels() }),
    ).not.toBeInTheDocument();
  });
});
