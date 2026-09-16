import { create } from "zustand";
import type {
  CalcArea,
  FeatureKind,
  ModelFeature,
  ModelReceiver,
} from "./types";
import { CommandStack } from "./command-stack";

/**
 * The CRS a workspace is in when nothing says otherwise. `aconiq init` defaults
 * a project to the same one, and the map draws in it.
 */
export const DEFAULT_MODEL_CRS = "EPSG:4326";

interface ModelState {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  /**
   * The CRS the stored coordinates are in.
   *
   * It is state rather than the constant it used to be because it decides what
   * a run does with the model: a geographic CRS has to be projected into a
   * metric one before any distance is measured, and a metric one must be left
   * alone. A hardcoded EPSG:4326 got that wrong in both directions — it would
   * project a model already in metres, and it could not tell a lon/lat pair
   * from an easting/northing pair that happens to look like one.
   */
  crs: string;
  dirty: boolean;
  canUndo: boolean;
  canRedo: boolean;

  /** Declares which CRS the stored coordinates are in. It moves nothing. */
  setCRS: (crs: string) => void;

  addFeature: (feature: ModelFeature) => void;
  updateFeature: (feature: ModelFeature) => void;
  removeFeature: (id: string) => void;
  reset: () => void;
  markClean: () => void;

  undo: () => void;
  redo: () => void;

  getFeatureById: (id: string) => ModelFeature | undefined;
  featuresByKind: (kind: FeatureKind) => ModelFeature[];

  addReceiver: (receiver: ModelReceiver) => void;
  updateReceiver: (receiver: ModelReceiver) => void;
  removeReceiver: (id: string) => void;
  loadModel: (model: LoadedModel) => void;
  mergeModel: (model: LoadedModel) => MergeSkips;
  hydrateModel: (model: LoadedModel) => void;
  getReceiverById: (id: string) => ModelReceiver | undefined;

  setCalcArea: (area: CalcArea) => void;
  clearCalcArea: () => void;
}

/** A complete model (draft restore, import) — replaced in, or merged in. */
export interface LoadedModel {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  /**
   * The CRS the carried coordinates are in. Optional: a draft written before
   * the store held a CRS carries none, and so does a caller that has nothing to
   * say about it. Absent means {@link DEFAULT_MODEL_CRS} on a replacement, and
   * means "keep what the workspace has" on a merge.
   */
  crs?: string;
}

/**
 * How many objects a model holds: features, receivers, and the calculation
 * area, which counts as one.
 *
 * The area is an object the import carries and installs like any other — a file
 * holding nothing else still replaces the workspace when Replace is confirmed —
 * so leaving it out made the import wizard offer "Import 0 objects" for exactly
 * that file, and under-report every mixed file by one. One function because the
 * preview's button and the page's confirmation and done step must not disagree
 * about what "this many" means.
 */
export function countModelObjects({
  features,
  receivers,
  calcArea,
}: LoadedModel): number {
  return features.length + receivers.length + (calcArea === null ? 0 : 1);
}

/** What a {@link ModelState.mergeModel} left behind, for the caller to report. */
export interface MergeSkips {
  /** Features whose id the workspace already held. */
  features: number;
  /** Receivers whose id the workspace already held. */
  receivers: number;
  /** True when the merge carried a calculation area and the workspace kept its own. */
  calcArea: boolean;
}

/** What a merge would land, and what it would leave behind. */
export interface MergePlan {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  /** The CRS the merged workspace is in — see {@link planMerge}. */
  crs: string;
  skipped: MergeSkips;
}

/**
 * What merging `incoming` into `current` would do — computed without touching
 * the store, so the import page can say it before the reader commits rather
 * than report it afterwards.
 *
 * An id `current` already holds is skipped, not re-minted and not overwritten.
 * Re-minting would turn a re-import of the same file into a second copy of
 * every feature, and ids are what the project keys on; skipping is what makes
 * a merge idempotent. Ids are one namespace across features and receivers,
 * because `validateProjectModel` checks them as one — which also covers a
 * duplicate inside `incoming` itself.
 *
 * An existing calculation area wins over an imported one. The model holds at
 * most one, and the one the reader drew is the one they can see.
 *
 * The CRS follows the same principle with one exception: a workspace that
 * already holds something keeps its own, because its coordinates are in it and
 * relabelling them would move nothing while claiming otherwise. An *empty*
 * workspace adopts the incoming CRS instead — there is nothing there for the
 * default to be true of, and dropping it would read a metric file as lon/lat.
 */
