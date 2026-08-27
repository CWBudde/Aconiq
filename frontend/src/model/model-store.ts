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
  getReceiverById: (id: string) => ModelReceiver | undefined;

  setCalcArea: (area: CalcArea) => void;
  clearCalcArea: () => void;
}

/** A complete model replacement (draft restore, import). */
export interface LoadedModel {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
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

    loadFeatures: (features) => {
      commandStack.clear();
      set({
        features,
        receivers: [],
        calcArea: null,
        dirty: false,
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
