/**
 * A window drawn or moved in the elevation view, turned into the feature the
 * model stores.
 *
 * The key, the `from` / `to` it carries and the Point at its centre are
 * `facadeset.PlaceWindow`'s — the placer `aconiq import --windows` uses —
 * through the WASM kernel in browser mode and `POST /api/v1/facades/window`
 * in API mode. The browser keys no window and measures no frame of its own
 * (P18 C2).
 */

import type { Backend } from "@/api/backend";
import {
  projectWorkspace,
  resolveComputeModel,
  type Workspace,
} from "@/model/compute-crs";
import { featuresToGeoJSON } from "@/model/to-geojson";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import type { WindowPlacement } from "@/wasm/types";

/**
 * Places `placement` and returns the window feature in the workspace's CRS.
 *
 * It asks in the compute CRS the elevation was measured in — the same
 * projection, from the same cache — so `part` / `ring` / `edge` name the
 * edge the view showed; and moves the answer back, Point and facade `from` /
 * `to` together, as a save through another CRS moves them.
 */
export async function placeWindowFeature(
  backend: Pick<Backend, "placeWindow" | "transformCoordinates">,
  workspace: Workspace,
  placement: WindowPlacement,
): Promise<ModelFeature> {
  const transform = (req: Parameters<Backend["transformCoordinates"]>[0]) =>
    backend.transformCoordinates(req);
  const { features, projection } = await resolveComputeModel(
    { transform },
    workspace,
  );

  const { feature } = await backend.placeWindow({
    crs: projection.computeCRS,
    model: featuresToGeoJSON(
      features.filter(
        (f) => f.kind === "building" && f.id === placement.building_ref,
      ),
    ),
    window: placement,
  });

  // `id` and `kind` are the feature's own fields in the store, not properties.
  const properties = { ...feature.properties };
  delete properties.id;
  delete properties.kind;
  const placed: ModelFeature = {
    id: placement.id,
    kind: "window",
    properties,
    geometry: feature.geometry,
  };

  if (!projection.applied) return placed;

  const back = await projectWorkspace(
    transform,
    {
      features: [placed],
      receivers: [],
      calcArea: null,
      crs: projection.computeCRS,
    },
    workspace.crs,
  );
  return back.features[0] ?? placed;
}

/**
 * The first `win-<n>` nothing in the model carries. Receivers count too:
 * `validateProjectModel` checks feature and receiver ids as one namespace, so
 * a window named after a receiver would be a `feature.id.duplicate`.
 */
export function nextWindowId({
  features,
  receivers,
}: {
  features: readonly ModelFeature[];
  receivers: readonly ModelReceiver[];
}): string {
  const taken = new Set([
    ...features.map((f) => f.id),
    ...receivers.map((r) => r.id),
  ]);
  let n = 1;
  while (taken.has(`win-${String(n)}`)) n += 1;
  return `win-${String(n)}`;
}
