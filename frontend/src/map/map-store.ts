import { create } from "zustand";
import { type BasemapId, readStoredBasemap, storeBasemap } from "./basemap";

interface LayerVisibility {
  [groupId: string]: boolean;
}

interface MapState {
  /** Current basemap style ID */
  basemap: BasemapId;
  setBasemap: (id: BasemapId) => void;

  /**
   * Whether the basemap tile source has failed in this session.
   *
   * It selects the style `MapView` builds with, and it is in the store rather
   * than in component state because it outlives one `MapView`: the map is torn
   * down and rebuilt when it flips, which is exactly how the model layers get
   * re-added (`ModelLayers` syncs on data, not on `styledata`, so swapping the
   * style in place would drop every model layer until the next edit).
   */
  tilesFailed: boolean;
  reportTilesFailed: () => void;
  /** The notice's Retry: try the real basemap again. */
  clearTilesFailed: () => void;

  /** Layer group visibility */
  layerVisibility: LayerVisibility;
  toggleLayer: (groupId: string) => void;
  setLayerVisible: (groupId: string, visible: boolean) => void;
}

export const useMapStore = create<MapState>((set) => ({
  basemap: readStoredBasemap(),
  setBasemap: (id) => {
    storeBasemap(id);
    set({ basemap: id });
  },

  tilesFailed: false,
  reportTilesFailed: () => {
    set({ tilesFailed: true });
  },
  clearTilesFailed: () => {
    set({ tilesFailed: false });
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
