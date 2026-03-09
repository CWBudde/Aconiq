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

/** Drawing modes — "calc-area" maps to polygon internally but signals a different intent */
export type DrawMode = "point" | "linestring" | "polygon" | "select" | "static" | "calc-area";

interface UseDrawOptions {
  onFinish?: (mode: DrawMode, feature: GeoJSON.Feature) => void;
}

interface UseDrawReturn {
  activeMode: DrawMode;
  setMode: (mode: DrawMode) => void;
  cancel: () => void;
}

export function useDraw(options: UseDrawOptions = {}): UseDrawReturn {
  const map = useMap();
  const drawRef = useRef<TerraDraw | null>(null);
  const [activeMode, setActiveMode] = useState<DrawMode>("static");
  const activeModeRef = useRef<DrawMode>("static");
  const onFinishRef = useRef(options.onFinish);
  onFinishRef.current = options.onFinish;

  useEffect(() => {
    if (!map) return;

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
    draw.setMode("static");

    draw.on("finish", (id: string | number) => {
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

    drawRef.current = draw;

    return () => {
      draw.stop();
      drawRef.current = null;
    };
  }, [map]);

  const setMode = useCallback((mode: DrawMode) => {
    // "calc-area" reuses terra-draw's polygon mode but is tracked separately
    const terraMode = mode === "calc-area" ? "polygon" : mode;
    drawRef.current?.setMode(terraMode);
    activeModeRef.current = mode;
    setActiveMode(mode);
  }, []);

  const cancel = useCallback(() => {
    drawRef.current?.setMode("static");
    activeModeRef.current = "static";
    setActiveMode("static");
  }, []);

  return { activeMode, setMode, cancel };
}
