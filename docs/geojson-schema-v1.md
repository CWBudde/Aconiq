# GeoJSON Schema v1 (Phase 4)

Status date: 2026-03-06

This schema is the minimal common input for `aconiq import` / `aconiq validate`.

## Container

- GeoJSON `FeatureCollection`.
- Each item must be a GeoJSON `Feature` with `properties` and `geometry`.

## Required Properties

- `id` (string or numeric, normalized to string)
- `kind` (string): one of
  - `source`
  - `building`
  - `barrier`
  - `receiver`

### `source` Features

- `source_type` required: `point` | `line` | `area`
- Geometry compatibility:
  - `point` -> `Point` or `MultiPoint`
  - `line` -> `LineString` or `MultiLineString`
  - `area` -> `Polygon` or `MultiPolygon`

### `building` Features

- `height_m` required and `> 0`
- Geometry must be `Polygon` or `MultiPolygon`

### `barrier` Features

- `height_m` required and `> 0`
- Geometry must be `LineString` or `MultiLineString`

### `receiver` Features

- `height_m` required and `> 0`
- Geometry must be `Point`
- Optional for 16. BImSchV traffic-noise assessment on explicit receivers:
  - `bimschv16_area_category`
  - accepted examples:
    - `allgemeines Wohngebiet`
    - `Mischgebiet`
    - `Gewerbegebiet`
    - `Krankenhaus`
  - the current assessment/export slice uses explicit receiver IDs plus this
    property to compare `LrDay`/`LrNight` against the legal threshold table

## Standard-Specific Geometry Conventions

The normalized model stays standard-agnostic, but some standards consume extra
geometry conventions from feature properties.

### RLS-19 Road

- Line sources may use `LineString` / `MultiLineString` coordinates in either
  `2D` (`[x, y]`) or `3D` (`[x, y, z]`) form.
- A source feature may optionally provide `lane_count` (or imported OSM `lanes`)
  to derive the normative source line automatically from the reference line.
  The current implementation applies the Bild 6 lane-count placement rules with
  right-hand traffic and a default 3.5 m lane width.
- When `3D` coordinates are present, the vertex `z` values are mapped to
  per-vertex road elevations for rising / descending roads.
- Alternatively, a source feature may provide:
  - `elevation_m`: one uniform elevation for the whole line, or
  - `centerline_elevations`: one elevation value per line vertex.
- For explicit per-direction modeling inside one source feature, RLS-19 also
  accepts `properties.rls19_directional_sources`, an array of objects where
  each entry defines one directional line source with:
  - `centerline` or `coordinates`
  - optional `id` / `direction_id` / `direction`
  - optional `lane_count`
  - optional per-direction acoustic overrides such as `traffic_day_*`,
    `traffic_night_*`, `speed_*_kph`, `surface_type`, `gradient_percent`,
    `junction_type`, `junction_distance_m`, `reflection_surcharge_db`,
    `elevation_m`, and `centerline_elevations`
- If directional sources would resolve to different `surface_type` values, the
  input must already be harmonized to a single shared surface choice that
  reflects the larger per-direction correction.

### Schall 03 Rail

Schall 03 has two computation chains, and the properties are namespaced so they
cannot be confused. The `rail_*` properties feed the non-normative preview data
pack only; the `schall03_*` properties below feed the normative Anlage-2 chain.
A run resolves to the normative chain when at least one `source` feature carries
`schall03_operations`, and fails otherwise unless the preview chain is opted
into with `--param schall03_engine=preview`. See
`docs/conformance/schall03-konformitaetserklaerung.md`.

On a `source` feature with `source_type: line`:

