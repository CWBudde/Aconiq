import { create } from "zustand";
import type { BasemapId } from "./basemap";

interface LayerVisibility {
  [groupId: string]: boolean;
}

interface MapState {
  /** Current basemap style ID */
  basemap: BasemapId;
  setBasemap: (id: BasemapId) => void;

  /** Layer group visibility */
  layerVisibility: LayerVisibility;
  toggleLayer: (groupId: string) => void;
  setLayerVisible: (groupId: string, visible: boolean) => void;

  /** Currently selected feature IDs */
  selectedFeatureIds: string[];
  setSelectedFeatureIds: (ids: string[]) => void;
  clearSelection: () => void;

  /** Hovered feature ID (for highlight) */
  hoveredFeatureId: string | null;
  setHoveredFeatureId: (id: string | null) => void;
}

export const useMapStore = create<MapState>((set) => ({
  basemap: "light",
  setBasemap: (id) => {
    set({ basemap: id });
  },

  layerVisibility: {},
  toggleLayer: (groupId) => {
    set((state) => ({
      layerVisibility: {
        ...state.layerVisibility,
        [groupId]: !(state.layerVisibility[groupId] ?? true),
      },
    }));
  },
  setLayerVisible: (groupId, visible) => {
    set((state) => ({
      layerVisibility: {
        ...state.layerVisibility,
        [groupId]: visible,
      },
    }));
  },

  selectedFeatureIds: [],
  setSelectedFeatureIds: (ids) => {
    set({ selectedFeatureIds: ids });
  },
  clearSelection: () => {
    set({ selectedFeatureIds: [], hoveredFeatureId: null });
  },

  hoveredFeatureId: null,
  setHoveredFeatureId: (id) => {
    set({ hoveredFeatureId: id });
  },
}));
