// TypeScript mirror of the Go RLS-19 road types.
// Field names match Go's JSON serialization exactly.
// Structs without json tags use PascalCase (Go's default).

export interface Point2D {
  x: number;
  y: number;
}

export interface PointReceiver {
  id: string;
  point: Point2D;
  height_m: number;
}

export interface TrafficInput {
  pkw_per_hour: number;
  lkw1_per_hour: number;
  lkw2_per_hour: number;
  krad_per_hour: number;
}

export interface SpeedInput {
  pkw_kph: number;
  lkw1_kph: number;
  lkw2_kph: number;
  krad_kph: number;
}

export interface Barrier {
  id: string;
  geometry: Point2D[];
  height_m: number;
}

// SurfaceType string values from Go constants.
export type SurfaceType =
  | "" // not specified
  | "SMA"
  | "AB"
  | "OPA"
  | "Pflaster"
  | "Beton"
  | "LOA"
  | "DSH-V"
  | "Gussasphalt"
  | "beschaedigt";

// JunctionType is a Go int — serializes as a number (0=none, 1=signalized, 2=roundabout, 3=other).
export type JunctionType = 0 | 1 | 2 | 3;
export const JunctionNone = 0 as JunctionType;
export const JunctionSignalized = 1 as JunctionType;
export const JunctionRoundabout = 2 as JunctionType;
export const JunctionOther = 3 as JunctionType;

export interface RoadSource {
  id: string;
  centerline: Point2D[];
  surface_type: SurfaceType;
  speeds: SpeedInput;
  gradient_percent?: number;
  junction_type?: JunctionType;
  junction_distance_m?: number;
  reflection_surcharge_db?: number;
  traffic_day: TrafficInput;
  traffic_night: TrafficInput;
}

// PropagationConfig has no json tags in Go → PascalCase keys.
export interface PropagationConfig {
  SegmentLengthM: number;
  MinDistanceM: number;
  ReceiverHeightM: number;
}

export interface ReceiverIndicators {
  lr_day: number;
  lr_night: number;
}

// ReceiverOutput has no json tags in Go → PascalCase keys.
export interface ReceiverOutput {
  Receiver: PointReceiver;
  Indicators: ReceiverIndicators;
}

export interface ComputeRequest {
  receivers: PointReceiver[];
  sources: RoadSource[];
  barriers: Barrier[];
  config?: PropagationConfig;
}
