/**
 * Reshaping a feature that is already in the model.
 *
 * `useDraw` has always built `TerraDrawSelectMode` with draggable features and
 * draggable, deletable midpoints — it was simply never fed anything: nothing in
 * the app called `draw.addFeatures`, so select mode was armed over an empty
 * terra-draw store and clicking a feature did nothing. This hook is the feed.
 * While select mode is armed and the workspace has a feature selected, that
 * feature's geometry is handed to terra-draw, selected, dragged by the user, and
 * read back into the model.
 *
 * Three things are load-bearing.
 *
 * **The geometry comes from the display model, not from the store.** Terra-draw
 * draws on MapLibre and therefore in WGS84; the store may hold metres. The
 * page's one `useDisplayModel()` result is the model already projected for the
 * map, so taking the shape from there is what makes the handle the user grabs
 * land on the feature they can see. The way back is
 * {@link useInverseProjection}, the same transform `use-draw-projection.ts`
 * uses, so the map still has exactly one path by which a coordinate reaches the
 * model.
 *
 * **There are two commit triggers, and which one applies is decided by the
 * store's CRS.** Over a model already in `DISPLAY_CRS` the inverse is the
 * identity, so every `change` commits straight away — free, and the map redraws
 * from the model as the drag happens. Over a metric model each commit would be
 * a `POST /api/v1/transform`, one per pointer move, so the latest display
 * geometry is held and projected **once**, when the gesture ends. The
 * asymmetry is deliberate: the cheap path gets live feedback, the expensive one
 * gets a single round trip, and both produce exactly one undo step.
 *
 * **One gesture is one undo step.** Every commit carries `geometry:<id>`, and
 * `CommandStack` merges a run of same-key commands keeping the newest `execute`
 * and the oldest `undo` — see its `execute` for why that asymmetry is the only
 * correct one. `sealHistory()` closes the run when the gesture ends, so a second
 * drag of the same feature is a second step rather than a continuation of the
 * first.
 */

import { useCallback, useEffect, useRef } from "react";
import { useModelStore } from "@/model/model-store";
import type { Geometry, Position } from "@/model/types";
import { DISPLAY_CRS } from "./display-model";
import type { DisplayModel } from "./display-model";
import { useDrawContext } from "./use-draw-context";
import { useInverseProjection } from "./use-inverse-projection";

/**
 * The geometry types terra-draw can edit.
 *
 * It models a point, a linestring and a polygon and nothing else, so a building
 * imported as a `MultiPolygon` — which `docs/geojson-schema-v1.md` allows — has
 * no editable form here. Refusing to arm on one is the honest answer: handing
 * terra-draw only the first ring would silently drop the rest of the feature on
 * the first commit.
 */
const EDITABLE: Record<string, string> = {
  Point: "point",
  LineString: "linestring",
  Polygon: "polygon",
};

/** What the hook is currently editing, and what it has not committed yet. */
interface EditSession {
  id: string;
  /** Receivers are a separate store array with a separate command. */
  receiver: boolean;
  /** True where the store is in `DISPLAY_CRS` and a commit costs no transform. */
  synchronous: boolean;
  /** The latest display-CRS geometry, held for the single deferred commit. */
  pending: Geometry | null;
  /** Set once the gesture has ended, so a later teardown is a no-op. */
  ended: boolean;
}

export interface GeometryEditOptions {
  /** The page's one `useDisplayModel()` result — the model as the map draws it. */
  display: DisplayModel;
  /** The feature the workspace has selected, or null. */
  featureId: string | null;
  /**
   * Bumped by the page on every selection, including a repeat of the id already
   * selected.
   *
   * Without it a feature deselected on the map could not be picked up again: the
   * terra-draw copy is removed on deselect, so clicking the feature reaches only
   * MapLibre, which sets the same id the page already holds and changes no
   * dependency here.
   */
  selectionEpoch: number;
  /**
   * The page's `drawingDisabled`. A model the map cannot project cannot be
   * reshaped either — the same inverse transform that would refuse a drawn shape
   * would refuse this one, only after the user had already moved it.
   */
  disabled: boolean;
}

