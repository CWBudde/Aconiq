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
 * Four things are load-bearing.
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
 * **One gesture is one undo step, and `finish` is what ends a gesture.** Every
 * commit carries `geometry:<id>`, and `CommandStack` merges a run of same-key
 * commands keeping the newest `execute` and the oldest `undo` — see its
 * `execute` for why that asymmetry is the only correct one. `sealHistory()`
 * closes the run. The boundary is terra-draw's `finish`, which it fires from
 * its drag-end path and which leaves the feature selected: waiting for
 * `deselect` instead would seal once for a whole run of drags, and over a
 * metric model would leave every drag but the last unwritten.
 *
 * **The model can move under an armed session, and terra-draw would not know.**
 * Its copy of the feature is its own; an undo, a redo or any other writer
 * changes the store without touching it, and the next `change` would commit the
 * stale copy straight back over the edit that just landed. So each session
 * remembers the geometry object the store held after its last write, and a
 * store geometry that is no longer that object re-arms the session on the
 * model's version. Reference identity is exact here rather than approximate:
 * the remembered object is read back out of the store, and undo restores the
 * distinct object it captured.
 */

import { useCallback, useEffect, useRef, useState } from "react";
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
  /**
   * The store's geometry object as of this session's last write, by reference.
   *
   * A store geometry that is not this object came from somewhere else — an
   * undo, a redo, another writer — and terra-draw's copy is stale against it.
   */
  stored: Geometry | null;
  /** Set once the gesture has ended, so a later teardown is a no-op. */
  ended: boolean;
}

/** The store's own geometry for a feature or receiver, by reference. */
function storedGeometry(id: string, receiver: boolean): Geometry | null {
  const state = useModelStore.getState();
  const geometry = receiver
    ? state.receivers.find((r) => r.id === id)?.geometry
    : state.features.find((f) => f.id === id)?.geometry;
  return geometry ?? null;
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
    instanceEpoch,
    addFeatures,
    selectFeature,
    removeFeatures,
    getFeature,
    subscribeSelection,
  } = useDrawContext();

  const crs = useModelStore((s) => s.crs);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
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
      } else {
        updateFeatureGeometry(session.id, geometry);
      }

      // Read back rather than remember what was handed over:
      // `updateReceiverGeometry` builds a geometry of its own from the
      // position, so the object the store holds is not the object passed in.
      // Recorded against whichever session is current, which after a `finish`
      // is no longer the one that started this commit.
      const current = sessionRef.current;
      if (current !== null && current.id === session.id) {
        current.stored = storedGeometry(session.id, session.receiver);
      }
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
   *
   * {@link useInverseProjection} is a newest-wins channel: a second projection
   * started while one is in flight supersedes it, and the first answer is
   * dropped. That is the right behaviour for this caller rather than a hazard
   * to guard against — terra-draw's geometry is cumulative, so back-to-back
   * drags of the same feature hand over shapes of which the later one already
   * contains the earlier. What is lost to a supersession is a round trip, not
   * an edit.
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
      onFinish: (id) => {
        const session = sessionRef.current;
        if (!session || session.ended || session.id !== id) return;
        endSession();
        // The gesture ended; the selection did not. Terra-draw keeps the
        // feature selected and in its store, so the next drag has nowhere to
        // land unless a session is opened for it — and it has to be a *new*
        // one, or `CommandStack` would read the drag just sealed and the one
        // about to start as a single step.
        sessionRef.current = {
          id: session.id,
          receiver: session.receiver,
          synchronous: session.synchronous,
          pending: null,
          stored: session.stored,
          ended: false,
        };
      },
      onDeselect: (id) => {
        if (sessionRef.current?.id !== id) return;
        endSession();
        removeFeatures([id]);
      },
    });
  }, [subscribeSelection, getFeature, commit, endSession, removeFeatures]);

  // Re-arming on a model edit the session did not make. Bumped rather than
  // acted on directly: re-arming *is* the arming effect, and duplicating its
  // add/select here would be a second place for terra-draw's copy to come from.
  const [resyncEpoch, setResyncEpoch] = useState(0);
  useEffect(() => {
    const session = sessionRef.current;
    if (!session || session.ended) return;
    const current = storedGeometry(session.id, session.receiver);
    // Gone from the model entirely: the page clears the selection for that, and
    // re-arming on nothing would only thrash.
    if (current === null || current === session.stored) return;
    // Anything still held is measured against a shape the model no longer has,
    // so it is dropped rather than landed: committing it on the way out would
    // undo the undo that got us here.
    session.pending = null;
    setResyncEpoch((epoch) => epoch + 1);
    // The store's arrays, so this runs on every model edit and on nothing else.
    // The comparison above is what separates this session's own writes — which
    // are most of them, since the synchronous path commits per pointer move —
    // from the edits that make terra-draw's copy stale.
  }, [features, receivers]);

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

    const receiverSession = feature === undefined;
    sessionRef.current = {
      id: featureId,
      receiver: receiverSession,
      synchronous: crs === DISPLAY_CRS,
      pending: null,
      // The store's geometry, not the display one handed to terra-draw above:
      // it is the baseline a later external edit is measured against, and over
      // a metric model the two are different coordinates for the same shape.
      stored: storedGeometry(featureId, receiverSession),
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
    // `instanceEpoch` is the terra-draw instance itself being rebuilt — a
    // basemap switch replaces the map and with it the store this feature was
    // put into, and nothing else in the dependency list moves when it does.
    // `resyncEpoch` is the model having been changed by someone else.
  }, [
    armed,
    featureId,
    selectionEpoch,
    crs,
    instanceEpoch,
    resyncEpoch,
    addFeatures,
    selectFeature,
    removeFeatures,
    endSession,
  ]);
}
