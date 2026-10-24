/**
 * What a drag, a draw or an arrow key does to a window's rectangle while it
 * is being edited in the elevation view.
 *
 * These only keep a gesture inside the facade the Go code measured — its
 * length and the building's height come from `facadeset.Elevation` — so the
 * drawing does not wander off the wall. Whether the window fits its edge is
 * still `facadeset.PlaceWindow`'s answer when the gesture ends: a frame these
 * let through is placed or refused there, never stored unchecked.
 */

import type { FacadeWindowFrame } from "@/wasm/types";

/** The smallest window a gesture makes, in metres on either side. */
export const MIN_WINDOW_M = 0.2;
/** A dragged edge lands on a 5 cm grid. */
const SNAP_PER_M = 20;
/** An arrow key moves a window 10 cm, with Shift a metre. */
export const NUDGE_M = 0.1;
export const NUDGE_SHIFT_M = 1;

export function snap(value: number): number {
  return Math.round(value * SNAP_PER_M) / SNAP_PER_M;
}

function clamp(value: number, lo: number, hi: number): number {
  return Math.min(Math.max(value, lo), Math.max(lo, hi));
}

/**
 * The window moved by (du, dz) metres, kept on the facade: along it, above the
 * ground and under the eaves. A window taller than the facade keeps its height
 * and sits on the ground.
 */
export function movedFrame(
  frame: FacadeWindowFrame,
  du: number,
  dz: number,
  lengthM: number,
  heightM: number,
): FacadeWindowFrame {
  return {
    ...frame,
    u_m: clamp(snap(frame.u_m + du), 0, lengthM - frame.width_m),
    sill_z_m: clamp(snap(frame.sill_z_m + dz), 0, heightM - frame.height_m),
  };
}

/**
 * The window resized from its top right corner: wider by du, taller by dz,
 * its left edge and sill where they were, its corner on the facade.
 */
export function resizedFrame(
  frame: FacadeWindowFrame,
  du: number,
  dz: number,
  lengthM: number,
  heightM: number,
): FacadeWindowFrame {
  return {
    ...frame,
    width_m: clamp(snap(frame.width_m + du), MIN_WINDOW_M, lengthM - frame.u_m),
    height_m: clamp(
      snap(frame.height_m + dz),
      MIN_WINDOW_M,
      heightM - frame.sill_z_m,
    ),
  };
}

/**
 * The rectangle spanned between two points of the facade, clipped to it, or
 * null when it is smaller than a window.
 */
export function drawnFrame(
  u0: number,
  z0: number,
  u1: number,
  z1: number,
  lengthM: number,
  heightM: number,
): FacadeWindowFrame | null {
  const left = clamp(snap(Math.min(u0, u1)), 0, lengthM);
  const right = clamp(snap(Math.max(u0, u1)), 0, lengthM);
  const bottom = clamp(snap(Math.min(z0, z1)), 0, heightM);
  const top = clamp(snap(Math.max(z0, z1)), 0, heightM);
  if (right - left < MIN_WINDOW_M || top - bottom < MIN_WINDOW_M) return null;
  return {
    u_m: left,
    sill_z_m: bottom,
    width_m: right - left,
    height_m: top - bottom,
  };
}

/** The window an arrow key moves it to, or null for any other key. */
export function nudgedFrame(
  frame: FacadeWindowFrame,
  key: string,
  shift: boolean,
  lengthM: number,
  heightM: number,
): FacadeWindowFrame | null {
  const step = shift ? NUDGE_SHIFT_M : NUDGE_M;
  switch (key) {
    case "ArrowLeft":
      return movedFrame(frame, -step, 0, lengthM, heightM);
    case "ArrowRight":
      return movedFrame(frame, step, 0, lengthM, heightM);
    case "ArrowUp":
      return movedFrame(frame, 0, step, lengthM, heightM);
    case "ArrowDown":
      return movedFrame(frame, 0, -step, lengthM, heightM);
    default:
      return null;
  }
}

/**
 * A new window's rectangle when it is added without drawing: 1.2 m × 1.5 m,
 * or what the facade has room for, centred along it with its sill at 0.9 m.
 */
export function defaultFrame(
  lengthM: number,
  heightM: number,
): FacadeWindowFrame {
  const width = Math.min(1.2, snap(lengthM / 2));
  const height = Math.min(1.5, snap(heightM / 2));
  return {
    u_m: snap((lengthM - width) / 2),
    sill_z_m: Math.max(0, Math.min(0.9, snap(heightM - height))),
    width_m: width,
    height_m: height,
  };
}

/** Whether two frames are the same rectangle, to the snapping grid. */
export function sameFrame(a: FacadeWindowFrame, b: FacadeWindowFrame): boolean {
  return (
    Math.abs(a.u_m - b.u_m) < 1e-9 &&
    Math.abs(a.sill_z_m - b.sill_z_m) < 1e-9 &&
    Math.abs(a.width_m - b.width_m) < 1e-9 &&
    Math.abs(a.height_m - b.height_m) < 1e-9
  );
}
