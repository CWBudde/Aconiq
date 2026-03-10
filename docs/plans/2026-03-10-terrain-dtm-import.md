# Terrain/DTM Import Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Import GeoTIFF elevation data and wire it into the noise propagation pipeline for receiver/source elevation draping and terrain-aware path profiling.

**Architecture:** A `terrain` package under `internal/geo/` provides a `Model` interface with bilinear-interpolated elevation queries. GeoTIFF files are imported via `noise import --terrain`, stored as `.noise/model/terrain.tif`, and registered in the project manifest. At run time, the engine loads the terrain model and passes it to standards modules via their PropagationConfig, enabling automatic receiver/source elevation draping.

**Tech Stack:** `github.com/gden173/geotiff` (pure Go, no cgo), existing `modelgeojson` normalization pipeline, existing `PropagationConfig.ReceiverTerrainZ` field.

---

## Task 1: Add geotiff dependency

**Files:**

- Modify: `backend/go.mod`

**Step 1:** Add the dependency.

```bash
cd backend && go get github.com/gden173/geotiff@latest
```

**Step 2:** Tidy.

```bash
go mod tidy
```

---

## Task 2: TerrainModel interface and GeoTIFF loader

**Files:**

- Create: `backend/internal/geo/terrain/terrain.go`
- Create: `backend/internal/geo/terrain/terrain_test.go`

**Step 1: Write the failing tests**

Test file `terrain_test.go` should cover:

- `TestLoad_FileNotFound` — error on missing file
- `TestLoad_ValidGeoTIFF` — loads a real GeoTIFF, checks bounds are non-zero
- `TestElevationAt_InsideBounds` — returns valid elevation for a point inside the grid
- `TestElevationAt_OutsideBounds` — returns `false` for a point outside bounds
- `TestElevationAt_BilinearInterpolation` — verifies interpolated value between known grid cells

For test data: create a small synthetic GeoTIFF in `testdata/` using a helper or commit a minimal binary fixture.

**Step 2: Implement `terrain.go`**

```go
package terrain

// Model provides elevation queries over a terrain surface.
type Model interface {
    // ElevationAt returns the interpolated elevation at (x, y) in the
    // terrain's native CRS. The second return value is false when the
    // point falls outside the terrain bounds.
    ElevationAt(x, y float64) (float64, bool)

    // Bounds returns [minX, minY, maxX, maxY] in the terrain's native CRS.
    Bounds() [4]float64
}
```

Implementation `GeoTIFFModel`:

- `Load(path string) (Model, error)` — opens GeoTIFF via `geotiff.Read()`, validates single-band float data, returns `GeoTIFFModel`
- `ElevationAt(x, y)` — delegates to `gtiff.AtCoord(x, y, true)` (bilinear=true), promotes float32→float64
- `Bounds()` — reads from `gtiff.Bounds()`, converts CornerCoordinates to [4]float64

**Step 3:** Run tests, verify pass.

**Step 4:** Commit: `feat(terrain): add terrain.Model interface and GeoTIFF loader`

---

## Task 3: CLI `--terrain` flag on import command

**Files:**

- Modify: `backend/internal/app/cli/import.go`

**Step 1:** Add `--terrain` string flag to `newImportCommand()`, wired into `runImport`.

**Step 2:** Implement `runTerrainImport()`:

1. Validate the file exists and has `.tif`/`.tiff` extension
2. Load via `terrain.Load()` to validate it's a readable GeoTIFF elevation grid
3. Copy the file to `.noise/model/terrain.tif` (overwrite if exists)
4. Register artifact in manifest:
   - ID: `"artifact-terrain"`
   - Kind: `"model.terrain_geotiff"`
   - Path: `.noise/model/terrain.tif`
5. Print summary: bounds, pixel scale, grid dimensions

**Step 3:** Allow `--terrain` to be used standalone or alongside `--input`.

**Step 4:** Run `go test ./internal/app/cli/...` — existing tests must still pass.

**Step 5:** Commit: `feat(cli): add --terrain flag to noise import for GeoTIFF DTM`

---

## Task 4: Terrain loading at run time

**Files:**

- Modify: `backend/internal/app/cli/run.go`

**Step 1:** In the run command, after loading the project manifest, check for the `artifact-terrain` artifact. If found, load the terrain model:

```go
var terrainModel terrain.Model
if terrainPath := findArtifactPath(proj, "artifact-terrain"); terrainPath != "" {
    tm, err := terrain.Load(filepath.Join(store.Root(), terrainPath))
    if err != nil {
        // log warning, continue without terrain
    }
    terrainModel = tm
}
```

**Step 2:** Define a helper `findArtifactPath(proj, id) string` that scans `proj.Artifacts` for a matching ID and returns its path.

**Step 3:** Pass `terrainModel` to the propagation config builder (see Task 5).

**Step 4:** Commit: `feat(run): load terrain model from project artifacts at run time`

---

## Task 5: Wire terrain into RLS-19 propagation

**Files:**

- Modify: `backend/internal/app/cli/run.go` (RLS-19 run section)

**Step 1:** When building the `rls19road.PropagationConfig`, if `terrainModel != nil`:

- Query `terrainModel.ElevationAt(receiver.X, receiver.Y)` for each receiver
- Set `propagationConfig.ReceiverTerrainZ` per-receiver (this requires changing from a single config to per-receiver configs, OR setting a representative value)

**Design note:** Currently `PropagationConfig.ReceiverTerrainZ` is a single float64 applied to all receivers. For grid-based runs this is a simplification. The proper integration would be to query terrain per-receiver inside the compute loop. For the initial implementation, set `ReceiverTerrainZ` per compute call if the module supports it, or leave it as a project-wide default from the terrain at the grid center.

**Step 2:** For source elevation draping: if a `RoadSource.ElevationM` is 0 (unset) and terrain is available, query terrain at the source midpoint and set `ElevationM`.

**Step 3:** Add a test in the CLI test suite that verifies terrain data flows through to propagation config.

**Step 4:** Commit: `feat(run): wire terrain elevation into RLS-19 propagation config`

---

## Task 6: Wire terrain into other standards (CNOSSOS, Schall 03, ISO 9613)

**Files:**

- Modify: `backend/internal/app/cli/run.go` (other standard run sections)

**Step 1:** For each standard that has a `ReceiverHeightM` or equivalent terrain parameter in its PropagationConfig, apply the same pattern: if terrain is loaded, query elevation at receiver/source positions and populate the config.

Standards with terrain-relevant config fields:

- `cnossos/road`, `cnossos/rail`, `cnossos/industry` — ground attenuation
- `schall03` — receiver elevation
- `iso9613` — ground/terrain effect

**Step 2:** Standards that don't yet have terrain fields in their PropagationConfig get no changes — terrain is additive, not breaking.

**Step 3:** Commit: `feat(run): wire terrain elevation into remaining standards modules`

---

## Task 7: Update PLAN.md and add integration test

**Files:**

- Modify: `PLAN.md` — mark Terrain/DTM import as done
- Add: integration test in CLI test suite with a synthetic GeoTIFF + GeoJSON model that verifies end-to-end terrain-aware import and run

**Step 1:** Update PLAN.md checklist.

**Step 2:** Add a golden test with a small synthetic terrain + model that demonstrates elevation draping in output.

**Step 3:** Commit: `feat(terrain): mark terrain/DTM import complete in PLAN.md`
