/**
 * The computed levels of a completed run, drawn on the workspace map.
 *
 * Two layers, from the two result containers a run writes. The **receiver
 * table** becomes one circle per receiver; the **raster** becomes one image
 * under the model, through `use-result-raster.ts`. The raster was unreachable
 * until recently and this file said so — browser mode stored the run's
 * SHA-256 where the binary belonged, `StoredArtifactContent.encoding` had no
 * binary case, and `results.RasterMetadata` carried no geotransform to place a
 * grid with. All three are closed. Contours are still out, and for a reason of
 * their own — see `layers.ts`, {@link RESULT_LAYER_GROUPS}.
 *
 * Both draw a container that declares decibels and no other: `NOISE_LEVEL_RAMP`
 * is a decibel ramp, and not every receiver table holds levels — see
 * `result-units.ts`. The run's evidence tier rides along beside its id, so a
 * scaffold run is not read under the same legend as a normative one.
 *
 * Which run is drawn: the one the reader came from, when `/model` was asked
 * for one, and the newest completed run otherwise. A substitution is said out
 * loud rather than made quietly — a row followed from an older run used to
 * land on the newest run's levels, under nothing that said the two differed.
 *
 * The coordinates are projected through `backend.transformCoordinates` — the
 * kernel's own projection, the same one `display-model.ts` draws the model
 * with — so a run and the model it came from land in the same place. The
 * receiver table's CRS is read off the run summary, which is where both
 * targets record it; the raster's is read off its own sidecar.
 */

import { useEffect, useMemo, useRef, useState } from "react";
import type maplibregl from "maplibre-gl";
import { backend } from "@/api/backend";
import type { ReceiverRecord, RunSummary } from "@/api/client";
import { useArtifactContent, useReceiverTable } from "@/api/hooks";
import { useRunFromRoute } from "@/run/use-run-from-route";
import { Button } from "@/ui/components/button";
import { EvidenceTierBadge } from "@/ui/evidence-tier-badge";
import { m } from "@/i18n/messages";
import { NOISE_LEVEL_RAMP } from "./color-ramp";
import { DISPLAY_CRS } from "./display-model";
import {
  BOTTOM_MODEL_LAYER_ID,
  RESULT_LEVEL_PROPERTY,
  RESULT_RASTER_GROUP_ID,
  RESULT_RASTER_LAYERS,
  RESULT_RECEIVER_LAYERS,
  RESULT_RECEIVERS_GROUP_ID,
  SOURCE_IDS,
} from "./layers";
import { MapPanel } from "./map-panel";
import { useMapStore } from "./map-store";
import { declaresLevels } from "./result-units";
import { useMap } from "./use-map";
import { useResultRaster, type ResultRaster } from "./use-result-raster";

/**
 * Module-level, so "nothing to draw" is the *same* value on every render. A
 * fresh literal per render would change the sync effect's dependencies every
 * time and re-run it forever.
 */
const EMPTY_COLLECTION: GeoJSON.FeatureCollection = {
  type: "FeatureCollection",
  features: [],
};
const NO_RECORDS: ReceiverRecord[] = [];
const NO_INDICATORS: string[] = [];

/** Projected receiver positions, in {@link DISPLAY_CRS}, or why there are none. */
type ProjectedPositions =
  | { status: "idle" }
  | { status: "projecting" }
  /** No projector in this mode — see `BackendCapabilities.canReprojectForDisplay`. */
  | { status: "unsupported"; crs: string }
  /** The run never recorded the CRS its receiver coordinates are in. */
  | { status: "unknown-crs" }
  | { status: "failed" }
  /** Parallel to the records they were built from, index for index. */
  | { status: "ready"; positions: GeoJSON.Position[] };

const IDLE: ProjectedPositions = { status: "idle" };
const UNKNOWN_CRS: ProjectedPositions = { status: "unknown-crs" };

/**
 * The newest completed run, by the time it finished.
 *
 * Not `runs[0]` and not `runs.at(-1)`: the list arrives in backend order, so
 * neither end of it is the newest — the same reason `useRunFromRoute` refuses
 * to fall back to the first run. A run with no finish time yet is ordered by
 * its start, so a manifest written by an older build still sorts.
 */
