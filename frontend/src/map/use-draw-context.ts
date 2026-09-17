import { createContext, useContext } from "react";
import type { DrawMode } from "./use-draw";

/**
 * What terra-draw reports about a feature the user is editing, rather than
 * drawing.
 *
 * `change` fires per pointer move while a feature or one of its vertices is
 * being dragged, and its `ids` cover terra-draw's own selection points and
 * midpoints as well as the feature — a listener has to filter for the id it
 * put there. `select` and `deselect` bracket the *selection*; `finish` brackets
 * a single gesture inside it.
 */
export interface DrawSelectionListeners {
  onChange?: (ids: string[], type: string) => void;
  onSelect?: (id: string) => void;
  /**
   * One editing gesture has ended: a feature or a vertex was dropped, a
   * midpoint was inserted, a coordinate was deleted.
   *
   * This — not `deselect` — is the per-drag boundary. Terra-draw's select mode
   * calls `onFinish` from its drag-end path and leaves the feature selected, so
   * a listener that waits for `deselect` sees one boundary for a whole run of
   * drags. `deselect` still arrives, once, when the selection is actually
   * cleared.
   */
  onFinish?: (id: string) => void;
  onDeselect?: (id: string) => void;
}

export interface DrawApi {
  activeMode: DrawMode;
  /**
   * Bumped every time the terra-draw instance behind this API is rebuilt.
   *
   * A basemap switch replaces the MapLibre map, which tears the instance and
   * its feature store down and builds an empty pair. Nothing else about the API
   * changes — `activeMode` is React state that survives it — so a consumer that
   * has put a feature into the store has no other way to learn that its copy is
   * gone. Anything holding state *inside* terra-draw depends on this.
   */
  instanceEpoch: number;
  setMode: (mode: DrawMode) => void;
  cancel: () => void;
  /**
   * Puts features into terra-draw's own store so select mode has something to
   * reshape. Each one needs `properties.mode` naming the mode that owns it
   * (`point`, `linestring`, `polygon`) or terra-draw rejects it.
   *
   * Coordinates go in and come out in `DISPLAY_CRS` — terra-draw knows nothing
   * about the model's CRS.
   *
   * Returns false when any feature was rejected, or when there is no terra-draw
   * instance yet. Terra-draw *validates* rather than throws, so without this the
   * caller would go on to select an id nothing holds.
   */
  addFeatures: (features: GeoJSON.Feature[]) => boolean;
  /** Arms select mode on a feature already added through {@link addFeatures}. */
  selectFeature: (id: string) => void;
  /** Takes features back out of terra-draw's store. Unknown ids are ignored. */
  removeFeatures: (ids: string[]) => void;
  /** A feature as terra-draw currently holds it, mid-drag included. */
  getFeature: (id: string) => GeoJSON.Feature | undefined;
  /**
   * Subscribes to the selection events, and returns the unsubscribe.
   *
   * A subscription rather than options on `DrawProvider`, because the consumer
   * is a hook *inside* the provider: `useDraw` has to run under `MapContext`
   * (see `draw-provider.tsx`), which puts it above everything that would want
   * to hand it a callback. The subscriber set also survives a map rebuild,
   * which tears the terra-draw instance down and builds a new one.
   */
  subscribeSelection: (listeners: DrawSelectionListeners) => () => void;
}

/**
 * The draw API, published by `DrawProvider`. Split from the provider the way
 * `use-map.ts` is split from `MapView`: a module that exports both a component
 * and a hook loses fast refresh.
 */
export const DrawContext = createContext<DrawApi | null>(null);

/** Throws outside a `DrawProvider`, rather than being silently inert. */
export function useDrawContext(): DrawApi {
  const draw = useContext(DrawContext);
  if (draw === null) {
    throw new Error("useDrawContext must be used inside a DrawProvider");
  }
  return draw;
}