- `schall03_operations` (**required** for the normative chain): a non-empty
  array, one entry per train type operating on the segment. Each entry gives
  either
  - `zugart`: a name from Beiblatt 1 (Eisenbahn, Tabelle 4) or Beiblatt 2
    (Straßenbahn), which supplies both the Fz composition and a default speed —
    e.g. `ICE-3-Vollzug`, `Nahverkehrszug-ET`, `Gueterzug-E-Lok`,
    `Niederflur-ET`; or
  - `fz_composition`: an array of `{ "fz": <Fz-Kategorie>, "count": <n> }`,
    where `fz` is 1–10 for Eisenbahn and 21–23 for Straßenbahn vehicles;

  plus `trains_per_hour_day` and `trains_per_hour_night`, an optional
  `speed_kph` (required when no `zugart` supplies one; it overrides the Zugart
  default when both are present), and an optional `train_type` label.

- `schall03_strecke_max_kph` (**required**): Streckenhöchstgeschwindigkeit in
  km/h. The effective speed per Nr. 4.3 / Nr. 5.3.2 is derived from this and the
  operation speed.
- `schall03_fahrbahn`: Eisenbahn track type (Tabelle 7) — `schwellengleis`
  (default, the reference type carrying no correction), `feste-fahrbahn`,
  `feste-fahrbahn-mit-absorber`, `bahnuebergang`.
- `schall03_s_fahrbahn`: Straßenbahn track type (Tabelle 15) —
  `schwellengleis` (default), `strassenbuendig`, `begruent-tief`,
  `begruent-hoch`.
- `schall03_surface`: active surface measure (Tabelle 8) — `none` (default),
  `bug`, `schienenstegdaempfer`, `schienenstegabschirmung`.
- `schall03_bridge_type`: 0 (default, no bridge) to 4, per Tabelle 9
  (Eisenbahn) or Tabelle 16 (Straßenbahn).
- `schall03_bridge_mitigation`: boolean, K_LM noise reduction on the bridge.
- `schall03_curve_radius_m`: curve radius in metres; 0 (default) means straight.
- `schall03_is_station`: boolean, Nr. 4.3 Personenbahnhof/Haltepunkt — raises
  the effective Eisenbahn speed to at least 70 km/h.
- `schall03_permanently_slow`: boolean, Nr. 5.3.2 exception for Straßenbahn
  sections permanently at v ≤ 30 km/h.
- `schall03_water_body_fraction`: 0–1, the share of the source–receiver path
  crossing water (Gl. 16).
- `elevation_m`: track elevation, shared with the preview path.

On a `barrier` feature (`LineString`, `height_m` required): each consecutive
vertex pair becomes one barrier panel of that height.

- `schall03_reflective`: boolean, default false. A reflective barrier is
  additionally treated as a reflecting wall, and Gl. 20's D_refl applies.
- `schall03_base_height_m`: height of the absorbing Sockel, read only when the
  barrier is reflective (Gl. 20).
- `schall03_thickness_m` and `schall03_parallel_edges`: for wide barriers with
  double diffraction (Gl. 22, Gl. 25).
- `schall03_wall_surface`: Tabelle 18 category used when the barrier reflects —
  `hard` (default), `building`, `absorbing`, `highly-absorbing`.

On a `building` feature (`Polygon`, `height_m` required):

- `schall03_reflecting_wall`: boolean, default false. When true, each outer-ring
  edge becomes a reflecting wall; `schall03_wall_surface` then defaults to
  `building`. Buildings are **not** treated as shielding obstacles, so this is
  opt-in — see the conformance declaration for why.

## Geometry Sanity Checks

- Coordinates must be finite numbers.
- `LineString` must have at least 2 points.
- Polygon rings must have at least 4 points and be closed.
- Basic self-intersection checks are applied to lines and rings.

## CRS Plausibility Checks

Validation uses project CRS from `.noise/project.json`.

- Geographic CRS (for example EPSG:4326) enforces lon/lat bounds.
- Projected CRS with lon/lat-like bounds emits a mismatch warning.

## Debug Exports

`aconiq import` writes:

- `.noise/model/model.normalized.geojson`
- `.noise/model/model.dump.json`
- `.noise/model/validation-report.json`
