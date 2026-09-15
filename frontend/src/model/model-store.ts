import { create } from "zustand";
import type {
  CalcArea,
  FeatureKind,
  ModelFeature,
  ModelReceiver,
} from "./types";
import { CommandStack } from "./command-stack";

interface ModelState {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  dirty: boolean;
  canUndo: boolean;
  canRedo: boolean;

  addFeature: (feature: ModelFeature) => void;
  updateFeature: (feature: ModelFeature) => void;
  removeFeature: (id: string) => void;
  loadFeatures: (features: ModelFeature[]) => void;
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

const commandStack = new CommandStack();

export const useModelStore = create<ModelState>((set, get) => {
  return {
    features: [],
    receivers: [],
    calcArea: null,
    dirty: false,
    canUndo: false,
    canRedo: false,

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

    // `dirty` means "the workspace differs from what the project holds", not
    // "there is an edit the draft has not seen". Content that arrives through
    // a load — a file, an OSM fetch, a recovered draft — comes from outside
    // the project, so a load leaves the model dirty; only `markClean` clears
    // it, and only a successful save (or, in browser mode, the draft write)
    // calls that.
    loadFeatures: (features) => {
      commandStack.clear();
      set({
        features,
        receivers: [],
        calcArea: null,
        dirty: true,
        canUndo: false,
        canRedo: false,
      });
    },

    reset: () => {
      commandStack.clear();
      set({
        features: [],
        receivers: [],
        calcArea: null,
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
    // stack (as `loadFeatures` does) is what keeps undo coherent — the former
    // `loadReceivers` set the array behind the stack's back and left it stale.
    loadModel: ({ features, receivers, calcArea }) => {
      commandStack.clear();
      set({
        features,
        receivers,
        calcArea,
        dirty: true,
        canUndo: false,
        canRedo: false,
      });
    },

    // Adds a model to the workspace instead of replacing it, as *one*
    // undoable command: a merge the reader regrets is one Ctrl+Z, not one per
    // imported feature.
    //
    // An id the workspace already holds is skipped, not re-minted and not
    // overwritten. Re-minting would turn a re-import of the same file into a
    // second copy of every feature — and, since ids are what the project keys
    // on, into a model the next save writes twice. Skipping makes the merge
    // idempotent: importing the same file twice leaves the workspace as the
    // first import left it. Ids are one namespace across features and
    // receivers, because `validateProjectModel` checks them as one.
    //
    // An existing calculation area wins over an imported one. The model holds
    // at most one, and the one the reader drew is the one they can see.
    //
    // `dirty: true`, like `loadFeatures` and `loadModel`: the content comes
    // from outside the project.
    mergeModel: (model) => {
      const state = get();
      const taken = new Set<string>([
        ...state.features.map((f) => f.id),
        ...state.receivers.map((r) => r.id),
      ]);

      const addedFeatures: ModelFeature[] = [];
      for (const feature of model.features) {
        if (taken.has(feature.id)) continue;
        taken.add(feature.id);
        addedFeatures.push(feature);
      }

      const addedReceivers: ModelReceiver[] = [];
      for (const receiver of model.receivers) {
        if (taken.has(receiver.id)) continue;
        taken.add(receiver.id);
        addedReceivers.push(receiver);
      }

      const previousArea = state.calcArea;
      const nextArea = previousArea ?? model.calcArea;

      const skipped: MergeSkips = {
        features: model.features.length - addedFeatures.length,
        receivers: model.receivers.length - addedReceivers.length,
        calcArea: previousArea !== null && model.calcArea !== null,
      };

      // A merge that changes nothing pushes nothing. Executing an empty
      // command would still clear the redo stack, so the second import of the
      // same file would be idempotent in the model and not in the undo
      // history.
      if (
        addedFeatures.length === 0 &&
        addedReceivers.length === 0 &&
        nextArea === previousArea
      ) {
        return skipped;
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
            calcArea: nextArea,
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.filter((f) => !addedIDs.has(f.id)),
            receivers: s.receivers.filter((r) => !addedIDs.has(r.id)),
            calcArea: previousArea,
            dirty: true,
          }));
        },
      });

      return skipped;
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
    hydrateModel: ({ features, receivers, calcArea }) => {
      commandStack.clear();
      set({
        features,
        receivers,
        calcArea,
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