export function useGeometryEdit({
  display,
  featureId,
  selectionEpoch,
  disabled,
}: GeometryEditOptions): void {
  const {
    activeMode,
    addFeatures,
    selectFeature,
    removeFeatures,
    getFeature,
    subscribeSelection,
  } = useDrawContext();

  const crs = useModelStore((s) => s.crs);
  const updateFeatureGeometry = useModelStore((s) => s.updateFeatureGeometry);
  const updateReceiverGeometry = useModelStore((s) => s.updateReceiverGeometry);
  const sealHistory = useModelStore((s) => s.sealHistory);
  const { project } = useInverseProjection();

  const sessionRef = useRef<EditSession | null>(null);

  // `display` and `project` are read through refs so that neither a model edit
  // nor a CRS-keyed callback identity re-runs the arming effect. Re-running it
  // mid-drag would remove the feature from under the user's pointer and add it
  // back — and on the synchronous path every pointer move *is* a model edit.
  const displayRef = useRef(display);
  displayRef.current = display;
  const projectRef = useRef(project);
  projectRef.current = project;

  const commit = useCallback(
    (session: EditSession, geometry: Geometry) => {
      if (session.receiver) {
        if (geometry.type !== "Point") return;
        updateReceiverGeometry(session.id, geometry.coordinates as Position);
        return;
      }
      updateFeatureGeometry(session.id, geometry);
    },
    [updateFeatureGeometry, updateReceiverGeometry],
  );

  /**
   * Ends the gesture: lands whatever is still held, then closes the undo run.
   *
   * The deferred commit is projected here rather than at `change` time, which
   * is the whole point of holding it — one transform per drag instead of one
   * per pointer move. A projection that fails drops the reshape, and what the
   * user sees is the feature snapping back to the geometry the model still
   * holds. That is a visible outcome rather than a silent one, which is why
   * there is no notice here: unlike a *drawn* shape, which exists nowhere else
   * and would simply vanish, the feature is still on the map.
   */
  const endSession = useCallback(() => {
    const session = sessionRef.current;
    if (!session || session.ended) return;
    session.ended = true;
    sessionRef.current = null;

    const pending = session.pending;
    if (pending !== null) {
      projectRef.current(pending, (projected) => {
        commit(session, projected);
        sealHistory();
      });
      return;
    }
    sealHistory();
  }, [commit, sealHistory]);

  // Terra-draw's own events. Subscribed once, and filtered against the session:
  // `change` also fires for the selection points and midpoints terra-draw adds
  // around the feature, which carry ids of their own.
  useEffect(() => {
    return subscribeSelection({
      onChange: (ids) => {
        const session = sessionRef.current;
        if (!session || session.ended) return;
        if (!ids.includes(session.id)) return;

        const feature = getFeature(session.id);
        if (!feature) return;
        const geometry = feature.geometry;
        // Terra-draw models points, lines and polygons and never a
        // GeometryCollection — the one GeoJSON geometry with no `coordinates`.
        if (!("coordinates" in geometry)) return;

        if (session.synchronous) {
          commit(session, geometry as Geometry);
          return;
        }
        session.pending = geometry as Geometry;
      },
      onDeselect: (id) => {
        if (sessionRef.current?.id !== id) return;
        endSession();
        removeFeatures([id]);
      },
    });
  }, [subscribeSelection, getFeature, commit, endSession, removeFeatures]);

  const armed =
    activeMode === "select" &&
    !disabled &&
    featureId !== null &&
    display.status === "ready";

  useEffect(() => {
    // `armed` carries `featureId !== null`, and the compiler follows the alias
    // — so `featureId` is a string from here down without a second check.
    if (!armed) return;

    const model = displayRef.current;
    if (model.status !== "ready") return;

    const feature = model.features.find((f) => f.id === featureId);
    const receiver = feature
      ? undefined
      : model.receivers.find((r) => r.id === featureId);
    const geometry: Geometry | undefined =
      feature?.geometry ?? receiver?.geometry;
    if (!geometry) return;

    const mode = EDITABLE[geometry.type];
    if (mode === undefined) return;

    const accepted = addFeatures([
      {
        type: "Feature",
        id: featureId,
        // Terra-draw keys a feature to the mode that owns it, and rejects one
        // whose `mode` names a mode the instance does not hold.
        properties: { mode },
        geometry: geometry as GeoJSON.Geometry,
      },
    ]);
    if (!accepted) return;

    sessionRef.current = {
      id: featureId,
      receiver: feature === undefined,
      synchronous: crs === DISPLAY_CRS,
      pending: null,
      ended: false,
    };
    selectFeature(featureId);

    return () => {
      endSession();
      removeFeatures([featureId]);
    };
    // `selectionEpoch` is a dependency on purpose: re-selecting the feature the
    // page already holds has to re-arm, and nothing else about it has changed.
    // `crs` re-arms too, because it decides which commit trigger applies.
  }, [
    armed,
    featureId,
    selectionEpoch,
    crs,
    addFeatures,
    selectFeature,
    removeFeatures,
    endSession,
  ]);
}
