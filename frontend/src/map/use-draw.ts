import { useCallback, useEffect, useRef, useState } from "react";
import {
  TerraDraw,
  TerraDrawPointMode,
  TerraDrawLineStringMode,
  TerraDrawPolygonMode,
  TerraDrawSelectMode,
  TerraDrawRenderMode,
} from "terra-draw";
import { TerraDrawMapLibreGLAdapter } from "terra-draw-maplibre-gl-adapter";
import { useMap } from "./use-map";
import type { DrawApi, DrawSelectionListeners } from "./use-draw-context";

/** Drawing modes — "calc-area" maps to polygon internally but signals a different intent */
export type DrawMode =
  | "point"
  | "linestring"
  | "polygon"
  | "select"
  | "static"
  | "calc-area";

interface UseDrawOptions {
  onFinish?: (mode: DrawMode, feature: GeoJSON.Feature) => void;
}

type UseDrawReturn = DrawApi;

/** "calc-area" reuses terra-draw's polygon mode but is tracked separately. */
function terraModeFor(mode: DrawMode): string {
  return mode === "calc-area" ? "polygon" : mode;
}

export function useDraw(options: UseDrawOptions = {}): UseDrawReturn {
  const map = useMap();
  const drawRef = useRef<TerraDraw | null>(null);
  const [activeMode, setActiveMode] = useState<DrawMode>("static");
  const activeModeRef = useRef<DrawMode>("static");
  // Published so that a consumer holding state inside terra-draw can re-place
  // it after a rebuild — see `DrawApi.instanceEpoch`.
  const [instanceEpoch, setInstanceEpoch] = useState(0);
  const onFinishRef = useRef(options.onFinish);
  onFinishRef.current = options.onFinish;

  // A set rather than a single callback: the subscribers are hooks rendered
  // under the provider, and the set outlives the terra-draw instance so a map
  // rebuild does not silently drop them.
  const selectionListeners = useRef(new Set<DrawSelectionListeners>());

  useEffect(() => {
    if (!map) return;

    // Whether MapLibre has already torn this map down — see the cleanup.
    // `remove` fires at the very end of `map.remove()`, so by the time this
    // hook cleans up after it the flag is set.
    let mapRemoved = false;
    const markRemoved = () => {
      mapRemoved = true;
    };
    map.on("remove", markRemoved);

    const draw = new TerraDraw({
      adapter: new TerraDrawMapLibreGLAdapter({ map }),
      modes: [
        new TerraDrawPointMode(),
        new TerraDrawLineStringMode(),
        new TerraDrawPolygonMode(),
        new TerraDrawSelectMode({
          flags: {
            point: { feature: { draggable: true } },
            linestring: {
              feature: {
                draggable: true,
                coordinates: {
                  midpoints: true,
                  draggable: true,
                  deletable: true,
                },
              },
            },
            polygon: {
              feature: {
                draggable: true,
                coordinates: {
                  midpoints: true,
                  draggable: true,
                  deletable: true,
                },
              },
            },
          },
        }),
        new TerraDrawRenderMode({ modeName: "static", styles: {} }),
      ],
    });

    draw.start();
    // The requested mode, not a hardcoded "static". `MapView` renders its
    // children while `map` is still null, so anything that arms a tool on
    // mount — `/model?draw=1`, or a click landing before `load` fires — has
    // already run `setMode` against a null `drawRef` and moved only React
    // state. Replaying `activeModeRef` here is what makes that request
    // survive initialization; without it the toolbar shows Point while
    // terra-draw sits in "static" and clicks do nothing. It also preserves
    // the armed tool across a basemap switch, which rebuilds the map.
    draw.setMode(terraModeFor(activeModeRef.current));

    draw.on("finish", (id: string | number) => {
      // Select mode finishes too. Terra-draw fires "finish" at the end of a
      // drag, and the handler below is written for a *newly drawn* shape: it
      // puts the tools back to "static" and removes the feature from the map.
      // Run in select mode it would disarm the tool mid-edit and delete the
      // feature being reshaped — so a reshape is never a *draw* finish, and
      // `use-geometry-edit.ts` owns the select-mode events instead.
      //
      // It is still the end of a gesture, and the only event that says so:
      // terra-draw leaves a dropped feature selected, so `deselect` comes once
      // for a whole run of drags rather than once per drag. Forwarding it is
      // what lets the owner project and seal per drag; swallowing it entirely
      // left the undo run open and, over a metric model, the reshape unwritten
      // until the selection was cleared.
      if (activeModeRef.current === "select") {
        for (const listener of selectionListeners.current) {
          listener.onFinish?.(String(id));
        }
        return;
      }

      const snapshot = draw.getSnapshot();
      const feature = snapshot.find((f) => f.id === id);
      if (feature && onFinishRef.current) {
        const currentMode = activeModeRef.current;
        activeModeRef.current = "static";
        setActiveMode("static");
        setTimeout(() => {
          try {
            draw.removeFeatures([id]);
          } catch {
            // may already be removed
          }
        }, 0);
        onFinishRef.current(currentMode, feature as GeoJSON.Feature);
      }
    });

    draw.on("change", (ids: (string | number)[], type: string) => {
      const asStrings = ids.map((id) => String(id));
      for (const listener of selectionListeners.current) {
        listener.onChange?.(asStrings, type);
      }
    });

    draw.on("select", (id: string | number) => {
      for (const listener of selectionListeners.current) {
        listener.onSelect?.(String(id));
      }
    });

    draw.on("deselect", (id: string | number) => {
      for (const listener of selectionListeners.current) {
        listener.onDeselect?.(String(id));
      }
    });

    drawRef.current = draw;
    // After `drawRef`, so the re-render this schedules finds the new instance
    // in place: a consumer re-arming on the epoch calls straight back in.
    setInstanceEpoch((epoch) => epoch + 1);

    return () => {
      map.off("remove", markRemoved);
      // This cleanup runs *after* the map is gone, both ways the map goes.
      // React destroys a deleted subtree's effects parent-first, so on leaving
      // the page `MapView`'s cleanup has already called `map.remove()`; and a
      // basemap switch rebuilds the map through that same cleanup one render
      // before this one runs. MapLibre's `remove()` deletes the style, so
      // `draw.stop()` against a removed map throws `getSource` of undefined
      // from the adapter's `clear()`. Nothing is lost by not calling it: the
      // adapter's listeners hung on a canvas the map already took out of the
      // DOM, and the layers it would remove went with the style. So a removed
      // map is skipped — silently, because there was no failure — and the
      // warning is kept for the case that matters: a live map whose teardown
      // fails, which would leave an adapter on it.
      if (!mapRemoved) {
        try {
          draw.stop();
        } catch (error) {
          console.warn("useDraw: terra-draw teardown failed", error);
        }
      }
      drawRef.current = null;
    };
  }, [map]);

  const setMode = useCallback((mode: DrawMode) => {
    drawRef.current?.setMode(terraModeFor(mode));
    activeModeRef.current = mode;
    setActiveMode(mode);
  }, []);

  const cancel = useCallback(() => {
    drawRef.current?.setMode("static");
    activeModeRef.current = "static";
    setActiveMode("static");
  }, []);

  const addFeatures = useCallback((features: GeoJSON.Feature[]): boolean => {
    const draw = drawRef.current;
    if (!draw) return false;
    try {
      const results = draw.addFeatures(
        features as Parameters<TerraDraw["addFeatures"]>[0],
      );
      // Terra-draw validates rather than throws, and a rejected feature leaves
      // an id nothing holds — which `selectFeature` would then throw on. The
      // caller checks this before selecting.
      return results.every((result) => result.valid);
    } catch (error) {
      console.warn("useDraw: terra-draw rejected a feature", error);
      return false;
    }
  }, []);

  const selectFeature = useCallback((id: string) => {
    try {
      drawRef.current?.selectFeature(id);
    } catch (error) {
      // Reported rather than swallowed: unlike `removeFeatures`, there is no
      // benign reason for this to fail once `addFeatures` has said the feature
      // was accepted, and a silent failure leaves select mode armed on nothing.
      console.warn("useDraw: could not select feature", id, error);
    }
  }, []);

  const removeFeatures = useCallback((ids: string[]) => {
    try {
      drawRef.current?.removeFeatures(ids);
    } catch {
      // Terra-draw throws on an id its store no longer holds, and the callers
      // are teardown paths that cannot know: a mode change clears the store
      // underneath them, and a map rebuild replaces it outright. Nothing is
      // left to remove in either case.
    }
  }, []);

  const getFeature = useCallback((id: string): GeoJSON.Feature | undefined => {
    const snapshot = drawRef.current?.getSnapshot() ?? [];
    return snapshot.find((f) => String(f.id) === id) as
      | GeoJSON.Feature
      | undefined;
  }, []);

  const subscribeSelection = useCallback(
    (listeners: DrawSelectionListeners) => {
      const set = selectionListeners.current;
      set.add(listeners);
      return () => {
        set.delete(listeners);
      };
    },
    [],
  );

  return {
    activeMode,
    instanceEpoch,
    setMode,
    cancel,
    addFeatures,
    selectFeature,
    removeFeatures,
    getFeature,
    subscribeSelection,
  };
}