function latestCompletedRun(runs: RunSummary[]): RunSummary | null {
  let latest: RunSummary | null = null;

  for (const run of runs) {
    if (run.status !== "completed") continue;
    if (latest === null || finishOrder(run) > finishOrder(latest)) {
      latest = run;
    }
  }

  return latest;
}

function finishOrder(run: RunSummary): string {
  return run.finished_at === "" ? run.started_at : run.finished_at;
}

/** One non-empty string off the parsed `run-summary.json`, or nothing. */
function summaryField(summary: unknown, key: string): string | undefined {
  if (typeof summary !== "object" || summary === null) return undefined;

  const value = (summary as Record<string, unknown>)[key];
  return typeof value === "string" && value !== "" ? value : undefined;
}

/**
 * The CRS the run's receiver coordinates are in, read off its own run summary.
 *
 * `null` for a run written before either target recorded it. That is not the
 * same as "WGS84": the coordinates would be metres in some unnamed projection,
 * and drawing them as degrees is the bug this whole file is careful about.
 */
function computeCRSOf(summary: unknown): string | null {
  return summaryField(summary, "compute_crs") ?? null;
}

/**
 * The receiver positions in {@link DISPLAY_CRS}.
 *
 * Keyed on the records and the CRS alone, never on the chosen indicator: the
 * picker moves far more often than the geometry does, and re-projecting a
 * quarter of a million receivers to recolour them would be a batch transform
 * per click.
 */
function useProjectedPositions(
  records: ReceiverRecord[],
  computeCRS: string | null,
  crsPending: boolean,
): ProjectedPositions {
  const canReproject = backend.capabilities.canReprojectForDisplay;
  const [state, setState] = useState<ProjectedPositions>(IDLE);

  // Monotonic, and compared on arrival: a run switching under a slow transform
  // must not be overwritten by the answer to the question before it, which
  // would draw the previous run's receivers in this one's place.
  const requestRef = useRef(0);

  useEffect(() => {
    if (records.length === 0) {
      requestRef.current += 1;
      setState(IDLE);
      return;
    }

    // No CRS, and none still on its way: the run summary either predates the
    // two keys or never had them. `idle` would leave the panel showing a
    // legend over an empty map with nothing to explain it, so this case gets
    // a state of its own and a sentence to go with it.
    if (computeCRS === null) {
      requestRef.current += 1;
      setState(crsPending ? IDLE : UNKNOWN_CRS);
      return;
    }

    // Already what MapLibre draws in. The identity short-circuit is not an
    // optimisation: `wasmkernel.resolveTarget` transforms unconditionally on
    // an explicit target, so a 4326 → 4326 request would build a projection
    // pipeline for nothing.
    if (computeCRS === DISPLAY_CRS) {
      requestRef.current += 1;
      setState({ status: "ready", positions: records.map((r) => [r.x, r.y]) });
      return;
    }

    if (!canReproject) {
      requestRef.current += 1;
      setState({ status: "unsupported", crs: computeCRS });
      return;
    }

    const request = (requestRef.current += 1);
    setState({ status: "projecting" });

    const coordinates: number[] = [];
    for (const record of records) {
      coordinates.push(record.x, record.y);
    }

    void backend
      .transformCoordinates({
        source_crs: computeCRS,
        target_crs: DISPLAY_CRS,
        coordinates,
      })
      .then(
        (response) => {
          if (requestRef.current !== request) return;
          setState(toPositions(response.coordinates, records.length));
        },
        () => {
          if (requestRef.current !== request) return;
          setState({ status: "failed" });
        },
      );
  }, [records, computeCRS, crsPending, canReproject]);

  return state;
}

/**
 * Pairs a flat response back up into positions, and refuses a short one.
 *
 * A truncated batch is worse than no answer: the positions stay aligned with
 * the records by index alone, so a missing pair silently shifts every level
 * after it onto the wrong receiver.
 */
