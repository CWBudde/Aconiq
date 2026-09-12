// Planar geometry shared by the domain model and the compute kernel.
//
// Field names match Go's JSON serialization exactly; the kernel transport in
// src/wasm/types.ts re-exports this type rather than defining its own so the
// dependency runs wasm → model, never the other way round.

export interface Point2D {
  x: number;
  y: number;
}
