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
}));
