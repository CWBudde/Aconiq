import { describe, expect, it } from "vitest";
import {
  defaultFrame,
  drawnFrame,
  MIN_WINDOW_M,
  movedFrame,
  nudgedFrame,
  resizedFrame,
  sameFrame,
  snap,
} from "./frame-edit";

const frame = { u_m: 2, sill_z_m: 1, width_m: 1.2, height_m: 1.5 };

describe("frame edits", () => {
  it("snaps to a 5 cm grid", () => {
    expect(snap(1.024)).toBe(1);
    expect(snap(1.026)).toBe(1.05);
  });

  it("keeps a moved window on its facade and above the ground", () => {
    expect(movedFrame(frame, 1.02, 0.5, 10)).toEqual({
      ...frame,
      u_m: 3,
      sill_z_m: 1.5,
    });
    expect(movedFrame(frame, 20, -5, 10)).toEqual({
      ...frame,
      u_m: 8.8,
      sill_z_m: 0,
    });
    expect(movedFrame(frame, -5, 0, 10).u_m).toBe(0);
  });

  it("resizes from the top right corner, no smaller than a window", () => {
    expect(resizedFrame(frame, 0.3, 0.5, 10)).toEqual({
      ...frame,
      width_m: 1.5,
      height_m: 2,
    });
    expect(resizedFrame(frame, -5, -5, 10)).toEqual({
      ...frame,
      width_m: MIN_WINDOW_M,
      height_m: MIN_WINDOW_M,
    });
    expect(resizedFrame(frame, 50, 0, 10).width_m).toBe(8);
  });

  it("draws between two points in either order, clipped to the facade", () => {
    expect(drawnFrame(3, 2.5, 1, 1, 10, 9)).toEqual({
      u_m: 1,
      sill_z_m: 1,
      width_m: 2,
      height_m: 1.5,
    });
    expect(drawnFrame(-1, -1, 0.5, 0.5, 10, 9)).toEqual({
      u_m: 0,
      sill_z_m: 0,
      width_m: 0.5,
      height_m: 0.5,
    });
    expect(drawnFrame(1, 1, 1.1, 3, 10, 9)).toBeNull();
  });

  it("nudges with the arrow keys, a metre with Shift", () => {
    expect(nudgedFrame(frame, "ArrowRight", false, 10)?.u_m).toBe(2.1);
    expect(nudgedFrame(frame, "ArrowLeft", true, 10)?.u_m).toBe(1);
    expect(nudgedFrame(frame, "ArrowUp", false, 10)?.sill_z_m).toBe(1.1);
    expect(nudgedFrame(frame, "ArrowDown", true, 10)?.sill_z_m).toBe(0);
    expect(nudgedFrame(frame, "Enter", false, 10)).toBeNull();
  });

  it("adds a window that fits even a small facade", () => {
    expect(defaultFrame(10, 9)).toEqual({
      u_m: 4.4,
      sill_z_m: 0.9,
      width_m: 1.2,
      height_m: 1.5,
    });
    const small = defaultFrame(1, 2);
    expect(small.u_m + small.width_m).toBeLessThanOrEqual(1);
    expect(small.sill_z_m + small.height_m).toBeLessThanOrEqual(2);
  });

  it("compares frames", () => {
    expect(sameFrame(frame, { ...frame })).toBe(true);
    expect(sameFrame(frame, { ...frame, u_m: 2.05 })).toBe(false);
  });
});
