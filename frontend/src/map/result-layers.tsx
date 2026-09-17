/**
 * The computed levels of the latest completed run, drawn on the workspace map.
 *
 * It draws the **receiver table** and nothing else, which is the only result
 * container both modes produce in the same shape. The raster half of this is
 * not merely unwritten — it is unreachable: browser mode stores the run's
 * SHA-256 where the raster binary belongs and `StoredArtifactContent.encoding`
 * has no case for binary at all, the GeoTIFF/COG/contour exports get no
 * `ArtifactRef` so no URL reaches them, and `results.RasterMetadata` carries no
 * geotransform to place a raster with. See PLAN.md, "Results on the map".
 *
 * The coordinates are projected through `backend.transformCoordinates` — the
 * kernel's own projection, the same one `display-model.ts` draws the model
 * with — so a run and the model it came from land in the same place. The CRS
 * they start in is read off the run summary, which is where both targets
 * record it: provenance is not an artifact, so the API never serves it.
 */

import { useEffect, useMemo, useRef, useState } from "react";
import type maplibregl from "maplibre-gl";
import { backend } from "@/api/backend";
import type { ReceiverRecord, RunSummary } from "@/api/client";
import { useArtifactContent, useReceiverTable, useRuns } from "@/api/hooks";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";
import { NOISE_LEVEL_RAMP } from "./color-ramp";
import { DISPLAY_CRS } from "./display-model";
import {
  RESULT_LEVEL_PROPERTY,
  RESULT_RECEIVER_LAYERS,
  RESULT_RECEIVERS_GROUP_ID,
  SOURCE_IDS,
} from "./layers";
import { MapPanel } from "./map-panel";
import { useMapStore } from "./map-store";
import { useMap } from "./use-map";

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
  | { status: "failed" }
  /** Parallel to the records they were built from, index for index. */
  | { status: "ready"; positions: GeoJSON.Position[] };

const IDLE: ProjectedPositions = { status: "idle" };

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

/**
 * The CRS the run's receiver coordinates are in, read off its own run summary.
 *
 * `null` for a run written before either target recorded it. That is not the
 * same as "WGS84": the coordinates would be metres in some unnamed projection,
 * and drawing them as degrees is the bug this whole file is careful about.
 */
function computeCRSOf(summary: unknown): string | null {
  if (typeof summary !== "object" || summary === null) return null;

  const value = (summary as Record<string, unknown>)["compute_crs"];
  return typeof value === "string" && value !== "" ? value : null;
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
): ProjectedPositions {
  const canReproject = backend.capabilities.canReprojectForDisplay;
  const [state, setState] = useState<ProjectedPositions>(IDLE);

  // Monotonic, and compared on arrival: a run switching under a slow transform
  // must not be overwritten by the answer to the question before it, which
  // would draw the previous run's receivers in this one's place.
  const requestRef = useRef(0);

  useEffect(() => {
    if (records.length === 0 || computeCRS === null) {
      requestRef.current += 1;
      setState(IDLE);
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
  }, [records, computeCRS, canReproject]);

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
 * Draws the latest completed run's receiver levels, and offers the picker and
 * the legend that make them readable.
 *
 * Must be rendered as a child of MapView (inside MapContext).
 */
export function ResultLayers() {
  const map = useMap();
  const { data: runs } = useRuns();

  const run = useMemo(() => latestCompletedRun(runs ?? []), [runs]);

  const tableArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.receiver_table_json",
  );
  const summaryArtifact = run?.artifacts.find(
    (artifact) => artifact.kind === "run.result.summary",
  );

  const { data: table } = useReceiverTable(tableArtifact?.id ?? null);
  const { data: summary } = useArtifactContent<unknown>(
    summaryArtifact?.id ?? null,
  );

  const records = table?.records ?? NO_RECORDS;
  const indicators = table?.indicator_order ?? NO_INDICATORS;
  const unit = table?.unit ?? "";
  const computeCRS = computeCRSOf(summary);

  // The band on screen. `null` means "whatever the table lists first", which
  // is resolved on read rather than written into state: seeding it from an
  // effect would paint one commit with no layer at all, and a run switched
  // underneath would keep a band the new table may not have.
  const [chosen, setChosen] = useState<string | null>(null);
  const indicator =
    chosen !== null && indicators.includes(chosen)
      ? chosen
      : (indicators[0] ?? "");

  const projected = useProjectedPositions(records, computeCRS);

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
      <p className="truncate font-mono text-2xs text-muted-foreground">
        {run.id}
      </p>
      <IndicatorPicker
        indicators={indicators}
        selected={indicator}
        onSelect={setChosen}
      />
      <ProjectionNotice projected={projected} />
      <Legend unit={unit} />
    </MapPanel>
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
