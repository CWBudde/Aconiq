import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { RunContours } from "@/api/backend";
import type { RasterMetadata, ReceiverTable, RunSummary } from "@/api/client";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { m } from "@/i18n/messages";
import { MapContext } from "./use-map";
import { useMapStore } from "./map-store";
import {
  BOTTOM_MODEL_LAYER_ID,
  LAYER_IDS,
  RESULT_CONTOURS_GROUP_ID,
  RESULT_LEVEL_PROPERTY,
  RESULT_RASTER_GROUP_ID,
  RESULT_RECEIVERS_GROUP_ID,
  SOURCE_IDS,
} from "./layers";
import { buildRasterBinary } from "@/model/raster-bin";
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
    tableError: Error | null;
    summary: unknown;
    summaryLoading: boolean;
    summaryError: Error | null;
    rasterMetadata: RasterMetadata | undefined;
    rasterBytes: ArrayBuffer | undefined;
    /** Every id `useArtifactBytes` was called with, `null` when disabled. */
    bytesAskedFor: (string | null)[];
    contours: RunContours | undefined;
    contoursError: Error | null;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    runs: [],
    table: undefined,
    tableError: null,
    summary: { compute_crs: "EPSG:25832", project_crs: "EPSG:25832" },
    summaryLoading: false,
    summaryError: null,
    rasterMetadata: undefined,
    rasterBytes: undefined,
    bytesAskedFor: [],
    contours: undefined,
    contoursError: null,
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
    data: artifactId === null || state.tableError ? undefined : state.table,
    isLoading: false,
    error: artifactId === null ? null : state.tableError,
  }),
  useArtifactContent: (artifactId: string | null) => ({
    data:
      artifactId === null || state.summaryLoading ? undefined : state.summary,
    isLoading: artifactId !== null && state.summaryLoading,
    error: artifactId === null ? null : state.summaryError,
  }),
  useRasterMetadata: (artifactId: string | null) => ({
    data: artifactId === null ? undefined : state.rasterMetadata,
    isLoading: false,
    error: null,
  }),
  useArtifactBytes: (artifactId: string | null) => {
    // Recorded, not just answered: a refusal the sidecar settles on its own
    // must never reach the network, and `null` here is what "never asked" is
    // spelled as — a query with no id does not fetch.
    state.bytesAskedFor.push(artifactId);
    return {
      data: artifactId === null ? undefined : state.rasterBytes,
      isLoading: false,
      error: null,
    };
  },
  // Keyed off the run id the hook passes, which is null for every case the
  // hook refuses before asking — no run, no raster artifact, no projector. A
  // mock that answered regardless would hide those refusals.
  useRunContours: (runId: string | null) => ({
    data: runId === null || state.contoursError ? undefined : state.contours,
    isLoading: false,
    error: runId === null ? null : state.contoursError,
  }),
}));

// jsdom has no 2D context, so the encoder is the seam: everything above it is
// arithmetic with its own tests, and the real canvas is `frontend/e2e/`'s.
vi.mock("./raster-canvas", () => ({
  encodeRasterPNG: (_rgba: Uint8ClampedArray, width: number, height: number) =>
    `data:image/png;base64,${String(width)}x${String(height)}`,
}));

interface FakeSource {
  type?: string;
  data?: unknown;
  url?: string;
  coordinates?: unknown;
}

/**
 * The MapLibre surface `ResultLayers` touches, and only that.
 *
 * `order` and `moveLayer` are not decoration. The result raster is the one
 * layer in this app inserted with a `beforeId`, because it arrives long after
 * `ModelLayers` has added its own and appending would draw it over the
 * buildings it is a result for — and a FakeMap that did not record insertion
 * position would pass whether that worked or not.
 */
class FakeMap {
  readonly sources = new Map<string, FakeSource>();
  readonly layers = new Map<string, { layout?: { visibility?: string } }>();
  /** Layer ids bottom to top, as MapLibre would hold them. */
  order: string[] = [];
  readonly moves: [string, string | undefined][] = [];
  readonly imageUpdates: { url: string; coordinates: unknown }[] = [];

  getStyle() {
    return { sources: {} };
  }

