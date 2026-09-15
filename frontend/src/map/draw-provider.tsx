import type * as React from "react";
import { useDraw } from "./use-draw";
import type { DrawMode } from "./use-draw";
import { DrawContext } from "./use-draw-context";

/**
 * Owns the terra-draw instance, and exists so that `useDraw` runs **inside**
 * `MapView`.
 *
 * `useDraw` reads the map through `useMap()`, which reads `MapContext` —
 * provided by `MapView` around its own children. The workspace page used to
 * call `useDraw` one level above the `<MapView>` it rendered, so `useMap()`
 * returned the context's default `null`, the terra-draw init effect
 * early-returned, and no adapter was ever attached: clicking a draw tool
 * highlighted the button and did nothing else. Every other `useMap()` consumer
 * — the layer control, the coordinate display, the popup, the model layers —
 * is a `MapView` child and was unaffected, which is part of why this went
 * unnoticed. The other part is that `pages/map.test.tsx` mocks `useDraw`.
 *
 * Rendering the provider as a `MapView` child is what makes the position
 * structural rather than a convention someone can quietly undo.
 */
export function DrawProvider({
  onFinish,
  children,
}: {
  onFinish: (mode: DrawMode, feature: GeoJSON.Feature) => void;
  children: React.ReactNode;
}) {
  const draw = useDraw({ onFinish });
  return <DrawContext value={draw}>{children}</DrawContext>;
}
