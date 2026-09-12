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

// SurfaceType string values from Go constants. Keep in sync with
// backend/internal/standards/rls19/road/model.go (SurfaceNotSpecified ...
// SurfaceUnpavedOrDamaged) and with RLS19_SURFACE_TYPES in
// src/model/source-acoustics.ts. Each value has its own DStrO correction row in
// rls19/road/tables.go, so omitting one here is a numeric defect, not a
// cosmetic one.
export type SurfaceType =
  | "" // not specified
  | "SMA" // generic alias: SMA 5/8 at <=60, SMA 8/11 at >60
  | "SMA-5-8"
  | "SMA-8-11"
  | "AB"
  | "OPA" // generic alias: OPA PA 11
  | "OPA-11"
  | "OPA-8"
  | "Pflaster" // generic alias: sonstiges Pflaster
  | "Pflaster-eben"
  | "Pflaster-sonstig"
  | "Beton"
  | "LOA"
  | "SMA-LA-8"
  | "DSH-V"
  | "Gussasphalt" // generic alias: laermarmer Gussasphalt
  | "Gussasphalt-nicht-geriffelt"
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

export interface Building {
  id: string;
  footprint: Point2D[];
  height_m: number;
  reflection_loss_db?: number;
}

// ParkingLotType is the RLS-19 Tabelle 6 Parkplatztyp, carried by name.
// The ordinal is a table row position and moves when the table does, so it is
// deliberately not a wire format — see parking.go's UnmarshalJSON.
export type ParkingLotType = "" | "pkw" | "motorrad" | "lkw-omnibus";

// ParkingFacilityType is the RLS-19 Tabelle 7 Parkplatztyp, which seeds the
// standard movement rates. Tabelle 7 has exactly these two rows.
export type ParkingFacilityType = "" | "park-and-ride" | "tank-rastanlage";

// ParkingSource is one RLS-19 §3.4 Parkplatzteilfläche.
//
// The movement rates are nullable on purpose: null means "not stated" and is
// refused, while an explicit 0 is a legal input meaning a period with no
// movements. They are not optional keys — a missing key must serialize as a
// visible null rather than vanish.
export interface ParkingSource {
  id: string;
  center: Point2D;
  elevation_m?: number;
  area_m2: number;
  num_spaces: number;
  parking_type: ParkingLotType;
  movements_per_space_day: number | null;
  movements_per_space_night: number | null;
}

export interface Point3D {
  x: number;
  y: number;
  z: number;
}

// TerrainEdge is a Böschungskante or Böschungsfuß — a 3D polyline.
export interface TerrainEdge {
  id: string;
  geometry: Point3D[];
}

export interface TerrainSlope {
  slope_crest: TerrainEdge;
  slope_foot?: TerrainEdge;
}

export interface TerrainProfile {
  slopes: TerrainSlope[];
}

// ReflectorType is a Go int — serializes as a number. It indexes Tabelle 8:
// 0 = unspecified (ReflectionLossDB applies, else the 0.5 dB facade row),
// 1 = schallharte Fassade oder Wand, 2 = schallabsorbierende Wand,
// 3 = stark schallabsorbierende Wand. As with JunctionType the ordinal is
// Go's own iota, not a table row this project renumbers.
export type ReflectorType = 0 | 1 | 2 | 3;

export interface Reflector {
  id: string;
  geometry: Point2D[];
  height_m: number;
  type?: ReflectorType;
  reflection_loss_db?: number;
}

// PropagationConfig has no json tags in Go → PascalCase keys.
//
// Browser mode builds only Buildings and ParkingSources today; Terrain and
// Reflectors are mirrored because the kernel accepts them and the parity tests
// drive them, not because a model can express them yet.
export interface PropagationConfig {
  SegmentLengthM: number;
  MinDistanceM: number;
  ReceiverHeightM: number;
  ReceiverTerrainZ?: number;
  Terrain?: TerrainProfile[];
  Reflectors?: Reflector[];
  Buildings?: Building[];
  ParkingSources?: ParkingSource[];
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