function toPositions(coordinates: number[], count: number): ProjectedPositions {
  if (coordinates.length !== count * 2) return { status: "failed" };

  const positions: GeoJSON.Position[] = [];
  for (let i = 0; i < count; i += 1) {
    const x = coordinates[i * 2];
    const y = coordinates[i * 2 + 1];
    if (x === undefined || y === undefined) return { status: "failed" };
    positions.push([x, y]);
  }

  return { status: "ready", positions };
}

/**
 * One point per receiver that has a level for the chosen indicator, carrying
 * that level under {@link RESULT_LEVEL_PROPERTY}.
 *
 * A receiver the table has no value for is left out rather than drawn at zero:
 * the ramp's bottom colour is a real answer ("below 40 dB(A)") and inventing
 * it would put a green dot over a receiver nothing was computed for.
 */
function toCollection(
  records: ReceiverRecord[],
  positions: GeoJSON.Position[],
  indicator: string,
): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  for (const [index, record] of records.entries()) {
    const position = positions[index];
    const level = record.values[indicator];
    if (position === undefined || level === undefined) continue;

    features.push({
      type: "Feature",
      id: record.id,
      properties: { id: record.id, [RESULT_LEVEL_PROPERTY]: level },
      geometry: { type: "Point", coordinates: position },
    });
  }

  return { type: "FeatureCollection", features };
}

/**
 * `useRunFromRoute`'s eligibility rule, at module scope so its memo identity is
 * stable across renders — the same reason `results.tsx` hoists its own.
 */
function isCompleted(run: RunSummary): boolean {
  return run.status === "completed";
}

/**
 * Draws a completed run's receiver levels and result raster, and offers the
 * picker and the legend that make them readable.
 *
 * Must be rendered as a child of MapView (inside MapContext).
 */