  getSource(id: string) {
    const source = this.sources.get(id);
    if (!source) return undefined;
    if (source.type === "image") {
      return {
        updateImage: (update: { url: string; coordinates: unknown }) => {
          source.url = update.url;
          source.coordinates = update.coordinates;
          this.imageUpdates.push(update);
        },
      };
    }
    return {
      setData: (data: unknown) => {
        source.data = data;
      },
    };
  }

  addSource(id: string, source: FakeSource) {
    this.sources.set(id, { ...source });
  }

  getLayer(id: string) {
    return this.layers.get(id);
  }

  addLayer(
    layer: { id: string; layout?: { visibility?: string } },
    beforeId?: string,
  ) {
    this.layers.set(layer.id, layer);
    const at = beforeId === undefined ? -1 : this.order.indexOf(beforeId);
    if (at < 0) this.order.push(layer.id);
    else this.order.splice(at, 0, layer.id);
  }

  setLayoutProperty(id: string, name: string, value: string) {
    const layer = this.layers.get(id);
    if (!layer) throw new Error(`no layer ${id}`);
    layer.layout = { ...layer.layout, [name]: value };
  }

  moveLayer(id: string, beforeId?: string) {
    this.moves.push([id, beforeId]);
    this.order = this.order.filter((existing) => existing !== id);
    const at = beforeId === undefined ? -1 : this.order.indexOf(beforeId);
    if (at < 0) this.order.push(id);
    else this.order.splice(at, 0, id);
  }

  /** What `ModelLayers` would already have added by the time a run arrives. */
  withModelLayers() {
    this.layers.set(BOTTOM_MODEL_LAYER_ID, {});
    this.order.push(BOTTOM_MODEL_LAYER_ID);
    return this;
  }
}

function renderLayers(
  map: FakeMap,
  requestedRunId: string | null = null,
  // A no-op default rather than an optional prop: `exactOptionalPropertyTypes`
  // refuses an explicit `undefined` for a `?:` prop, and the alternative —
  // spreading the prop in conditionally — reads as if the two cases differed.
  onRunDrawn: (run: RunSummary | null) => void = () => {},
) {
  render(
    <MapContext value={map as unknown as MapLibreMap | null}>
      <ResultLayers requestedRunId={requestedRunId} onRunDrawn={onRunDrawn} />
    </MapContext>,
  );
}

/**
 * The result layers in draw order, with the model's anchor kept for reference.
 *
 * Asserted as a sequence rather than as a pair of `indexOf` comparisons: the
 * raster, the halo and the line are stacked against *each other*, and two
 * separate "below the model" checks pass just as happily with the raster on
 * top of the contours it is supposed to sit under.
 */
