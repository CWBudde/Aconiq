/**
 * The result raster's pixels — the half of the raster layer that has no DOM.
 *
 * MapLibre 5 cannot colour a raster from its values: there is no `raster-color`
 * and no `["raster-value"]` in its style specification (both are Mapbox GL JS
 * v3), so the ramp has to be applied here and the layer handed finished pixels.
 * `raster-canvas.ts` is the one file that turns these bytes into an image;
 * everything in this one is arithmetic, which is what makes it testable —
 * jsdom has no 2D context at all.
 *
 * Two things here are easy to get wrong and impossible to see afterwards.
 *
 * **The row order.** A raster binary's row 0 is the *southernmost*, because
 * `geo.GridReceiverSet.Generate` walks Y ascending; a canvas `ImageData`'s row
 * 0 is the *top* of the image, which is its north edge. A map drawn without the
 * flip is vertically mirrored and looks entirely plausible — it is the same
 * failure `results.Georeference.Validate` refuses to risk when it rejects an
 * unrecognised `row_order` rather than guessing.
 *
 * **The colours.** These have to be the colours MapLibre paints the receiver
 * circles with, or the raster and the circles drawn on top of it would disagree
 * about what 57.5 dB looks like, under one legend that showed neither.
 * `rampToExpression` asks for `["interpolate", ["linear"], …]`, which MapLibre
 * resolves through `Color.interpolate` with its default `rgb` space: a
 * component-wise lerp of the unpremultiplied sRGB channels. So does this.
 */
import type { ColorStop } from "./color-ramp";

/** Channels per pixel. RGBA, because nodata is expressed as transparency. */
const CHANNELS = 4;

/** An opaque `#rrggbb` stop, parsed once per raster rather than per cell. */
interface RGB {
  r: number;
  g: number;
  b: number;
}

function parseHexColor(color: string): RGB {
  const hex = color.trim().replace(/^#/, "");
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) {
    throw new Error(`color stop ${color} is not a #rrggbb value`);
  }
  return {
    r: Number.parseInt(hex.slice(0, 2), 16),
    g: Number.parseInt(hex.slice(2, 4), 16),
    b: Number.parseInt(hex.slice(4, 6), 16),
  };
}

/**
 * The ramp's colour at a value, clamped to the end stops outside its range.
 *
 * Clamped rather than extrapolated, and rather than transparent: MapLibre's
 * `interpolate` clamps too, so 34 dB is the same green there and here. That a
 * value below the first stop is still painted is the ramp's own claim — the
 * first stop is labelled "< 40" — and is not the same thing as an uncomputed
 * cell, which {@link rasterToRGBA} makes transparent instead.
 */
export function rampColorRGB(ramp: readonly ColorStop[], value: number): RGB {
  const first = ramp[0];
  const last = ramp[ramp.length - 1];
  if (first === undefined || last === undefined) {
    throw new Error("a colour ramp needs at least one stop");
  }

  if (value <= first.value) return parseHexColor(first.color);
  if (value >= last.value) return parseHexColor(last.color);

  for (let i = 1; i < ramp.length; i += 1) {
    const upper = ramp[i];
    const lower = ramp[i - 1];
    if (upper === undefined || lower === undefined) break;
    if (value > upper.value) continue;

    const span = upper.value - lower.value;
    // Two stops at the same value: the upper one wins, which is what
    // `interpolate` does with a zero-width span rather than dividing by it.
    if (span <= 0) return parseHexColor(upper.color);

    const t = (value - lower.value) / span;
    const a = parseHexColor(lower.color);
    const b = parseHexColor(upper.color);
    return {
      r: Math.round(a.r + (b.r - a.r) * t),
      g: Math.round(a.g + (b.g - a.g) * t),
      b: Math.round(a.b + (b.b - a.b) * t),
    };
  }

  // Unreachable: the loop covers every value below `last.value`, which the
  // guard above already excluded. Kept so the function has no implicit return.
  return parseHexColor(last.color);
}

/**
 * RGBA bytes over a real `ArrayBuffer` rather than an `ArrayBufferLike`.
 *
 * `Uint8ClampedArray`'s default type parameter is the latter, and `ImageData`'s
 * constructor accepts only the former — so the alias is what lets
 * `raster-canvas.ts` hand these bytes straight to `new ImageData` without a
 * cast that would also silence a genuinely wrong buffer.
 */
export type RGBABytes = Uint8ClampedArray<ArrayBuffer>;

export interface RasterPixelsInput {
  /** One band, in raster order — row-major, row 0 southernmost. */
  values: Float64Array;
  width: number;
  height: number;
  /** The sidecar's `nodata`. Cells holding it are painted transparent. */
  nodata: number;
  ramp: readonly ColorStop[];
}

/**
 * RGBA for a canvas, top-down, with uncomputed cells transparent.
 *
 * `nodata` and any non-finite value become alpha 0 rather than the ramp's
 * bottom colour. `#1a9641` means "below 40 dB(A)", which is an answer; painting
 * a cell that was never computed with it would present an absence as a result,
 * the same lie `toCollection` already refuses to tell for a receiver.
 */
export function rasterToRGBA(input: RasterPixelsInput): RGBABytes {
  const { values, width, height, nodata, ramp } = input;

  if (!Number.isInteger(width) || width <= 0) {
    throw new Error(
      `raster width must be a positive integer, got ${String(width)}`,
    );
  }
  if (!Number.isInteger(height) || height <= 0) {
    throw new Error(
      `raster height must be a positive integer, got ${String(height)}`,
    );
  }
  if (values.length !== width * height) {
    throw new Error(
      `raster band holds ${String(values.length)} values, expected ${String(width * height)} for ${String(width)}x${String(height)}`,
    );
  }

  const rgba: RGBABytes = new Uint8ClampedArray(
    new ArrayBuffer(width * height * CHANNELS),
  );

  for (let row = 0; row < height; row += 1) {
    // The flip. Destination row 0 is the image's north edge, and the northern-
    // most source row is the last one.
    const sourceRow = height - 1 - row;
    for (let x = 0; x < width; x += 1) {
      const value = values[sourceRow * width + x];
      const at = (row * width + x) * CHANNELS;

      if (value === undefined || !Number.isFinite(value) || value === nodata) {
        // Alpha stays 0, and the colour channels stay 0 with it: a transparent
        // pixel's RGB is still sampled when the texture is filtered, so leaving
        // a colour there would bleed it into the neighbouring cell's edge.
        continue;
      }

      const { r, g, b } = rampColorRGB(ramp, value);
      rgba[at] = r;
      rgba[at + 1] = g;
      rgba[at + 2] = b;
      rgba[at + 3] = 255;
    }
  }

  return rgba;
}