export function ResultLayers({
  requestedRunId,
}: {
  /** The run `/model` was asked for, or `null`. See `map-params.ts`. */
  requestedRunId: string | null;
}) {
  const map = useMap();

  // The same resolver `/results` and `/export` use, so "which run does this id
  // mean" is answered once in the app. It refuses to fall back on its own;
  // falling back to the newest completed run is this page's decision, because
  // the map is not a run-detail page and an empty canvas explains nothing.
  const {
    runs,
    run: requested,
    state: runState,
  } = useRunFromRoute(requestedRunId ?? undefined, isCompleted);

  const newest = useMemo(() => latestCompletedRun(runs), [runs]);
  const run = requested ?? newest;
  const substituted = runState === "ineligible" || runState === "unknown";

  const tableArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.receiver_table_json",
  );
  const summaryArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.summary",
  );

  const { data: table, error: tableError } = useReceiverTable(
    tableArtifact?.id ?? null,
  );
  const {
    data: summary,
    isLoading: summaryLoading,
    error: summaryError,
  } = useArtifactContent<unknown>(summaryArtifact?.id ?? null);

  // A failed fetch is not absent data. Dropping the error left the panel
  // showing a picker and a legend over an empty source, indistinguishable
  // from a run whose coordinates could not be projected.
  const loadFailed = tableError != null || summaryError != null;

  const unit = table?.unit ?? "";
  // Unknown until the table arrives, and `declaresLevels("")` is false, so the
  // gate has to let an absent table through rather than call it a count table.
  const levelTable = table === undefined || declaresLevels(unit);

  const records = levelTable ? (table?.records ?? NO_RECORDS) : NO_RECORDS;
  const indicators = levelTable
    ? (table?.indicator_order ?? NO_INDICATORS)
    : NO_INDICATORS;
  const computeCRS = computeCRSOf(summary);
  // Still being fetched, so "no CRS" is not yet an answer.
  const crsPending = summaryArtifact !== undefined && summaryLoading;

  // The band on screen. `null` means "whatever the table lists first", which
  // is resolved on read rather than written into state: seeding it from an
  // effect would paint one commit with no layer at all, and a run switched
  // underneath would keep a band the new table may not have.
  const [chosen, setChosen] = useState<string | null>(null);
  const indicator =
    chosen !== null && indicators.includes(chosen)
      ? chosen
      : (indicators[0] ?? "");

  const projected = useProjectedPositions(records, computeCRS, crsPending);
  // The same `indicator` the circles use, so one picker click moves both
  // layers. The raster resolves it against the sidecar's own `band_names`
  // rather than trusting the two lists to agree.
  const raster = useResultRaster(run, indicator);

  const collection = useMemo(() => {
    if (projected.status !== "ready" || indicator === "") {
      return EMPTY_COLLECTION;
    }
    return toCollection(records, projected.positions, indicator);
  }, [records, projected, indicator]);

  // The choice the layer control already holds. Read here because the layer is
  // added long after the control is clickable — the result layers only exist
  // once a run is drawn — so a group switched off on an empty map would come
  // back on by itself the moment one arrived.
  const groupVisible = useMapStore(
    (s) => s.layerVisibility[RESULT_RECEIVERS_GROUP_ID] ?? true,
  );
  const rasterVisible = useMapStore(
    (s) => s.layerVisibility[RESULT_RASTER_GROUP_ID] ?? true,
  );

  useEffect(() => {
    if (!map) return;
    if (!isMapStyleReady(map)) return;

    try {
      const existing = map.getSource(SOURCE_IDS.resultReceivers);
      if (existing && "setData" in existing) {
        (existing as maplibregl.GeoJSONSource).setData(collection);
      } else if (!existing) {
        map.addSource(SOURCE_IDS.resultReceivers, {
          type: "geojson",
          data: collection,
        });
      }
    } catch (error) {
      console.error(
        `ResultLayers: could not sync GeoJSON source "${SOURCE_IDS.resultReceivers}"`,
        error,
      );
      return;
    }

    for (const layer of RESULT_RECEIVER_LAYERS) {
      try {
        if (!map.getLayer(layer.id)) {
          map.addLayer({
            ...layer,
            layout: { visibility: groupVisible ? "visible" : "none" },
          });
        }
      } catch (error) {
        console.error(`ResultLayers: could not add layer "${layer.id}"`, error);
        return;
      }
    }
  }, [map, collection, groupVisible]);

  useEffect(() => {
    if (!map) return;
    if (!isMapStyleReady(map)) return;

    // Anything but `ready` hides the layer rather than leaving it alone. The
    // image source keeps whatever it was last given, so a refusal that only
    // stopped updating would leave the previous band's pixels on screen under
    // a sentence saying that band could not be drawn.
    if (raster.status !== "ready") {
      try {
        for (const layer of RESULT_RASTER_LAYERS) {
          if (map.getLayer(layer.id)) {
            map.setLayoutProperty(layer.id, "visibility", "none");
          }
        }
      } catch (error) {
        console.error("ResultLayers: could not hide the result raster", error);
      }
      return;
    }

    try {
      const existing = map.getSource(SOURCE_IDS.resultRaster);
      if (existing && "updateImage" in existing) {
        // Both, always: the run can change under the picker, so the pixels and
        // the ground they sit on have to move together or one frame draws this
        // run's image on the previous run's extent.
        (existing as maplibregl.ImageSource).updateImage({
          url: raster.url,
          coordinates: raster.coordinates,
        });
      } else if (!existing) {
        map.addSource(SOURCE_IDS.resultRaster, {
          type: "image",
          url: raster.url,
          coordinates: raster.coordinates,
        });
      }
    } catch (error) {
      console.error(
        `ResultLayers: could not sync image source "${SOURCE_IDS.resultRaster}"`,
        error,
      );
      return;
    }

    for (const layer of RESULT_RASTER_LAYERS) {
      try {
        // The anchor has to exist before it is named: MapLibre throws on a
        // `beforeId` its style does not hold, and this layer is usually added
        // before `ModelLayers` has added any of its own.
        const anchored = Boolean(map.getLayer(BOTTOM_MODEL_LAYER_ID));

        if (!map.getLayer(layer.id)) {
          map.addLayer(
            {
              ...layer,
              layout: { visibility: rasterVisible ? "visible" : "none" },
            },
            anchored ? BOTTOM_MODEL_LAYER_ID : undefined,
          );
          // Back on after a refusal hid it. The control's own choice still
          // wins — `rasterVisible` is what it wrote — so this restores the
          // user's state, not "visible".
          map.setLayoutProperty(
            layer.id,
            "visibility",
            rasterVisible ? "visible" : "none",
          );
        } else if (anchored) {
          // Re-asserted rather than asserted once. The raster is normally the
          // first result to arrive and starts on top of nothing; the model
          // layers are added afterwards, over it, and this is what sinks it
          // back under them the first time the anchor appears. `moveLayer` to
          // a position a layer already holds is a no-op.
          map.moveLayer(layer.id, BOTTOM_MODEL_LAYER_ID);
        }
      } catch (error) {
        console.error(`ResultLayers: could not add layer "${layer.id}"`, error);
        return;
      }
    }
  }, [map, raster, rasterVisible]);

  // No run, or a run whose receiver table never existed. The indicator list is
  // the table's, and the raster follows it band for band, so there is nothing
  // for the panel to offer without one — both targets write the two together.
  if (run === null || tableArtifact === undefined) return null;

  return (
    <MapPanel
      position="bottom-right"
      inset="bottom-24 right-3"
      width="w-48"
      translucent
      role="region"
      aria-label={m.label_result_levels()}
      className="space-y-2"
    >
      {/* The run's identity, and how far its numbers can be trusted. A
          scaffold run painted under the same legend as a normative one tells
          a viewer who did not start it nothing about which it is. */}
      <div className="flex items-center gap-1.5">
        <p className="min-w-0 flex-1 truncate font-mono text-2xs text-muted-foreground">
          {run.id}
        </p>
        <EvidenceTierBadge
          tier={summaryField(summary, "evidence_tier")}
          className="shrink-0"
        />
      </div>
      {substituted && requestedRunId !== null ? (
        <p role="status" className="text-2xs text-muted-foreground">
          {m.msg_result_run_unavailable({ runId: requestedRunId })}
        </p>
      ) : null}
      <PanelBody
        loadFailed={loadFailed}
        levelTable={levelTable}
        unit={unit}
        indicators={indicators}
        indicator={indicator}
        onSelect={setChosen}
        projected={projected}
        raster={raster}
      />
    </MapPanel>
  );
}

