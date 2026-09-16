/**
 * The browser's copy of what `aconiq run` does before it reads a coordinate.
 *
 * Every standards module measures distance with `geo.Distance`, which is
 * `math.Hypot` over the coordinates it is handed. That is correct for a metric
 * projected CRS and silently wrong for a geographic one: in degrees a whole
 * city fits inside 0.05 units, every propagation distance falls under the
 * modules' minimum-distance clamp, and each receiver reports the source's
 * emission level verbatim. Browser mode defaults to EPSG:4326, so that was the
 * out-of-the-box path — tens of dB wrong, with no sign that anything was.
 *
 * The projection itself is the Go kernel's (`aconiq.transform`), not a second
 * transverse-Mercator implementation in TypeScript. The zone decision is the Go
 * kernel's too, so the browser cannot land a model in a different CRS than the
 * CLI would, or accept a site the CLI refuses.
 *
 * It runs once, over the whole workspace, before any scene is built — never per
 * builder. `buildParkingSources` computes a shoelace area in m² and a centroid,
 * and `getFeatureBBox`/`getPolygonBBox` feed a receiver grid whose padding and
 * resolution are metres by contract. All of them need the model already in
 * metres. This mirrors `cli.resolveComputeModel`, which projects the model and
 * then lets extraction run unchanged.
 */

import type { AconiqKernel } from "@/wasm/kernel";
import { AUTO_TARGET_CRS } from "@/wasm/types";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import type { CalcArea, ModelFeature, ModelReceiver } from "./types";

/**
 * A batch coordinate projection — `aconiq.transform`, or whatever stands in
 * for it.
 *
 * A bare function rather than the kernel object, because the second caller is
 * the map, which reaches the same kernel through `backend.transformCoordinates`
 * and has no kernel handle of its own. There is still exactly one projection
 * behind both: the Go one.
 */
export type CoordinateTransform = (
  req: TransformRequest,
) => Promise<TransformResponse>;

/** Which CRS a run computed in, and whether getting there moved the model. */
export interface ComputeProjection {
  /** The CRS the workspace's stored coordinates are in. */
  projectCRS: string;
  /** The CRS the model was computed in. Results are expressed in it. */
  computeCRS: string;
  /** False when the model was already metric and nothing moved. */
  applied: boolean;
}

/** The workspace a run reads, as it is held in the store. */
export interface Workspace {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  crs: string;
}

/**
 * The same workspace, in the CRS the run computes in.
 *
 * The element types are the store's own: a nominal brand would make feeding a
 * builder the store's raw array a compile error, but every builder is also
 * called directly by its own unit tests with plain fixtures, and branding would
 * turn each of those call sites into a cast — which announces the brand without
 * checking anything. What holds the invariant instead is that `startRun` reads
 * the store exactly once, through this function, and that the `road_geographic`
 * parity fixture fails by tens of dB if it ever stops doing so.
 */
export interface ComputeModel {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  projection: ComputeProjection;
}

/**
 * Properties that carry coordinates of their own rather than in
 * `geometry.coordinates`.
 *
 * `backend/internal/geo/modelgeojson/reproject.go` moves these alongside the
 * geometry, because two standards vocabularies attach geometry to a feature
 * through its properties: RLS-19 directional sources carry their own
 * centerline, and Schall 03 track features carry a point each.
 *
 * Browser mode cannot move them — the transform takes a flat coordinate batch,
 * and these are nested inside arbitrary property objects — so a model carrying
 * one is refused rather than projected around. Skipping them silently would
 * leave geometry in degrees inside a model that is otherwise in metres, which
 * places a directional source millions of metres from its own receivers.
 */
const PROPERTY_GEOMETRIES = [
  "rls19_directional_sources",
  "schall03_track_features",
] as const;

/**
 * Projects the workspace into the CRS a run computes in.
 *
 * A workspace already in a projected CRS comes back untouched, reporting
 * `applied: false` — the same early return `cli.resolveComputeModel` takes, so
 * a project in EPSG:25832 runs through exactly the code it ran through before.
 */
export async function resolveComputeModel(
  kernel: Pick<AconiqKernel, "transform">,
  workspace: Workspace,
): Promise<ComputeModel> {
  refuseUnreachablePropertyGeometry(workspace.features);

  // `async` rather than a plain promise-returning function: it is what turns
  // the refusal above into a rejection instead of a synchronous throw, which
  // every caller and every test here expects.
  const projected = await projectWorkspace(
    (req) => kernel.transform(req),
    workspace,
    AUTO_TARGET_CRS,
  );
  return projected;
}