export function planMerge(
  current: LoadedModel,
  incoming: LoadedModel,
): MergePlan {
  const taken = new Set<string>([
    ...current.features.map((f) => f.id),
    ...current.receivers.map((r) => r.id),
  ]);

  const features: ModelFeature[] = [];
  for (const feature of incoming.features) {
    if (taken.has(feature.id)) continue;
    taken.add(feature.id);
    features.push(feature);
  }

  const receivers: ModelReceiver[] = [];
  for (const receiver of incoming.receivers) {
    if (taken.has(receiver.id)) continue;
    taken.add(receiver.id);
    receivers.push(receiver);
  }

  const empty =
    current.features.length === 0 &&
    current.receivers.length === 0 &&
    current.calcArea === null;

  return {
    features,
    receivers,
    calcArea: current.calcArea ?? incoming.calcArea,
    crs:
      (empty ? (incoming.crs ?? current.crs) : current.crs) ??
      DEFAULT_MODEL_CRS,
    skipped: {
      features: incoming.features.length - features.length,
      receivers: incoming.receivers.length - receivers.length,
      calcArea: current.calcArea !== null && incoming.calcArea !== null,
    },
  };
}

const commandStack = new CommandStack();

export const useModelStore = create<ModelState>((set, get) => {
  return {
    features: [],
    receivers: [],
    calcArea: null,
    crs: DEFAULT_MODEL_CRS,
    dirty: false,
    canUndo: false,
    canRedo: false,

    // Not an undoable command and not a `dirty` edit: it declares what the
    // stored coordinates already were, rather than changing them.
    setCRS: (crs) => {
      set({ crs });
    },

    addFeature: (feature) => {
      commandStack.execute({
        description: `Add ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({ features: [...s.features, feature], dirty: true }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.filter((f) => f.id !== feature.id),
            dirty: true,
          }));
        },
      });
    },

    updateFeature: (feature) => {
      const previous = get().features.find((f) => f.id === feature.id);
      if (!previous) return;
      commandStack.execute({
        description: `Update ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({
            features: s.features.map((f) =>
              f.id === feature.id ? feature : f,
            ),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.map((f) =>
              f.id === feature.id ? previous : f,
            ),
            dirty: true,
          }));
        },
      });
    },

    removeFeature: (id) => {
      const feature = get().features.find((f) => f.id === id);
      if (!feature) return;
      const index = get().features.indexOf(feature);
      commandStack.execute({
        description: `Remove ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({
            features: s.features.filter((f) => f.id !== id),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => {
            const next = [...s.features];
            next.splice(index, 0, feature);
            return { features: next, dirty: true };
          });
        },
      });
    },

    reset: () => {
      commandStack.clear();
      set({
        features: [],
        receivers: [],
        calcArea: null,
        crs: DEFAULT_MODEL_CRS,
        dirty: false,
        canUndo: false,
        canRedo: false,
      });
    },

    markClean: () => {
      set({ dirty: false });
    },

    undo: () => {
      commandStack.undo();
    },

    redo: () => {
      commandStack.redo();
    },

    getFeatureById: (id) => {
      return get().features.find((f) => f.id === id);
    },

    featuresByKind: (kind) => {
      return get().features.filter((f) => f.kind === kind);
    },

    addReceiver: (receiver) => {
      commandStack.execute({
        description: `Add receiver ${receiver.id}`,
        execute: () => {
          set((s) => ({ receivers: [...s.receivers, receiver], dirty: true }));
        },
        undo: () => {
          set((s) => ({
            receivers: s.receivers.filter((r) => r.id !== receiver.id),
            dirty: true,
          }));
        },
      });
    },

    updateReceiver: (receiver) => {
      const previous = get().receivers.find((r) => r.id === receiver.id);
      if (!previous) return;
      commandStack.execute({
        description: `Update receiver ${receiver.id}`,
        execute: () => {
          set((s) => ({
            receivers: s.receivers.map((r) =>
              r.id === receiver.id ? receiver : r,
            ),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => ({
            receivers: s.receivers.map((r) =>
              r.id === receiver.id ? previous : r,
            ),
            dirty: true,
          }));
        },
      });
    },

    removeReceiver: (id) => {
      const receiver = get().receivers.find((r) => r.id === id);
      if (!receiver) return;
      const index = get().receivers.indexOf(receiver);
      commandStack.execute({
        description: `Remove receiver ${receiver.id}`,
        execute: () => {
          set((s) => ({
            receivers: s.receivers.filter((r) => r.id !== id),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => {
            const next = [...s.receivers];
            next.splice(index, 0, receiver);
            return { receivers: next, dirty: true };
          });
        },
      });
    },

    // A bulk replacement is not an undoable edit: the command stack still
    // holds closures over the *previous* features/receivers, so undoing after a
    // load would splice stale objects back into the new model. Clearing the
    // stack is what keeps undo coherent — the former `loadReceivers` set the
    // array behind the stack's back and left it stale.
    //
    // `dirty` means "the workspace differs from what the project holds", not
    // "there is an edit the draft has not seen". Content that arrives through
    // a load — a file, an OSM fetch, a recovered draft — comes from outside
    // the project, so a load leaves the model dirty; only `markClean` clears
    // it, and only a successful save (or, in browser mode, the draft write)
    // calls that.
    //
    // It replaces the *whole* model. The former `loadFeatures` took features
    // alone and emptied `receivers` and `calcArea` with them, which is how an
    // import used to delete every placed receiver; it is deleted rather than
    // documented, so nothing can reach for it again.
    loadModel: ({ features, receivers, calcArea, crs }) => {
      commandStack.clear();
      set({
        features,
        receivers,
        calcArea,
        // A replacement replaces the CRS too: the coordinates that arrive are
        // in the CRS they arrive in, and keeping the previous one would label
        // them with a system they are not in.
        crs: crs ?? DEFAULT_MODEL_CRS,
        dirty: true,
        canUndo: false,
        canRedo: false,
      });
    },

    // Adds a model to the workspace instead of replacing it, as *one*
    // undoable command: a merge the reader regrets is one Ctrl+Z, not one per
    // imported feature. {@link planMerge} decides what of it lands.
    //
    // `dirty: true`, like `loadModel`: the content comes from outside the
    // project.
    mergeModel: (model) => {
      const state = get();
      const plan = planMerge(
        {
          features: state.features,
          receivers: state.receivers,
          calcArea: state.calcArea,
          crs: state.crs,
        },
        model,
      );

      const { features: addedFeatures, receivers: addedReceivers } = plan;
      const previousArea = state.calcArea;
      const previousCRS = state.crs;

      // A merge that changes nothing pushes nothing. Executing an empty
      // command would still clear the redo stack, so the second import of the
      // same file would be idempotent in the model and not in the history.
      if (
        addedFeatures.length === 0 &&
        addedReceivers.length === 0 &&
        plan.calcArea === previousArea &&
        plan.crs === previousCRS
      ) {
        return plan.skipped;
      }

      const addedIDs = new Set<string>([
        ...addedFeatures.map((f) => f.id),
        ...addedReceivers.map((r) => r.id),
      ]);

      commandStack.execute({
        description: `Merge ${String(addedFeatures.length)} features and ${String(addedReceivers.length)} receivers`,
        execute: () => {
          set((s) => ({
            features: [...s.features, ...addedFeatures],
            receivers: [...s.receivers, ...addedReceivers],
            calcArea: plan.calcArea,
            crs: plan.crs,
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.filter((f) => !addedIDs.has(f.id)),
            receivers: s.receivers.filter((r) => !addedIDs.has(r.id)),
            calcArea: previousArea,
            crs: previousCRS,
            dirty: true,
          }));
        },
      });

      return plan.skipped;
    },

    // `loadModel`'s twin for content that comes *from* the project rather than
    // from outside it: same replacement, but it lands clean.
    //
    // The difference is not cosmetic. Hydrating through `loadModel` would mark
    // a freshly reloaded workspace dirty, which arms the `beforeunload` guard
    // on every reload and makes the autosave write a draft of the project's
    // own model two seconds later — a draft carrying no hash, which then
    // guarantees a fetch on every subsequent start. Clean is also what keeps
    // the autosave idle, so a divergent draft survives to be offered.
    hydrateModel: ({ features, receivers, calcArea, crs }) => {
      commandStack.clear();
      set({
        features,
        receivers,
        calcArea,
        crs: crs ?? DEFAULT_MODEL_CRS,
        dirty: false,
        canUndo: false,
        canRedo: false,
      });
    },

    getReceiverById: (id) => {
      return get().receivers.find((r) => r.id === id);
    },

    setCalcArea: (area) => {
      const previous = get().calcArea;
      commandStack.execute({
        description: "Set calculation area",
        execute: () => {
          set({ calcArea: area, dirty: true });
        },
        undo: () => {
          set({ calcArea: previous, dirty: true });
        },
      });
    },

    clearCalcArea: () => {
      const previous = get().calcArea;
      if (!previous) return;
      commandStack.execute({
        description: "Clear calculation area",
        execute: () => {
          set({ calcArea: null, dirty: true });
        },
        undo: () => {
          set({ calcArea: previous, dirty: true });
        },
      });
    },
  };
});

// Mirroring the command stack into `canUndo`/`canRedo` happens *outside* the
// store initializer. Subscribing from inside it captured that initializer's
// `set`, and `CommandStack.subscribe` returns an unsubscribe function that was
// thrown away — so every re-evaluation of this module (HMR, a test that resets
// module state) added another listener that kept writing to the store it was
// created with. Keeping the disposer lets those cases detach cleanly.
const unsubscribeCommandStack = commandStack.subscribe(() => {
  useModelStore.setState({
    canUndo: commandStack.canUndo(),
    canRedo: commandStack.canRedo(),
  });
});

/** Detaches the command-stack listener. For HMR and test teardown. */
export function disposeModelStore(): void {
  unsubscribeCommandStack();
}

if (import.meta.hot) {
  import.meta.hot.dispose(disposeModelStore);
}
