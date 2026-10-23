/** The drawing's size in SVG user units; it scales to its container. */
export const ELEVATION_VIEW_WIDTH = 640;
export const ELEVATION_VIEW_HEIGHT = 360;
/** Room around the facade for the storey labels and the ground line. */
export const ELEVATION_VIEW_PAD = 36;

/**
 * Where a facade's frame lands in the drawing: `u` metres along the facade
 * and `z` metres above the ground, at one scale for both axes so a window
 * keeps its proportions. This only draws what the Go code measured — the
 * frame itself (which edge, how long, which way u runs) is never computed
 * here.
 */
export interface ElevationScale {
  scale: number;
  x(u: number): number;
  y(z: number): number;
}

export function elevationScale(
  lengthM: number,
  heightM: number,
): ElevationScale {
  const pad = ELEVATION_VIEW_PAD;
  const scale = Math.min(
    (ELEVATION_VIEW_WIDTH - 2 * pad) / lengthM,
    (ELEVATION_VIEW_HEIGHT - 2 * pad) / heightM,
  );
  return {
    scale,
    x: (u) => pad + u * scale,
    y: (z) => ELEVATION_VIEW_HEIGHT - pad - z * scale,
  };
}
