import { describe, expect, it } from "vitest";
import {
  gridReceiverAt,
  gridReceiverId,
  materializeGrid,
} from "./receiver-grid";

describe("receiver grid", () => {
  it("names receivers as padStart does, past four digits too", () => {
    expect(gridReceiverId(1)).toBe("R0001");
    expect(gridReceiverId(999)).toBe("R0999");
    expect(gridReceiverId(9999)).toBe("R9999");
    expect(gridReceiverId(10000)).toBe("R10000");
    expect(gridReceiverId(123456)).toBe("R123456");
  });

  // The witness `wasmkernel/grid_test.go` pins: at this origin, resolution
  // and column, a fused multiply-add rounds to 477895.77799999999. JavaScript
  // rounds the product and the sum separately, and the kernel must too.
  it("places a receiver at the twice-rounded coordinate", () => {
    const grid = {
      origin_x: 412345.678,
      origin_y: 5_800_000,
      resolution_m: 7.7,
      width: 8514,
      height: 1,
      height_m: 4,
    };
    expect(gridReceiverAt(grid, 8513).point.x).toBe(477895.77800000005);
  });

  // The receiver-list path sends its receivers through JSON.stringify, which
  // writes -0 as 0, and stores the kernel's echo; a grid run names them here.
  // A height of "-0" must come out as the same 0 both ways, or the stored CSV
  // differs ("-0" against "0") for the same run.
  it("names a negative-zero height as the zero the wire carries", () => {
    const grid = {
      origin_x: -0,
      origin_y: -0,
      resolution_m: 10,
      width: 2,
      height: 1,
      height_m: -0,
    };
    const first = gridReceiverAt(grid, 0);
    expect(Object.is(first.height_m, 0)).toBe(true);
    expect(Object.is(first.point.x, 0)).toBe(true);
    expect(Object.is(first.point.y, 0)).toBe(true);
    expect(first).toEqual(JSON.parse(JSON.stringify(first)));
  });

  it("walks row-major from cell (0,0), as the run's receivers always did", () => {
    const grid = {
      origin_x: 100.5,
      origin_y: 200.25,
      resolution_m: 0.1,
      width: 3,
      height: 2,
      height_m: 4.5,
    };
    expect(materializeGrid(grid)).toEqual([
      { id: "R0001", point: { x: 100.5, y: 200.25 }, height_m: 4.5 },
      { id: "R0002", point: { x: 100.5 + 1 * 0.1, y: 200.25 }, height_m: 4.5 },
      { id: "R0003", point: { x: 100.5 + 2 * 0.1, y: 200.25 }, height_m: 4.5 },
      { id: "R0004", point: { x: 100.5, y: 200.25 + 0.1 }, height_m: 4.5 },
      {
        id: "R0005",
        point: { x: 100.5 + 0.1, y: 200.25 + 0.1 },
        height_m: 4.5,
      },
      {
        id: "R0006",
        point: { x: 100.5 + 2 * 0.1, y: 200.25 + 0.1 },
        height_m: 4.5,
      },
    ]);
  });
});
