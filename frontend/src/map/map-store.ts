import { create } from "zustand";
import { type BasemapId, readStoredBasemap, storeBasemap } from "./basemap";
import {
  type LayerVisibility,
  readStoredLayerVisibility,
  readStoredResultIndicator,
  storeLayerVisibility,
  storeResultIndicator,
} from "./map-preferences";

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

  /**
   * Layer group visibility, remembered across reloads.
   *
   * Only the groups the user has actually touched appear here; a group that is
   * absent takes its own `defaultVisible`, which is why this resets to `{}`
   * rather than to a record of `true`s. Spelling the defaults out here would
   * be a third copy of them, after the group definitions and
   * `applyLayerVisibility`.
   *
   * Persisting it is what makes {@link resetLayerVisibility} necessary: before,
   * a user who had hidden everything got it back with a refresh, and now the
   * map stays empty until something says otherwise. The Map settings tab is
   * that something.
   */
  layerVisibility: LayerVisibility;
  toggleLayer: (groupId: string) => void;
  setLayerVisible: (groupId: string, visible: boolean) => void;
  resetLayerVisibility: () => void;

  /**
   * Which band of a result table the map paints, or `null` for "whatever the
   * table lists first".
   *
   * A preference, not a promise: `ResultLayers` resolves it against the
   * indicators the current run's table actually carries, and falls back to the
   * first when this names one the table lacks. Do not seed component state
   * from it — the comment at that resolution says why.
   */
  resultIndicator: string | null;
  setResultIndicator: (indicator: string | null) => void;
}

/** Write through to storage, then to state — the order `setBasemap` set. */
function persistVisibility(visibility: LayerVisibility): LayerVisibility {
  storeLayerVisibility(visibility);
  return visibility;
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

  layerVisibility: readStoredLayerVisibility(),
  toggleLayer: (groupId) => {
    set((state) => ({
      layerVisibility: persistVisibility({
        ...state.layerVisibility,
        [groupId]: !(state.layerVisibility[groupId] ?? true),
      }),
    }));
  },
  setLayerVisible: (groupId, visible) => {
    set((state) => ({
      layerVisibility: persistVisibility({
        ...state.layerVisibility,
        [groupId]: visible,
      }),
    }));
  },
  resetLayerVisibility: () => {
    set({ layerVisibility: persistVisibility({}) });
  },

  resultIndicator: readStoredResultIndicator(),
  setResultIndicator: (indicator) => {
    storeResultIndicator(indicator);
    set({ resultIndicator: indicator });
  },
}));
