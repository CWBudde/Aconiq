import type { PointReceiver, ReceiverGrid } from "@/wasm/types";

/**
 * The receivers an automatic grid names, read off its descriptor.
 *
 * A grid run sends the kernel the six numbers of a {@link ReceiverGrid} and
 * the kernel expands them in Go (`wasmkernel.ReceiverGrid.Receivers`); the
 * browser names the same receivers here, for the run's table and its
 * `output_hash`. The two must agree to the bit, so both use the same form:
 *
 * - row-major from cell (0,0), the sequence number counting from 1;
 * - each coordinate `origin + index * resolution_m` — a product and a sum,
 *   each rounded on its own. Never an accumulation (`x += resolution_m`),
 *   which drifts away from the product a few cells in;
 * - ids `R${String(seq).padStart(4, "0")}`, so the 10 000th is `R10000`.
 *
 * `wasmkernel/grid_test.go` and `receiver-grid.test.ts` pin the same witness
 * for the rounding, and the two-path test in `browser-parity.test.ts` holds a
 * whole run to the receiver list it replaced.
 */

/** The id of the `seq`th receiver of a grid, counting from 1. */
export function gridReceiverId(seq: number): string {
  return `R${String(seq).padStart(4, "0")}`;
}

/** The receiver at `index` (from 0) of a grid, in row-major order. */
export function gridReceiverAt(
  grid: ReceiverGrid,
  index: number,
): PointReceiver {
  const row = Math.floor(index / grid.width);
  const col = index % grid.width;
  return {
    id: gridReceiverId(index + 1),
    point: {
      x: grid.origin_x + col * grid.resolution_m,
      y: grid.origin_y + row * grid.resolution_m,
    },
    height_m: grid.height_m,
  };
}

/**
 * Every receiver of a grid, as the run used to send them one by one. The
 * receiver-list path keeps this as the reference a grid run is compared
 * against; a grid run itself never builds the list to send it.
 */
export function materializeGrid(grid: ReceiverGrid): PointReceiver[] {
  const count = grid.width * grid.height;
  const receivers: PointReceiver[] = [];
  for (let index = 0; index < count; index++) {
    receivers.push(gridReceiverAt(grid, index));
  }
  return receivers;
}
