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
/**
 * `distance_scaled` lets a source line far from the receiver be split more
 * coarsely, up to the `l_i <= s_i / 2` bound the Anmerkung to RLS-19 Nr. 3.2
 * publishes. It changes the level — by +0.066 dB at worst on the convergence
 * fixture, always upward — so it is a declared parameter, not a switch the UI
 * may flip on the user's behalf.
 */
export type SegmentLengthMode = "fixed" | "distance_scaled";

// Browser mode builds only Buildings and ParkingSources today; Terrain and
// Reflectors are mirrored because the kernel accepts them and the parity tests
// drive them, not because a model can express them yet.
export interface PropagationConfig {
  SegmentLengthM: number;
  /**
   * Which Teilstück length rule the kernel applies. Absent means "fixed",
   * because the Go field's zero value is fixed mode — so a request that has
   * never heard of this option computes exactly what it always did.
   */
  SegmentLengthMode?: SegmentLengthMode;
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

/**
 * An automatic receiver grid, named instead of listed. Mirrors
 * `wasmkernel.ReceiverGrid`, which expands it exactly as
 * `@/model/receiver-grid` does: row-major from cell (0,0), each coordinate
 * `origin + index * resolution_m` rounded twice, ids `R0001`… — so the
 * receivers the kernel computes are the ones the browser names in the run's
 * table and hash.
 */
export interface ReceiverGrid {
  origin_x: number;
  origin_y: number;
  resolution_m: number;
  width: number;
  height: number;
  height_m: number;
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
 * A run whose receivers are an automatic grid: {@link ComputeRequest} with the
 * six numbers of a {@link ReceiverGrid} in place of one object per receiver.
 * It is what `rls19RoadGrid` takes, and `estimate` takes it too.
 */
export type GridComputeRequest = Omit<ComputeRequest, "receivers"> & {
  grid: ReceiverGrid;
};

/** Either way a run can name its receivers. */
export type AnyComputeRequest = ComputeRequest | GridComputeRequest;

/** How many receivers a request names, without expanding a grid. */
export function requestReceiverCount(req: AnyComputeRequest): number {
  return "grid" in req
    ? req.grid.width * req.grid.height
    : req.receivers.length;
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
 * Which receivers stand inside a building. Mirrors
 * `wasmkernel.FootprintMaskRequest`: `points` are interleaved `x, y` pairs, as
 * {@link TransformRequest.coordinates} are, and each footprint is one polygon
 * — exterior ring first, holes after, each ring a list of `[x, y]`. Both in the
 * compute CRS.
 */
export type FootprintMaskRequest =
  | { points: number[]; footprints: [number, number][][][] }
  /** A grid run's points, as the receiver grid the run sends. */
  | { grid: ReceiverGrid; footprints: [number, number][][][] };

/** The indices into the request's points that are masked, ascending. */
export interface FootprintMaskResponse {
  masked: number[];
}

/**
 * `wasmkernel.FacadeReceiversRequest`: the facade receiver set of a run, as
 * `--facade-building`, `--facade-spacing-m`, `--facade-courtyard` and
 * `--allow-unconfirmed` ask for it. `model` is a GeoJSON FeatureCollection of
 * every building and window, already in `crs`, the run's compute CRS.
 */
export interface FacadeReceiversRequest {
  crs: string;
  model: unknown;
  standard_id: string;
  buildings: string[];
  spacing_m: number;
  include_courtyard?: boolean;
  allow_unconfirmed?: boolean;
}

/**
 * A facade a receiver stands at, in `results.ReceiverFacade`'s shape: the same
 * object a receiver table row carries (`ReceiverFacade` in `@/api/client`),
 * with `from` / `to` in the compute CRS.
 */
export interface KernelReceiverFacade {
  building_ref: string;
  part: number;
  ring: number;
  edge: number;
  from: [number, number];
  to: [number, number];
}

/**
 * A window as a rectangle in its facade's frame, in metres: the u of its left
 * edge, the height of its sill, its width and its height
 * (`geo.FacadeWindow`).
 */
export interface FacadeWindowFrame {
  u_m: number;
  sill_z_m: number;
  width_m: number;
  height_m: number;
}

/**
 * `wasmkernel.FacadeElevationRequest`, and the body of
 * `POST /api/v1/facades/elevation`: one building of `model` to unfold, by its
 * feature id. `model` is a GeoJSON FeatureCollection already in `crs`, the
 * compute CRS a facade run would use, so the facades are the edges its keys
 * are numbered on.
 */
export interface FacadeElevationRequest {
  crs: string;
  model: unknown;
  building: string;
}

/**
 * A validation finding about a window, a facade or a building, with its level.
 * `facade` names the facade of the building `feature_id` names, for a finding
 * about one facade (`facade.windows.no_candidate`).
 */
export interface ElevationFinding {
  level: "error" | "warning";
  code: string;
  feature_id?: string;
  facade?: { part: number; ring: number; edge: number };
  message: string;
}

/**
 * A window feature as the elevation draws it (`facadeset.ElevationWindow`).
 * `on_party_wall` is a window whose centre stands on a party wall of its
 * facade, which a facade run refuses while it is sensitive.
 */
export interface ElevationWindow {
  id: string;
  window: FacadeWindowFrame;
  sensitive: boolean;
  review_status: "proposed" | "confirmed" | "rejected";
  on_party_wall: boolean;
  findings: ElevationFinding[];
}

/** A stretch of a facade, in metres along it from its start (`geo.FacadeSpan`). */
export interface FacadeSpan {
  from_um: number;
  to_um: number;
}

/**
 * One facade: a canonical edge, with `from` / `to` in the request's CRS, its
 * length in metres, the bearing it faces from grid north, the stretches
 * another building stands against (where facade mode places no point), and
 * the windows keyed to it (`facadeset.FacadeElevation`).
 */
export interface FacadeElevation {
  part: number;
  ring: number;
  edge: number;
  from: [number, number];
  to: [number, number];
  length_m: number;
  facing_deg: number;
  courtyard: boolean;
  party_walls: FacadeSpan[];
  windows: ElevationWindow[];
  /** Validation's review flags about this facade (P18 D3). */
  findings: ElevationFinding[];
}

/**
 * A building unfolded into its facades (`facadeset.BuildingElevation`):
 * `slab_heights_m` are its storeys' ceiling slabs above the ground, ground
 * floor first, where facade mode places its points; `detached` are the
 * windows keyed to it whose key names no edge of it as it is now
 * (`window.facade.moved`). A window whose key names no building of the model
 * is in no building's elevation.
 */
export interface FacadeElevationResponse {
  building_ref: string;
  height_m: number;
  levels?: number;
  slab_heights_m: number[];
  slab_height_source: "measured" | "levels" | "assumed";
  facades: FacadeElevation[];
  detached: ElevationWindow[];
  /** Validation's review flags about the building as a whole (P18 D3). */
  findings: ElevationFinding[];
}

/**
 * A window as the elevation view draws or moves it
 * (`facadeset.WindowPlacement`): the edge it is on, as the elevation listed
 * it, its rectangle in that facade's frame, and the review fields. An absent
 * `review_status` is `proposed`.
 */
export interface WindowPlacement {
  id: string;
  building_ref: string;
  part: number;
  ring: number;
  edge: number;
  window: FacadeWindowFrame;
  sensitive: boolean;
  review_status?: ElevationWindow["review_status"];
  evidence?: unknown[];
}

/**
 * `wasmkernel.PlaceWindowRequest`, and the body of
 * `POST /api/v1/facades/window`: the model holding the window's building, in
 * `crs` — the CRS the elevation was asked in, so `part` / `ring` / `edge`
 * name the edge it showed.
 */
export interface PlaceWindowRequest {
  crs: string;
  model: unknown;
  window: WindowPlacement;
}

/** The window feature the model stores, in the request's CRS. */
export interface PlaceWindowResponse {
  feature: {
    type: "Feature";
    id?: string;
    properties: Record<string, unknown>;
    geometry: { type: "Point"; coordinates: [number, number] };
  };
}

/**
 * `facadeset.PhotoTransform`, and the body of `POST /api/v1/facades/rectify`:
 * the four facade corners clicked in a photo, in pixels with y running down —
 * bottom-left, bottom-right, top-right, top-left, seen from outside — and
 * where they are in the facade's frame, `[u, z]` in metres. It is also the
 * `transform` a window's photo evidence records (P18 D1).
 */
export interface PhotoTransform {
  image_points: [number, number][];
  facade_points: [number, number][];
}

/**
 * `facadeset.Rectification`: the homography a {@link PhotoTransform} solves
 * to, row-major, from a pixel to `(u, z)` on the facade (`matrix`) and back
 * (`inverse`), scaled so that w is positive over the photographed facade.
 */
export interface Rectification {
  matrix: number[];
  inverse: number[];
}

/**
 * One generated facade receiver and the attributes its table row carries. A
 * point placed at a window names the window and what its height was derived
 * from, and its `slab_height_source` is empty: the window fixes the height.
 */
export interface FacadeReceiver extends PointReceiver {
  placement_rule: string;
  facade: KernelReceiverFacade;
  storey: number;
  slab_height_source: string;
  review_status: string;
  height_basis?: string;
  window_id?: string;
  window?: FacadeWindowFrame;
}

/**
 * The generated set, in the order a run computes it, with the counts and the
 * provenance keys `aconiq run` records for it.
 */
export interface FacadeReceiversResponse {
  receivers: FacadeReceiver[];
  proposed_count: number;
  assumed_height_count: number;
  metadata: Record<string, string>;
}

/**
 * `wasmkernel.CustomReceiversRequest`: the receivers a custom run would compute
 * at, as the browser placed them in model order, and the model they stand in —
 * buildings and receivers with their properties, already in `crs`, the run's
 * compute CRS. `allow_unconfirmed` is `--allow-unconfirmed`.
 */
export interface CustomReceiversRequest {
  crs: string;
  model: unknown;
  standard_id: string;
  receivers: PointReceiver[];
  allow_unconfirmed?: boolean;
}

/**
 * One receiver a custom run keeps, with the facade attributes its table row
 * carries — present only as the complete set a row accepts.
 */
export interface CustomReceiver extends PointReceiver {
  placement_rule?: string;
  facade?: KernelReceiverFacade;
  storey?: number;
}

/**
 * The kept set, in the order a run computes it — rejected points dropped —
 * with the counts and the provenance keys `aconiq run` records for it.
 */
export interface CustomReceiversResponse {
  receivers: CustomReceiver[];
  proposed_count: number;
  assumed_height_count: number;
  rejected_count: number;
  metadata: Record<string, string>;
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

/**
 * Where a raster's cells sit on the ground. Mirrors `results.Georeference`.
 *
 * `origin_x`/`origin_y` is the *centre* of cell (0,0), not a corner: a grid
 * receiver is a point in the middle of the cell it stands for. `row_order` is
 * always `"south-up"` — row 0 holds the southernmost cells — and the Go side
 * refuses any other value rather than guessing, because a north-up raster read
 * as south-up is a vertically mirrored noise map that looks entirely plausible.
 */
export interface RasterGeoreference {
  origin_x: number;
  origin_y: number;
  pixel_size_m: number;
  row_order: string;
}

/**
 * A run's raster sidecar. Mirrors `results.RasterMetadata`.
 *
 * This is the parsed `.json` the run wrote, and browser mode hands it back
 * untouched beside the `.bin` bytes rather than reconstructing it: the shape,
 * the nodata value, the band names and the georeference have to be the ones
 * the run recorded. The sidecar carries a few further bookkeeping keys
 * (`data_file`, `encoding`, `cell_count`) that describe a file on disk; the
 * kernel ignores them, and passing the whole object through is fine.
 */
export interface RasterMetadata {
  width: number;
  height: number;
  bands: number;
  nodata: number;
  /** Per band, keyed by band name — the kernel passes it through untouched. */
  units: Record<string, string>;
  band_names?: string[];
  crs?: string;
  georeference?: RasterGeoreference;
}

/**
 * What to trace, and where to put the result. Mirrors
 * `wasmkernel.ContourRequest`.
 *
 * The raster *values* are not in here — they travel beside this JSON as a
 * `Uint8Array`, because a 500x500 two-band grid is four megabytes of float64
 * and rendering those as JSON numbers would cost more than the tracing does.
 *
 * `target_crs` is required, and unlike {@link TransformRequest} it has no
 * `auto`: a map asking for contours already knows which CRS it draws in.
 *
 * `interval` defaults to 5 dB (the EU END convention) when omitted.
 * `min_level` and `max_level` default to the raster's own data range.
 */
export interface ContourRequest {
  raster: RasterMetadata;
  target_crs: string;
  interval?: number;
  min_level?: number;
  max_level?: number;
}

/** One contour line at one dB level. Mirrors `contour.Line`. */
export interface ContourLine {
  level: number;
  band_name: string;
  /** Vertices as `[x, y]` pairs, in {@link ContourResult.crs}. */
  points: [number, number][];
}

/**
 * The traced contours and the CRS they are in. Mirrors `contour.Result`.
 *
 * `crs` is always populated, because a consumer handed bare lines guesses
 * WGS84 — which for a metric run puts the site off the coast of Africa.
 * `interval` is echoed for the same reason: the caller may have omitted it and
 * taken the default, and a legend naming the step has to know which it got.
 *
 * `lines` is always an array, never `null`. `contour.FromRaster` substitutes an
 * empty slice before returning, precisely so that a raster with no valid data
 * in any band does not marshal as `"lines": null` over both boundaries and
 * leave every consumer to defend against it separately.
 */
export interface ContourResult {
  crs: string;
  interval: number;
  lines: ContourLine[];
}
