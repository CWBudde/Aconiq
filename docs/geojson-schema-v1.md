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