/**
 * Moves a whole workspace into `targetCRS`, one batch, one traversal each way.
 *
 * The body `resolveComputeModel` used to be, with the target lifted out of it.
 * Two callers want the same traversal for different reasons: a run projects
 * into the CRS it computes in, and the map projects into the CRS it draws in.
 * Writing the second traversal separately is how the two would come to disagree
 * about the order they visit coordinates in — and a scatter that disagrees with
 * its collect silently moves a model to somewhere plausible.
 *
 * The property-geometry refusal deliberately does *not* live here. It is about
 * computing: a model carrying `rls19_directional_sources` cannot be projected
 * for a run, but its geometry draws fine, and refusing here would blank the map
 * for a model that has nothing wrong with what the map shows.
 */
export async function projectWorkspace(
  transform: CoordinateTransform,
  workspace: Workspace,
  targetCRS: string,
): Promise<ComputeModel> {
  const collected: number[] = [];
  walkWorkspace(workspace, (x, y) => {
    collected.push(x, y);
    return null;
  });

  const response = await transform({
    source_crs: workspace.crs,
    target_crs: targetCRS,
    coordinates: collected,
  });

  const projection: ComputeProjection = {
    projectCRS: workspace.crs,
    computeCRS: response.target_crs,
    applied: response.applied,
  };

  if (!response.applied) {
    return {
      features: workspace.features,
      receivers: workspace.receivers,
      calcArea: workspace.calcArea,
      projection,
    };
  }

  if (response.coordinates.length !== collected.length) {
    throw new Error(
      `The compute projection returned ${String(response.coordinates.length)} values for ${String(collected.length)} sent; the model was not projected.`,
    );
  }

  // The same traversal a second time, consuming from an index where the first
  // pass appended. Running one traversal twice rather than two traversals once
  // each is what makes it impossible for the collect and the scatter to
  // disagree about the order they visit coordinates in.
  let next = 0;
  const moved = walkWorkspace(workspace, () => {
    const x = response.coordinates[next] ?? Number.NaN;
    const y = response.coordinates[next + 1] ?? Number.NaN;
    next += 2;
    return [x, y];
  });

  return { ...moved, projection };
}

/**
 * Visits every coordinate of the workspace in a fixed order — features in array
 * order, then receivers, then the calculation area's rings — and rebuilds it
 * from what `visit` returns.
 *
 * `visit` returning `null` means "leave this coordinate alone", which is what
 * the collecting pass returns for every one of them; it then discards the
 * rebuilt workspace, which is the price of reading and writing through one
 * traversal instead of two that could disagree. The depth-first descent through
 * the coordinate tree mirrors `walkCoordinates` and `transformCoordinates` in
 * `geo/modelgeojson/reproject.go`.
 */
function walkWorkspace(
  workspace: Workspace,
  visit: (x: number, y: number) => [number, number] | null,
): Omit<ComputeModel, "projection"> {
  return {
    features: workspace.features.map((feature) => ({
      ...feature,
      geometry: {
        ...feature.geometry,
        coordinates: mapCoordinates(
          feature.geometry.coordinates,
          visit,
        ) as ModelFeature["geometry"]["coordinates"],
      },
    })),
    receivers: workspace.receivers.map((receiver) => ({
      ...receiver,
      geometry: {
        ...receiver.geometry,
        coordinates: mapCoordinates(
          receiver.geometry.coordinates,
          visit,
        ) as ModelReceiver["geometry"]["coordinates"],
      },
    })),
    calcArea:
      workspace.calcArea === null
        ? null
        : {
            ...workspace.calcArea,
            geometry: {
              ...workspace.calcArea.geometry,
              coordinates: mapCoordinates(
                workspace.calcArea.geometry.coordinates,
                visit,
              ) as CalcArea["geometry"]["coordinates"],
            },
          },
  };
}

/**
 * Rebuilds a GeoJSON coordinate tree with every `[x, y]` pair replaced by what
 * `visit` returns for it. A third ordinate is an absolute elevation in metres
 * and is carried through untouched, exactly as the Go side does.
 */
function mapCoordinates(
  coords: unknown,
  visit: (x: number, y: number) => [number, number] | null,
): unknown {
  if (!Array.isArray(coords)) return coords;

  const values: unknown[] = coords;
  const [first, second] = values;

  if (typeof first === "number" && typeof second === "number") {
    const replacement = visit(first, second);
    if (replacement === null) return coords;
    return [replacement[0], replacement[1], ...values.slice(2)];
  }

  return values.map((element) => mapCoordinates(element, visit));
}

function refuseUnreachablePropertyGeometry(features: ModelFeature[]): void {
  for (const feature of features) {
    const properties = feature.properties;
    if (properties === undefined) continue;

    for (const key of PROPERTY_GEOMETRIES) {
      if (!(key in properties)) continue;

      throw new Error(
        `Feature "${feature.id}" carries "${key}", whose coordinates browser mode cannot project. ` +
          `Run this model through \`aconiq run\`, or set the project CRS to the metric CRS the model is already in.`,
      );
    }
  }
}