function stackOf(map: FakeMap): string[] {
  const interesting = new Set<string>([
    LAYER_IDS.resultRaster,
    LAYER_IDS.resultContoursHalo,
    LAYER_IDS.resultContours,
    BOTTOM_MODEL_LAYER_ID,
  ]);

  return map.order.filter((id) => interesting.has(id));
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
  units: { LrDay: "dB(A)", LrNight: "dB(A)" },
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
  state.tableError = null;
  state.summary = {
    compute_crs: "EPSG:25832",
    project_crs: "EPSG:25832",
    evidence_tier: "normative",
  };
  state.summaryLoading = false;
  state.summaryError = null;
  state.contours = undefined;
  state.contoursError = null;
  state.requests = [];
  state.bytesAskedFor = [];
  // `resultIndicator` is in the store, and therefore persisted, so a picker
  // click in one test would otherwise choose the band the next one starts on.
  localStorage.clear();
  useMapStore.setState({
    basemap: "light",
    layerVisibility: {},
    resultIndicator: null,
  });
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

  /*
   * `onRunDrawn` is the only thing that tells the page which run these layers
   * resolved, and the page cannot work it out: the fallback to the newest
   * completed run is made here, over a run list the page deliberately does
   * not fetch. A click on a circle then links to whatever this reported.
   *
   * Tested against the real component, because the one other place it appears
   * is a stub in `pages/map.test.tsx` — a stub cannot catch the producer
   * drifting, and a wrong run here is not visible on screen: the map names
   * the run it drew, so a link to a stale one looks entirely ordinary.
   */
  describe("reporting the run it drew", () => {
    it("reports the requested run when the route names one", async () => {
      state.runs = [
        completedRun("run-1", "2026-01-01T10:00:00Z"),
        completedRun("run-2", "2026-01-02T10:00:00Z"),
      ];
      const drawn = vi.fn<(run: RunSummary | null) => void>();

      const map = new FakeMap();
      renderLayers(map, "run-1", drawn);

      await screen.findByText("run-1");
      expect(drawn.mock.calls.at(-1)?.[0]?.id).toBe("run-1");
    });

    it("reports the newest completed run when the route names none", async () => {
      state.runs = [
        completedRun("run-2", "2026-01-02T10:00:00Z"),
        completedRun("run-1", "2026-01-01T10:00:00Z"),
      ];
      const drawn = vi.fn<(run: RunSummary | null) => void>();

      const map = new FakeMap();
      renderLayers(map, null, drawn);

      await screen.findByText("run-2");
      expect(drawn.mock.calls.at(-1)?.[0]?.id).toBe("run-2");
    });

    it("reports the substitute, not the run that was asked for", async () => {
      // An id the project does not hold draws the newest completed run
      // instead. Reporting the *requested* id here would hand the page a run
      // the layers are not drawing, and a row link built from it would open a
      // different run's table than the circles the reader clicked.
      state.runs = [completedRun("run-2", "2026-01-02T10:00:00Z")];
      const drawn = vi.fn<(run: RunSummary | null) => void>();

      const map = new FakeMap();
      renderLayers(map, "run-does-not-exist", drawn);

      await screen.findByText("run-2");
      expect(drawn.mock.calls.at(-1)?.[0]?.id).toBe("run-2");
    });

    it("reports null while no run has completed", () => {
      state.runs = [];
      const drawn = vi.fn<(run: RunSummary | null) => void>();

      const map = new FakeMap();
      renderLayers(map, null, drawn);

      expect(drawn).toHaveBeenCalledWith(null);
    });

    it("reports the change when the requested run moves", async () => {
      state.runs = [
        completedRun("run-1", "2026-01-01T10:00:00Z"),
        completedRun("run-2", "2026-01-02T10:00:00Z"),
      ];
      const drawn = vi.fn<(run: RunSummary | null) => void>();
      const map = new FakeMap();

      const tree = (requested: string | null) => (
        <MapContext value={map as unknown as MapLibreMap | null}>
          <ResultLayers requestedRunId={requested} onRunDrawn={drawn} />
        </MapContext>
      );
      const { rerender } = render(tree("run-1"));
      await screen.findByText("run-1");
      expect(drawn.mock.calls.at(-1)?.[0]?.id).toBe("run-1");

      rerender(tree("run-2"));

      await screen.findByText("run-2");
      expect(drawn.mock.calls.at(-1)?.[0]?.id).toBe("run-2");
    });
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
        <ResultLayers requestedRunId={null} />
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
        <ResultLayers requestedRunId={null} />
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

  it("names the evidence tier the run summary carries", async () => {
    // A scaffold run's dB(A) are invented, and under the same legend as a
    // normative run's they read the same. The run id alone does not say which
    // of the two a viewer who did not start the run is looking at.
    state.summary = {
      compute_crs: "EPSG:25832",
      project_crs: "EPSG:25832",
      evidence_tier: "scaffold",
    };

    const map = new FakeMap();
    renderLayers(map);

    const badge = await screen.findByTestId("evidence-tier-badge");
    expect(badge).toHaveAttribute("data-tier", "scaffold");
  });

  it("paints a mixed table's decibel indicators and withholds its counts", async () => {
    // `beb-exposure` lists Lden and Lnight beside dwelling and person counts.
    // It used to declare one unit for the table — `"mixed"`, true of no column
    // — and this panel refused the whole run over it, so a reader got nothing
    // rather than the two indicators that really are decibels. Feeding a count
    // through the 35–80 dB ramp is still refused: that is what the per
    // indicator unit buys, and it is the half that must not regress.
    state.table = {
      indicator_order: ["Lden", "estimated_persons"],
      units: { Lden: "dB", estimated_persons: "count" },
      records: [
        {
          id: "B1",
          x: 6660000,
          y: 564400000,
          height_m: 9,
          values: { Lden: 62.4, estimated_persons: 12 },
        },
      ],
    };

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(drawn(map).features).toHaveLength(1);
    });

    // Offered, and the only one offered. A single indicator renders as a
    // label rather than a button, so the count's absence is asserted on the
    // text and the dB one on the panel reading it back.
    expect(screen.getByText("Lden")).toBeInTheDocument();
    expect(screen.queryByText("estimated_persons")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "estimated_persons" }),
    ).not.toBeInTheDocument();

    // The legend names the selected indicator's own unit, not the table's.
    expect(screen.getByText("(dB)", { exact: false })).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_result_table_not_levels()),
    ).not.toBeInTheDocument();
  });

  it("still refuses a table whose every indicator is a count", async () => {
    // The refusal did not go away, it narrowed: with no decibel indicator at
    // all there is nothing for the ramp to paint and the panel says so.
    state.table = {
      indicator_order: ["estimated_dwellings", "estimated_persons"],
      units: { estimated_dwellings: "count", estimated_persons: "count" },
      records: [
        {
          id: "B1",
          x: 6660000,
          y: 564400000,
          height_m: 9,
          values: { estimated_dwellings: 4, estimated_persons: 12 },
        },
      ],
    };

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.msg_result_table_not_levels()),
    ).toBeInTheDocument();
    expect(drawn(map).features).toEqual([]);
    expect(state.requests).toEqual([]);
  });

  it("says the CRS is unknown rather than showing a legend over nothing", async () => {
    // A run summary written before either target recorded the CRS. `idle`
    // left the picker and the legend up describing colours that were nowhere.
    state.summary = { project_crs: "EPSG:25832" };

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.msg_result_levels_unknown_crs()),
    ).toBeInTheDocument();
    expect(drawn(map).features).toEqual([]);
  });

  it("waits for the summary before calling the CRS unknown", () => {
    state.summaryLoading = true;

    const map = new FakeMap();
    renderLayers(map);

    expect(screen.queryByText(m.msg_result_levels_unknown_crs())).toBeNull();
  });

  it("distinguishes a failed fetch from absent data", async () => {
    // A transient 404/500 used to leave the source empty and the panel
    // showing a picker and a legend with nothing to explain either.
    state.tableError = new Error("boom");

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.error_load_result_levels()),
    ).toBeInTheDocument();
    expect(screen.queryByText(m.label_result_legend())).toBeNull();
  });
});

