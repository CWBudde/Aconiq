// TypeScript mirror of the Go RLS-19 road types.
// Field names match Go's JSON serialization exactly.
// Structs without json tags use PascalCase (Go's default).

import type { Point2D } from "@/model/geometry";
import type {
  ParkingFacilityType,
  ParkingLotType,
  ParkingSource,
} from "@/model/rls19-parking-types";

// Point2D and the parking vocabulary are domain types owned by src/model/ and
// re-exported here so kernel-facing callers keep a single import site. The
// dependency runs wasm → model, never the other way round.
export type { ParkingFacilityType, ParkingLotType, ParkingSource, Point2D };

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

/**
 * Which CRS a request computes in, as the kernel reads it.
 *
 * The same fact `@/model/compute-crs`'s `ComputeProjection` carries, in the
 * snake_case every CRS on a wire in this project is spelled with —
 * {@link TransformRequest}'s `source_crs`, a run summary's `compute_crs`, a
 * run's provenance. Mirrors `wasmkernel.ComputeProjection`.
 *
 * The kernel cannot work this out for itself. `aconiq run` reads the project
 * CRS off the manifest; browser mode projects the model here, in TypeScript,
 * and hands the kernel coordinates that are already metric — so by the time a
 * request arrives, nothing in it names a CRS. A terrain model has to be queried
 * in the CRS its raster was written in, and this is the only thing that says
 * what to transform through.
 */
export interface KernelProjection {
  project_crs: string;
  compute_crs: string;
  applied: boolean;
}

export interface ComputeRequest {
  receivers: PointReceiver[];
  sources: RoadSource[];
  barriers: Barrier[];
  config?: PropagationConfig;
  /**
   * Optional, and required the moment a terrain model is loaded: a request that
   * computes over a DTM without saying which CRS it computes in is refused by
   * the kernel rather than answered from a grid queried in the wrong one.
   */
  projection?: KernelProjection;
}

/**
 * What `aconiq.loadTerrain` answers with — the grid it just took, described in
 * the terrain's own CRS. Mirrors Go's `terrain.Info`.
 *
 * The bounds are deliberately *not* converted into the CRS a run computes in:
 * the projection of a rectangle is not a rectangle, so any four numbers would
 * be too large or too small somewhere.
 */
export interface TerrainInfo {
  /** [minX, minY, maxX, maxY], in the CRS the DTM was declared in. */
  bounds: [number, number, number, number];
  pixel_size: [number, number];
  grid_size: [number, number];
}

/**
 * Asks the kernel to make the same CRS decision `aconiq run` makes: project a
 * geographic model into the ETRS89 / UTM zone its centre falls in, and leave a
 * model that is already metric exactly where it is.
 *
 * Mirrors `wasmkernel.AutoTargetCRS`.
 */
export const AUTO_TARGET_CRS = "auto";

/**
 * A batch of coordinates to move between two CRS.
 *
 * Coordinates are flat and interleaved — x0, y0, x1, y1, … — not nested pairs:
 * a per-point crossing of the WASM boundary is not viable for a model of any
 * size, so the call is batched, and flat halves the JSON of the batch.
 *
 * `target_crs` is {@link AUTO_TARGET_CRS} to let the kernel decide, or an
 * explicit `EPSG:nnnn` to transform unconditionally — which is what makes the
 * inverse direction free.
 */
export interface TransformRequest {
  source_crs: string;
  target_crs: string;
  coordinates: number[];
}

/**
 * Where the coordinates ended up. `target_crs` is the CRS they are actually in,
 * which for an `auto` request is the resolved zone. `applied` is false only
 * when nothing moved, and then `coordinates` are the input values verbatim.
 */
export interface TransformResponse {
  source_crs: string;
  target_crs: string;
  applied: boolean;
  coordinates: number[];
}
