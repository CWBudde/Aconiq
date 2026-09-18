// The kernel's contours against the lines Go traces.
//
// contours.test.ts drives the same entry point, but what it can see is the
// boundary: the argument order, the echoed CRS, a bounding box around the
// vertices. It would pass against a kernel built from a stale tree, and it
// would pass against a second marching-squares implementation hidden behind
// the same export — which is exactly the thing `internal/report/contour`'s
// package comment says must not exist, because `window.aconiq.contours` and
// GET /api/v1/runs/{id}/contours answer the same question about where a 55 dB
// line falls.
//
// So this file compares vertices. The fixture and the goldens belong to the Go
// tree — backend/internal/report/contour/parity_test.go writes them and
// `just update-golden` regenerates them — so nothing here restates what the
// lines ought to be: it reads the raster bytes `results.SaveRaster` actually
// wrote, puts the recorded request to the real WASM kernel, and compares
// against what `contour.FromRaster` actually returned.
//
// Skipped, not failed, when the kernel is not built — `kernelSkipReason()`
// turns that into a failure under ACONIQ_REQUIRE_WASM, which CI sets.

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "./kernel-node";
import type { ContourRequest, ContourResult, RasterMetadata } from "./types";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses.
const FIXTURE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/report/contour/testdata/contour-parity",
);

const skipReason = kernelSkipReason();

/** `contourParityInput` in parity_test.go. */
interface ParityInput {
  metadata: RasterMetadata;
  bands: number[][];
  request: {
    target_crs: string;
    interval: number;
  };
}

function parityInput(): ParityInput {
  return JSON.parse(
    readFileSync(resolve(FIXTURE_DIR, "input.golden.json"), "utf8"),
  ) as ParityInput;
}

function parityGolden(): ContourResult {
  return JSON.parse(
    readFileSync(resolve(FIXTURE_DIR, "contours.golden.json"), "utf8"),
  ) as ContourResult;
}

/**
 * The raster payload as the CLI wrote it, rather than as this file would encode
 * it. Encoding the fixture's `bands` here would make the frontend a second
 * writer of the byte contract, which is what `raster-bin.ts` exists to prevent
 * and what raster-bin.parity.test.ts pins from its own side.
 */
function rasterPayload(): Uint8Array {
  const file = readFileSync(resolve(FIXTURE_DIR, "raster.golden.bin"));

  // Copy rather than view: readFileSync hands back a Buffer over a *pooled*
  // ArrayBuffer larger than the file and shared with other reads, so
  // `file.buffer` is not the payload and the kernel's length check would
  // rightly refuse it.
  return new Uint8Array(
    file.buffer.slice(file.byteOffset, file.byteOffset + file.byteLength),
  );
}

/**
 * parity_test.go's `contourParityRound6`. The goldens are written rounded, so
 * the comparison rounds too — 1e-6 is their precision and explicitly not a
 * tolerance on where a contour falls.
 */
function round6(value: number): number {
  return Math.round(value * 1e6) / 1e6;
}

describe.skipIf(skipReason !== null)("kernel contours vs. Go goldens", () => {
  it("traces the fixture raster into the same lines", async () => {
    const input = parityInput();
    const expected = parityGolden();

    const request: ContourRequest = {
      raster: input.metadata,
      target_crs: input.request.target_crs,
      interval: input.request.interval,
    };

    const kernel = await getNodeKernel();
    const result = await kernel.contours(rasterPayload(), request);

    // The raster is EPSG:25832 and the request asks for EPSG:4326, so an
    // echoed CRS that is not the requested one means the reprojection did not
    // run. The interval is echoed for the legend, and 4 dB is deliberately not
    // the 5 dB default: a kernel that ignored the field would say 5 here and
    // trace a different set of levels below.
    expect(result.crs).toBe(expected.crs);
    expect(result.interval).toBe(expected.interval);

    // Line for line, in order. `joinSegments` sorts its output precisely so
    // that the order is reproducible, so the order is part of the contract
    // rather than an artefact of how the cells happened to be walked.
    expect(result.lines.map((line) => [line.band_name, line.level])).toEqual(
      expected.lines.map((line) => [line.band_name, line.level]),
    );

    expected.lines.forEach((want, index) => {
      const got = result.lines[index];
      const where = `line ${String(index)} (${want.band_name} at ${String(want.level)} dB)`;

      expect(got, where).toBeDefined();
      // Vertex count before vertices: a line traced one cell short still
      // agrees on every vertex it does have, and the fixture's nodata cell
      // makes exactly that failure possible.
      expect(got?.points.length, `${where} vertex count`).toBe(
        want.points.length,
      );

      want.points.forEach(([x, y], vertex) => {
        const point = got?.points[vertex];
        expect(
          Math.abs(round6(point?.[0] ?? NaN) - x),
          `${where} vertex ${String(vertex)} x`,
        ).toBeLessThanOrEqual(1e-6);
        expect(
          Math.abs(round6(point?.[1] ?? NaN) - y),
          `${where} vertex ${String(vertex)} y`,
        ).toBeLessThanOrEqual(1e-6);
      });
    });
  });

  // What the comparison above rests on, stated rather than left implicit: the
  // goldens carry both bands, told apart by name, and the two bands' levels do
  // not overlap. Without that a band mix-up would only swap two labels and the
  // vertex comparison would still pass on whichever line happened to sort
  // first.
  it("distinguishes the two bands by name, not by position", () => {
    const expected = parityGolden();

    const levels = new Map<string, number[]>();
    for (const line of expected.lines) {
      levels.set(line.band_name, [
        ...(levels.get(line.band_name) ?? []),
        line.level,
      ]);
    }

    const day = levels.get("LrDay") ?? [];
    const night = levels.get("LrNight") ?? [];

    expect(day.length).toBeGreaterThan(0);
    expect(night.length).toBeGreaterThan(0);
    expect(Math.min(...day)).toBeGreaterThan(Math.max(...night));
  });
});

if (skipReason !== null) {
  describe("kernel contours vs. Go goldens", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the runner output.
    });
  });
}