/**
 * The result raster: one image under the model, from the run's `.bin`.
 *
 * What these pin is placement and refusal. The pixels have their own tests in
 * `raster-image.test.ts` and the extent arithmetic in `raster-extent.test.ts`;
 * what only a map can answer is where the layer lands in the stack, that a
 * picker click updates the image instead of adding a second one, and that a
 * raster which cannot be drawn says why rather than simply not appearing.
 */
describe("ResultLayers: the result raster", () => {
  /** A 2x1 grid over two bands, already in EPSG:4326 so no transform is needed. */
  const RASTER_METADATA: RasterMetadata = {
    width: 2,
    height: 1,
    bands: 2,
    nodata: -9999,
    units: { LrDay: "dB(A)", LrNight: "dB(A)" },
    band_names: ["LrDay", "LrNight"],
    crs: "EPSG:4326",
    georeference: {
      origin_x: 10,
      origin_y: 50,
      pixel_size_m: 2,
      row_order: "south-up",
    },
  };

  function rasterRun(id: string, finishedAt: string): RunSummary {
    const run = completedRun(id, finishedAt);
    return {
      ...run,
      artifacts: [
        ...run.artifacts,
        {
          id: `${id}-raster-meta`,
          kind: "run.result.raster_metadata",
          path: `runs/${id}/results/rls19-road.json`,
          created_at: finishedAt,
        },
        {
          id: `${id}-raster-bin`,
          kind: "run.result.raster_binary",
          path: `runs/${id}/results/rls19-road.bin`,
          created_at: finishedAt,
        },
      ],
    };
  }

  beforeEach(() => {
    state.runs = [rasterRun("run-1", "2026-01-01T10:00:05Z")];
    state.rasterMetadata = RASTER_METADATA;
    state.rasterBytes = buildRasterBinary({ width: 2, height: 1, bands: 2 }, [
      [55, 65],
      [45, 50],
    ]);
  });

  it("adds the image below the model layers, and sinks it when they arrive later", async () => {
    // The order the raster is added in is the one it cannot rely on: the bytes
    // arrive long after `ModelLayers` has run, so appending would draw the
    // result over the buildings that produced it.
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)).toBeDefined();
    });

    // No anchor existed, so it was appended and no `beforeId` was passed —
    // MapLibre throws on one its style does not hold.
    expect(map.moves).toEqual([]);

    // The model layers land afterwards, over it. The next sync must sink it.
    map.withModelLayers();
    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    // Sunk under the contours rather than under the model directly, which is
    // the whole point of the anchor list: naming the model here would lift the
    // raster over the lines it is the backdrop for.
    await waitFor(() => {
      expect(map.moves).toContainEqual([
        LAYER_IDS.resultRaster,
        LAYER_IDS.resultContoursHalo,
      ]);
    });
    expect(stackOf(map)).toEqual([
      LAYER_IDS.resultRaster,
      LAYER_IDS.resultContoursHalo,
      LAYER_IDS.resultContours,
      BOTTOM_MODEL_LAYER_ID,
    ]);
  });

  it("anchors the image under the model when the model is already there", async () => {
    const map = new FakeMap().withModelLayers();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)).toBeDefined();
    });

    expect(stackOf(map)).toEqual([
      LAYER_IDS.resultRaster,
      LAYER_IDS.resultContoursHalo,
      LAYER_IDS.resultContours,
      BOTTOM_MODEL_LAYER_ID,
    ]);
  });

  it("places the image on the outer corners of the outer cells", async () => {
    // A 2x1 grid of 2 m cells centred on (10, 50) and (12, 50) covers 9..13 by
    // 49..51 — half a pixel beyond the receivers at its edges, in TL/TR/BR/BL.
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.sources.get(SOURCE_IDS.resultRaster)).toBeDefined();
    });

    expect(map.sources.get(SOURCE_IDS.resultRaster)?.coordinates).toEqual([
      [9, 51],
      [13, 51],
      [13, 49],
      [9, 49],
    ]);
    // Already in the display CRS, so no projection was asked for on its
    // account. The one request in flight is the receiver table's, whose CRS
    // comes off the run summary and is metric.
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]?.coordinates).toHaveLength(4);
  });

  it("updates the image on a picker click rather than adding a second source", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.sources.get(SOURCE_IDS.resultRaster)).toBeDefined();
    });

    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    await waitFor(() => {
      expect(map.imageUpdates).toHaveLength(1);
    });
    // The receivers, the raster and the contours — the three a run draws, and
    // no fourth. The contour source is added whether or not there are lines to
    // put in it, so its toggle answers the same way in every state.
    expect(map.sources.size).toBe(3);
  });

  it("keeps the chosen band when the layers are torn down and rebuilt", async () => {
    // The choice used to be component state, so leaving the map and coming
    // back silently reverted to the table's first band while the picker still
    // read as a deliberate choice the user had made.
    const first = new FakeMap();
    const { unmount } = render(
      <MapContext value={first as unknown as MapLibreMap | null}>
        <ResultLayers requestedRunId={null} />
      </MapContext>,
    );

    await waitFor(() => {
      expect(first.sources.get(SOURCE_IDS.resultRaster)).toBeDefined();
    });
    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "LrNight" })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    });

    unmount();
    renderLayers(new FakeMap());

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "LrNight" })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    });
  });

  it("falls back to the first band when the remembered one is not in this table", async () => {
    // A remembered indicator is a preference, not a promise. The band a run
    // was left on may simply not exist in the next run's table, and resolving
    // that on read — rather than seeding state from the stored value — is what
    // keeps a stale name from painting nothing at all.
    useMapStore.setState({ resultIndicator: "Lden" });

    renderLayers(new FakeMap());

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "LrDay" })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    });
    // Still remembered, not overwritten: going back to a run that does have
    // the band must return to it.
    expect(useMapStore.getState().resultIndicator).toBe("Lden");
  });

  it("brings the raster back after a newer run has loaded", async () => {
    // The regression Codex caught on #60. A newer run's artifacts pass through
    // `loading`, which hides the layer; the layer then *exists*, so the add
    // branch that restored visibility never ran again and the raster stayed
    // hidden until the user toggled it or the map was rebuilt. Everything here
    // is the ordinary case: no refusal, no toggle, just a second run finishing.
    const map = new FakeMap().withModelLayers();
    const { rerender } = render(
      <MapContext value={map as unknown as MapLibreMap | null}>
        <ResultLayers requestedRunId={null} />
      </MapContext>,
    );

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)).toBeDefined();
    });

    // The newer run's metadata has not arrived yet — the status the effect
    // hides on.
    state.rasterMetadata = undefined;
    rerender(
      <MapContext value={map as unknown as MapLibreMap | null}>
        <ResultLayers requestedRunId={null} />
      </MapContext>,
    );

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)?.layout?.visibility).toBe(
        "none",
      );
    });

    // And now it has.
    state.rasterMetadata = RASTER_METADATA;
    rerender(
      <MapContext value={map as unknown as MapLibreMap | null}>
        <ResultLayers requestedRunId={null} />
      </MapContext>,
    );

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)?.layout?.visibility).toBe(
        "visible",
      );
    });
  });

  it("honours a group switched off before the layer existed", async () => {
    useMapStore.setState({
      layerVisibility: { [RESULT_RASTER_GROUP_ID]: false },
    });

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultRaster)).toBeDefined();
    });

    expect(map.getLayer(LAYER_IDS.resultRaster)?.layout?.visibility).toBe(
      "none",
    );
  });

  it("says the receivers were not a grid, and draws no image", async () => {
    // `exactOptionalPropertyTypes` refuses an explicit `undefined` here, so the
    // key is removed rather than blanked — which is what an explicit-receiver
    // run's sidecar actually looks like.
    const withoutGeoreference: RasterMetadata = { ...RASTER_METADATA };
    delete withoutGeoreference.georeference;
    state.rasterMetadata = withoutGeoreference;

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.msg_result_raster_not_grid()),
    ).toBeInTheDocument();
    expect(map.sources.get(SOURCE_IDS.resultRaster)).toBeUndefined();
    expect(state.bytesAskedFor.every((id) => id === null)).toBe(true);
  });

  it("refuses a band the sidecar does not name, rather than painting band 0", async () => {
    // The picker's names are the receiver table's and the bands are the
    // sidecar's. They agree today; a fallback to band 0 would paint LrNight
    // under the label LrDay the day they stop.
    state.rasterMetadata = { ...RASTER_METADATA, band_names: ["LrDay"] };

    const map = new FakeMap();
    renderLayers(map);

    fireEvent.click(await screen.findByRole("button", { name: "LrNight" }));

    expect(
      await screen.findByText(
        m.msg_result_raster_no_band({ indicator: "LrNight" }),
      ),
    ).toBeInTheDocument();
    // The image source still holds LrDay's pixels — an image source keeps
    // whatever it was last given — so the layer is hidden. Leaving it visible
    // would show one band under a sentence naming another.
    expect(map.getLayer(LAYER_IDS.resultRaster)?.layout?.visibility).toBe(
      "none",
    );
  });

  it("refuses a grid larger than one texture without ever reading its bytes", async () => {
    // The cap used to be applied *after* the memo had read the band, allocated
    // the RGBA buffer and encoded a PNG — so the long thin browser grid the cap
    // exists for allocated hundreds of megabytes to put a one-line refusal on
    // screen. Withholding the bytes is what proves the order: nothing can have
    // decoded them.
    state.rasterMetadata = { ...RASTER_METADATA, width: 5000, height: 2 };
    state.rasterBytes = undefined;

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(
        m.msg_result_raster_too_large({ width: 5000, height: 2 }),
      ),
    ).toBeInTheDocument();
    expect(map.sources.get(SOURCE_IDS.resultRaster)).toBeUndefined();
    expect(map.getLayer(LAYER_IDS.resultReceiverLevel)).toBeDefined();
    // And never asked for them either: withholding the bytes proves the memo
    // did not decode them, this proves the hook did not download them. The
    // grid this cap exists for is tens of megabytes.
    expect(state.bytesAskedFor.every((id) => id === null)).toBe(true);
  });

  it("draws nothing and says nothing for a run that wrote no raster", async () => {
    // An explicit-receiver run places no grid. That is not a failure, so the
    // panel carries no sentence about it.
    state.runs = [completedRun("run-1", "2026-01-01T10:00:05Z")];

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultReceiverLevel)).toBeDefined();
    });

    expect(map.sources.get(SOURCE_IDS.resultRaster)).toBeUndefined();
    expect(screen.queryByText(m.msg_result_raster_not_grid())).toBeNull();
    expect(screen.queryByText(m.msg_result_raster_failed())).toBeNull();
  });
});

