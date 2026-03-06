# GeoJSON Schema v1 (Phase 4)

Status date: 2026-03-06

This schema is the minimal common input for `noise import` / `noise validate`.

## Container

- GeoJSON `FeatureCollection`.
- Each item must be a GeoJSON `Feature` with `properties` and `geometry`.

## Required Properties

- `id` (string or numeric, normalized to string)
- `kind` (string): one of
  - `source`
  - `building`
  - `barrier`

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

`noise import` writes:
- `.noise/model/model.normalized.geojson`
- `.noise/model/model.dump.json`
- `.noise/model/validation-report.json`
