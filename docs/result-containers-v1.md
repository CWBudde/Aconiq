# Result Containers v1 (Phase 6)

Status date: 2026-03-06

## Decisions

1. Raster persistence: **Option A selected**

- Custom binary payload (`float64` little-endian) + JSON metadata sidecar.
- GeoTIFF is deferred until dependency strategy is finalized.

2. Receiver tables: **CSV + JSON selected for v1**

- CSV for interoperability.
- JSON for structured debugging and machine-readable exchange.
- Parquet deferred.

## Raster Container API

Implemented in `backend/internal/report/results`:

- Metadata: width, height, bands, nodata, unit, band names
- Indexing: `At(x,y,band)`, `Set(x,y,band,value)`
- Utilities: `Fill`, `Values`, validation

Persistence files:

- `<base>.json` metadata
- `<base>.bin` binary values

## Receiver Table API

Implemented in `backend/internal/report/results`:

- `ReceiverTable` with ordered indicators and unit
- `ReceiverRecord` with coordinates, height, and per-indicator values
- Validation for duplicate IDs, required indicators, and finite numeric values
- Writers for JSON and CSV outputs

## `aconiq export` Skeleton

`aconiq export` now:

- Selects a run (latest by default or explicit `--run-id`)
- Creates export bundle directory under `.noise/exports/`
- Copies available run log + provenance files
- Writes `export-summary.json`
- Optionally emits sample raster/table files (`--emit-sample-results`)
