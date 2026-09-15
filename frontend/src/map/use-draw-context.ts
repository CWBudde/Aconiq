import { createContext, useContext } from "react";
import type { DrawMode } from "./use-draw";

export interface DrawApi {
  activeMode: DrawMode;
  setMode: (mode: DrawMode) => void;
  cancel: () => void;
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
