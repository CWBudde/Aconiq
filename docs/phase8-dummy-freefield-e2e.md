# Phase 8 Dummy Freefield E2E

Status date: 2026-03-06

This phase adds an end-to-end offline execution path with a clearly non-normative demonstrator standard.

## Implemented

- Standard module: `backend/internal/standards/dummy/freefield`
  - simple free-field attenuation (`L = E - 20*log10(d)`)
  - energetic summation across sources
- CLI execution: `noise run --standard dummy-freefield`
  - loads normalized model (`.noise/model/model.normalized.geojson`)
  - extracts point/multipoint sources
  - generates a grid receiver set from source extent + padding
  - executes `backend/internal/engine`
  - persists run outputs

## Run Parameters (`--param key=value`)

- `grid_resolution_m` (default `10`)
- `grid_padding_m` (default `20`)
- `receiver_height_m` (default `4`)
- `source_emission_db` (default `90`)
- `chunk_size` (default `128`)
- `workers` (default `0` = auto)
- `disable_cache` (default `false`)

## Persisted Outputs

Under `.noise/runs/<run-id>/results`:

- `receivers.json`
- `receivers.csv`
- `ldummy.json`
- `ldummy.bin`
- `run-summary.json`

## Golden Fixture

- Model fixture: `backend/internal/app/cli/testdata/phase8/model.geojson`
- Expected snapshot: `backend/internal/app/cli/testdata/phase8-dummy-freefield.golden.json`
- Test: `backend/internal/app/cli/run_phase8_test.go`
