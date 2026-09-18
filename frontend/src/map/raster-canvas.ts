/**
 * The one file in this tree that touches a canvas.
 *
 * It exists to be a single import a test can replace. jsdom — v28 here, with no
 * `canvas` package installed — throws from `HTMLCanvasElement.prototype
 * .getContext`, so every unit test of the raster layer mocks this module, and
 * the real path is covered by `frontend/e2e/`, which drives a real Chromium in
 * WASM mode. Keeping the encoder to three statements is what makes that
 * division honest: there is no logic in here left to go untested.
 */
import type { RGBABytes } from "./raster-image";

/**
 * A PNG data URL for the pixels, or `null` where there is no 2D context.
 *
 * `null` rather than a throw: a browser that refuses a 2D context (a hardened
 * profile, a canvas-fingerprinting blocker) is a reason to say the raster could
 * not be drawn, not a reason to take the map down. The caller already has a
 * status for it and the receiver levels still draw.
 *
 * A data URL rather than `toBlob` + `createObjectURL`: `toBlob` is async, so it
 * would need revocation bookkeeping across every run and indicator switch, and
 * at these sizes it is not worth one. A noise raster is large flat regions of a
 * ten-colour ramp, which PNG encodes into tens of kB; base64 adds a third of
 * that. If grids ever grow past what a data URL can carry, the blob path — with
 * a revoke on cleanup — is the answer, not a bigger string.
 */
export function encodeRasterPNG(
  rgba: RGBABytes,
  width: number,
  height: number,
): string | null {
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;

  const context = canvas.getContext("2d");
  if (context === null) return null;

  context.putImageData(new ImageData(rgba, width, height), 0, 0);
  return canvas.toDataURL("image/png");
}