/**
 * What the panel says under the run id, which is not always a legend.
 *
 * Three mutually exclusive answers, in the order a reader needs them: the
 * results could not be fetched at all; they were fetched but are not levels;
 * or they are, and here is what the colours mean.
 */
function PanelBody({
  loadFailed,
  levelTable,
  unit,
  indicators,
  indicator,
  onSelect,
  projected,
  raster,
}: {
  loadFailed: boolean;
  levelTable: boolean;
  unit: string;
  indicators: string[];
  indicator: string;
  onSelect: (indicator: string) => void;
  projected: ProjectedPositions;
  raster: ResultRaster;
}) {
  if (loadFailed) {
    return (
      <p role="status" className="text-2xs text-destructive">
        {m.error_load_result_levels()}
      </p>
    );
  }

  if (!levelTable) {
    return (
      <p role="status" className="text-2xs text-muted-foreground">
        {m.msg_result_table_not_levels({ unit })}
      </p>
    );
  }

  return (
    <>
      <IndicatorPicker
        indicators={indicators}
        selected={indicator}
        onSelect={onSelect}
      />
      <ProjectionNotice projected={projected} />
      <RasterNotice raster={raster} />
      <Legend unit={unit} />
    </>
  );
}

/**
 * Which band of the receiver table is painted.
 *
 * Toggle buttons rather than a listbox, and `aria-pressed` rather than a
 * radio group: this is the same "one of a few, all visible" choice the draw
 * toolbar makes, and it answers it the same way. One indicator is no choice at
 * all, so the row is a label instead of a control nobody can move.
 */