/**
 * Which run the map draws, when a link named one.
 *
 * The substitution is the point. A row followed from an older run used to land
 * on the newest run's levels with nothing on screen saying the two differed.
 */
describe("ResultLayers: the run a link asked for", () => {
  beforeEach(() => {
    state.runs = [
      completedRun("run-2", "2026-01-02T10:00:00Z"),
      completedRun("run-1", "2026-01-01T10:00:00Z"),
    ];
  });

  it("draws the run the link named, not the newest one", async () => {
    const map = new FakeMap();
    renderLayers(map, "run-1");

    expect(await screen.findByText("run-1")).toBeInTheDocument();
  });

  it("falls back to the newest run and names the one it could not show", async () => {
    const map = new FakeMap();
    renderLayers(map, "run-404");

    expect(
      await screen.findByText(
        m.msg_result_run_unavailable({ runId: "run-404" }),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("run-2")).toBeInTheDocument();
  });

  it("treats a run that has not completed the same way, and says which", async () => {
    // Not "unknown": the run exists and the reader can see it in the run list.
    state.runs = [
      ...state.runs,
      { ...completedRun("run-3", ""), status: "running" },
    ];

    const map = new FakeMap();
    renderLayers(map, "run-3");

    expect(
      await screen.findByText(m.msg_result_run_unavailable({ runId: "run-3" })),
    ).toBeInTheDocument();
    expect(screen.getByText("run-2")).toBeInTheDocument();
  });
});

/**
 * What the map does with the lines Go traced.
 *
 * Nothing here checks *where* a contour falls — that is
 * `contour/parity_test.go` and `wasm/contours.parity.test.ts`, which pin the
 * two boundaries against one golden. What is pinned here is everything between
 * the answer and the screen: the band filter, the stacking, the refusal, and
 * the toggle.
 */
describe("ResultLayers: the result contours", () => {
  const CONTOURS: RunContours = {
    crs: "EPSG:4326",
    interval: 5,
    lines: [
      {
        level: 55,
        band_name: "LrDay",
        points: [
          [10, 50],
          [11, 50],
          [11, 51],
        ],
      },
      {
        level: 60,
        band_name: "LrDay",
        points: [
          [10.2, 50.2],
          [10.8, 50.2],
        ],
      },
      {
        level: 45,
        band_name: "LrNight",
        points: [
          [10.1, 50.1],
          [10.9, 50.1],
        ],
      },
      // Dropped: MapLibre accepts a one-vertex LineString and draws nothing,
      // so it would be a feature that exists and is invisible.
      { level: 65, band_name: "LrDay", points: [[10.5, 50.5]] },
    ],
  };

  function contourRun(id: string, finishedAt: string): RunSummary {
    const run = completedRun(id, finishedAt);
    return {
      ...run,
      artifacts: [
        ...run.artifacts,
        {
          id: `${id}-raster-meta`,
          kind: "run.result.raster_metadata",
          path: `runs/${id}/results/rls19-road.json`,
          created_at: finishedAt,
        },
      ],
    };
  }

  function contoursOf(map: FakeMap): GeoJSON.FeatureCollection {
    const source = map.sources.get(SOURCE_IDS.resultContours);
    if (!source) throw new Error("the contour source was never added");
    return source.data as GeoJSON.FeatureCollection;
  }

  /** The level each drawn line carries, in draw order. */
  function levelsOf(map: FakeMap): unknown[] {
    return contoursOf(map).features.map((feature) => {
      const properties = feature.properties as Record<string, unknown> | null;
      return properties?.[RESULT_LEVEL_PROPERTY];
    });
  }

  beforeEach(() => {
    state.runs = [contourRun("run-1", "2026-01-01T10:00:05Z")];
    state.contours = CONTOURS;
    // No sidecar, so the raster layer is inert and every assertion below is
    // about the lines alone. `contourRun` carries no binary artifact either.
    state.rasterMetadata = undefined;
  });

  it("draws the chosen band's lines, each carrying its own level", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(contoursOf(map).features).toHaveLength(2);
    });

    // The level rides on the property `RESULT_CONTOUR_LAYERS` interpolates
    // over, which is what makes one source paint five colours.
    expect(levelsOf(map)).toEqual([55, 60]);
    expect(contoursOf(map).features[0]?.geometry).toEqual({
      type: "LineString",
      coordinates: [
        [10, 50],
        [11, 50],
        [11, 51],
      ],
    });
  });

  it("moves to the other band without asking for the lines again", async () => {
    // One request carries every band — `GenerateContours` walks them all — so
    // the picker filters what is already here. Re-requesting would put a
    // round trip behind a click that has the answer in hand.
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(contoursOf(map).features).toHaveLength(2);
    });

    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    await waitFor(() => {
      expect(contoursOf(map).features).toHaveLength(1);
    });
    expect(levelsOf(map)).toEqual([45]);
    expect(map.sources.size).toBe(2); // the receivers and the contours, no more
  });

  it("empties the source when the backend refuses, rather than stranding a band", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(contoursOf(map).features).toHaveLength(2);
    });

    state.contoursError = new Error(
      "this run's raster declares no georeference",
    );
    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    await waitFor(() => {
      expect(contoursOf(map).features).toHaveLength(0);
    });
    // The layer stays where the control put it: hiding it here would make the
    // toggle report a state nothing set.
    expect(map.getLayer(LAYER_IDS.resultContours)?.layout?.visibility).toBe(
      "visible",
    );
  });

  it("says so in its own words, with the backend's reachable", async () => {
    state.contoursError = new Error("contours can only be moved between CRS…");

    const map = new FakeMap();
    renderLayers(map);

    const notice = await screen.findByText(m.msg_result_contours_failed());
    expect(notice).toHaveAttribute(
      "title",
      "contours can only be moved between CRS…",
    );
  });

  it("stays silent when the panel has already said the receivers are no grid", async () => {
    // `ErrNotAGrid` is raised on exactly the condition `useResultRaster`
    // reports as `not-a-grid`, so both notices would name one cause twice.
    // The raster path needs both artifacts before it will say anything.
    const run = contourRun("run-1", "2026-01-01T10:00:05Z");
    state.runs = [
      {
        ...run,
        artifacts: [
          ...run.artifacts,
          {
            id: "run-1-raster-bin",
            kind: "run.result.raster_binary",
            path: "runs/run-1/results/rls19-road.bin",
            created_at: "2026-01-01T10:00:05Z",
          },
        ],
      },
    ];
    state.rasterMetadata = {
      width: 2,
      height: 1,
      bands: 2,
      nodata: -9999,
      units: { LrDay: "dB(A)", LrNight: "dB(A)" },
      band_names: ["LrDay", "LrNight"],
      crs: "EPSG:4326",
    };
    state.contoursError = new Error(
      "this run's raster declares no georeference",
    );

    const map = new FakeMap();
    renderLayers(map);

    expect(
      await screen.findByText(m.msg_result_raster_not_grid()),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_result_contours_failed()),
    ).not.toBeInTheDocument();
  });

  it("honours a group switched off before the layers existed", async () => {
    useMapStore.setState({
      layerVisibility: { [RESULT_CONTOURS_GROUP_ID]: false },
    });

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultContours)).toBeDefined();
    });

    expect(map.getLayer(LAYER_IDS.resultContoursHalo)?.layout?.visibility).toBe(
      "none",
    );
    expect(map.getLayer(LAYER_IDS.resultContours)?.layout?.visibility).toBe(
      "none",
    );
  });

  it("keeps the line over its own halo when both are sunk under the model", async () => {
    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.getLayer(LAYER_IDS.resultContours)).toBeDefined();
    });

    map.withModelLayers();
    fireEvent.click(screen.getByRole("button", { name: "LrNight" }));

    await waitFor(() => {
      expect(map.moves).toContainEqual([
        LAYER_IDS.resultContours,
        BOTTOM_MODEL_LAYER_ID,
      ]);
    });
    expect(stackOf(map)).toEqual([
      LAYER_IDS.resultContoursHalo,
      LAYER_IDS.resultContours,
      BOTTOM_MODEL_LAYER_ID,
    ]);
  });

  it("asks for nothing when the run wrote no raster to trace", async () => {
    // The refusal is knowable from the artifact list, so it costs no request.
    state.runs = [completedRun("run-1", "2026-01-01T10:00:05Z")];

    const map = new FakeMap();
    renderLayers(map);

    await waitFor(() => {
      expect(map.sources.get(SOURCE_IDS.resultContours)).toBeDefined();
    });
    expect(contoursOf(map).features).toHaveLength(0);
    expect(
      screen.queryByText(m.msg_result_contours_failed()),
    ).not.toBeInTheDocument();
  });
});
