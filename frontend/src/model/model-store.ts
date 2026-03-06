import { create } from "zustand";
import type { FeatureKind, ModelFeature } from "./types";
import { CommandStack } from "./command-stack";

interface ModelState {
  features: ModelFeature[];
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
}

const commandStack = new CommandStack();

export const useModelStore = create<ModelState>((set, get) => {
  commandStack.subscribe(() => {
    set({
      canUndo: commandStack.canUndo(),
      canRedo: commandStack.canRedo(),
    });
  });

  return {
    features: [],
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
      set({ features, dirty: false, canUndo: false, canRedo: false });
    },

    reset: () => {
      commandStack.clear();
      set({ features: [], dirty: false, canUndo: false, canRedo: false });
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
  };
});
