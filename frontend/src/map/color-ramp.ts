/**
 * Color ramps for noise level visualization.
 *
 * Standard noise mapping uses a green-yellow-orange-red-purple ramp
 * where each 5 dB step gets a distinct color. This matches conventions
 * in environmental noise mapping (ISO 1996, EU END).
 */

export interface ColorStop {
  value: number;
  color: string;
  label: string;
}

/** Standard noise level ramp (dB(A), 5 dB steps) */
export const NOISE_LEVEL_RAMP: ColorStop[] = [
  { value: 35, color: "#1a9641", label: "< 40" },
  { value: 40, color: "#69b764", label: "40" },
  { value: 45, color: "#a6d96a", label: "45" },
  { value: 50, color: "#d9ef8b", label: "50" },
  { value: 55, color: "#fee08b", label: "55" },
  { value: 60, color: "#fdae61", label: "60" },
  { value: 65, color: "#f46d43", label: "65" },
  { value: 70, color: "#d73027", label: "70" },
  { value: 75, color: "#a50026", label: "75" },
  { value: 80, color: "#67001f", label: "> 75" },
];

/**
 * Build a MapLibre interpolate expression from a color ramp.
 * Suitable for use in paint properties like `fill-color` or `raster-color`.
 */
export function rampToExpression(
  ramp: ColorStop[],
  property = "value",
): unknown[] {
  const stops: unknown[] = [];
  for (const stop of ramp) {
    stops.push(stop.value, stop.color);
  }
  return ["interpolate", ["linear"], ["get", property], ...stops];
}
