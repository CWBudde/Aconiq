/**
 * The elevation of one building, measured by the Go code.
 *
 * The facades, their lengths, the bearing each faces and the storeys' slab
 * heights all come from `facadeset.Elevation` — the WASM kernel in browser
 * mode, `POST /api/v1/facades/elevation` in API mode — for the reason
 * `transform` is the frontend's only projection: a second facade frame here
 * would number edges, or measure a window, differently from the run that
 * places points at them (P18 C2).
 */

import type { Backend } from "@/api/backend";
import { resolveComputeModel, type Workspace } from "@/model/compute-crs";
import { getFeatureBBox, type GridExtent } from "@/model/grid-estimate";
import { featuresToGeoJSON } from "@/model/to-geojson";
import type { ModelFeature } from "@/model/types";
import type { FacadeElevationResponse } from "@/wasm/types";

export interface LoadedElevation {
  elevation: FacadeElevationResponse;
  /** The CRS the facades' `from` / `to` are in: the run's compute CRS. */
  computeCRS: string;
}

/**
 * Unfolds `buildingId` as a facade run would see it.
 *
 * The workspace is projected into the compute CRS first, exactly as a run
 * projects it — the same call, and its cache — so the facades are the
 * canonical edges a run's keys are numbered on, and a geographic workspace is
 * measured in metres. Only the building, its windows and the buildings near
 * it cross the boundary: nothing else bears on its elevation, and the
 * neighbours decide its party walls.
 *
 * "Its windows" are those whose key names `buildingId`. A window whose key
 * names a building the model does not have (`window.facade.unresolved`) is
 * in no building's elevation, so it is not sent: nothing says which building
 * it meant, and the validation report lists it.
 */
export async function loadFacadeElevation(
  backend: Pick<Backend, "facadeElevation" | "transformCoordinates">,
  workspace: Workspace,
  buildingId: string,
): Promise<LoadedElevation> {
  const { features, projection } = await resolveComputeModel(
    { transform: (req) => backend.transformCoordinates(req) },
    workspace,
  );

  const building = features.find(
    (feature) => feature.kind === "building" && feature.id === buildingId,
  );
  const reach =
    building === undefined ? null : grown(getFeatureBBox([building]));

  const elevation = await backend.facadeElevation({
    crs: projection.computeCRS,
    model: featuresToGeoJSON(
      features.filter((feature) =>
        feature.kind === "building"
          ? feature.id === buildingId || touches(feature, reach)
          : feature.kind === "window" &&
            windowBuildingRef(feature) === buildingId,
      ),
    ),
    building: buildingId,
  });

  return { elevation, computeCRS: projection.computeCRS };
}

/**
 * How far around the building a neighbour is sent, in metres of the compute
 * CRS. Facade mode probes a facade 0.1 m out, so only a footprint reaching
 * within 0.1 m of the building can make a party wall: anything this box
 * leaves out cannot change the answer, and the margin is ten times that.
 */
const NEIGHBOUR_MARGIN_M = 1;

function grown(box: GridExtent | null): GridExtent | null {
  if (box === null) return null;
  return {
    minX: box.minX - NEIGHBOUR_MARGIN_M,
    minY: box.minY - NEIGHBOUR_MARGIN_M,
    maxX: box.maxX + NEIGHBOUR_MARGIN_M,
    maxY: box.maxY + NEIGHBOUR_MARGIN_M,
  };
}

/** Whether a footprint's box overlaps `reach`. */
function touches(feature: ModelFeature, reach: GridExtent | null): boolean {
  const box = reach === null ? null : getFeatureBBox([feature]);
  return (
    reach !== null &&
    box !== null &&
    box.minX <= reach.maxX &&
    box.maxX >= reach.minX &&
    box.minY <= reach.maxY &&
    box.maxY >= reach.minY
  );
}

/** The building a window's facade key names, or null when it names none. */
export function windowBuildingRef(feature: ModelFeature): string | null {
  const facade = feature.properties?.facade;
  if (typeof facade !== "object" || facade === null) return null;
  const ref = (facade as Record<string, unknown>).building_ref;
  return typeof ref === "string" ? ref : null;
}