function IndicatorPicker({
  indicators,
  selected,
  onSelect,
}: {
  indicators: string[];
  selected: string;
  onSelect: (indicator: string) => void;
}) {
  if (indicators.length === 0) return null;

  if (indicators.length === 1) {
    return (
      <p className="text-2xs font-medium">
        <span className="text-muted-foreground">
          {m.label_result_indicator()}
        </span>{" "}
        {selected}
      </p>
    );
  }

  return (
    <div
      role="group"
      aria-label={m.label_result_indicator()}
      className="flex flex-wrap gap-1"
    >
      {indicators.map((indicator) => (
        <Button
          key={indicator}
          variant={indicator === selected ? "default" : "ghost"}
          size="sm"
          className="h-6 px-1.5 text-2xs"
          aria-pressed={indicator === selected}
          onClick={() => {
            onSelect(indicator);
          }}
        >
          {indicator}
        </Button>
      ))}
    </div>
  );
}

/**
 * Why the levels are not on the map, when they are not.
 *
 * A panel that showed a legend over an empty canvas would be describing
 * colours that are nowhere, so each case that leaves the source empty says so
 * in its own words.
 */
function ProjectionNotice({ projected }: { projected: ProjectedPositions }) {
  if (projected.status === "projecting") {
    return (
      <p role="status" className="text-2xs text-muted-foreground">
        {m.status_projecting_result_levels()}
      </p>
    );
  }

  if (projected.status === "unsupported") {
    return (
      <p role="status" className="text-2xs text-muted-foreground">
        {m.msg_result_levels_unprojectable({ crs: projected.crs })}
      </p>
    );
  }

  if (projected.status === "unknown-crs") {
    return (
      <p role="status" className="text-2xs text-muted-foreground">
        {m.msg_result_levels_unknown_crs()}
      </p>
    );
  }

  if (projected.status === "failed") {
    return (
      <p role="status" className="text-2xs text-destructive">
        {m.msg_result_levels_failed()}
      </p>
    );
  }

  return null;
}

/**
 * Why there is no raster under the circles, when there is a reason worth one.
 *
 * Silent for the cases the notices around it already cover in the same words.
 * `unsupported` and `unknown-crs` are the projection's, and `ProjectionNotice`
 * says them about the receivers with the same cause and the same remedy; a
 * second sentence beside it would read as a second problem. `not-levels` is
 * the gate `PanelBody` already refused the whole panel on, so it cannot reach
 * here. What is left is the four a reader could not otherwise account for: a
 * run whose receivers were never a grid, a grid too large to draw, a band the
 * sidecar does not carry, and a failure while drawing it.
 */
function RasterNotice({ raster }: { raster: ResultRaster }) {
  switch (raster.status) {
    case "not-a-grid":
      return (
        <p role="status" className="text-2xs text-muted-foreground">
          {m.msg_result_raster_not_grid()}
        </p>
      );
    case "no-such-band":
      return (
        <p role="status" className="text-2xs text-muted-foreground">
          {m.msg_result_raster_no_band({ indicator: raster.indicator })}
        </p>
      );
    case "too-large":
      return (
        <p role="status" className="text-2xs text-muted-foreground">
          {m.msg_result_raster_too_large({
            width: raster.width,
            height: raster.height,
          })}
        </p>
      );
    case "failed":
      return (
        <p role="status" className="text-2xs text-destructive">
          {m.msg_result_raster_failed()}
        </p>
      );
    default:
      return null;
  }
}

/**
 * What the colours mean, straight off {@link NOISE_LEVEL_RAMP}.
 *
 * The labels are the ramp's own, so the legend cannot drift from the paint
 * expression beside it — `color-ramp.test.ts` pins the 5 dB step and the
 * open-ended ends those labels promise.
 */
function Legend({ unit }: { unit: string }) {
  return (
    <div>
      <p className="text-2xs font-medium text-muted-foreground">
        {m.label_result_legend()}
        {unit === "" ? "" : ` (${unit})`}
      </p>
      <ul className="mt-1 grid grid-cols-2 gap-x-2">
        {NOISE_LEVEL_RAMP.map((stop) => (
          <li key={stop.value} className="flex items-center gap-1">
            <span
              aria-hidden="true"
              style={{ backgroundColor: stop.color }}
              className="size-2.5 shrink-0 rounded-sm border"
            />
            <span className="text-2xs tabular-nums">{stop.label}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function isMapStyleReady(map: maplibregl.Map): boolean {
  try {
    const style = map.getStyle();
    return Boolean(style.sources);
  } catch {
    return false;
  }
}
