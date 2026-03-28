# SoundPLAN Import Findings

Status: working note for Priority 6. This document records the currently verified file coverage, the remaining gaps, and the staging plan for `noise import --from-soundplan`.

## Sample project in repo

Reference fixture:

- `interoperability/Schienenprojekt - Schall 03/`

Observed files in that fixture:

- `Project.sp`
- `*.res`
- `GeoRail.geo`
- `GeoObjs.geo`
- `GeoWand.geo`
- `GeoTmp.geo`
- `CalcArea.geo`
- `Höhen.txt`
- `TS03.abs`
- result subdirectories such as `RSPS0011/` and `RRLK0022/`
- auxiliary files such as `*.dgm`, `*.ntd`, `*.ets`, `*.esn`, `*.ess`

## Implemented parser coverage

`backend/internal/io/soundplanimport/` currently covers:

- `Project.sp`: Windows-1252 INI parsing, project metadata, enabled standards, time slices, selected road/rail/industry standards, geometry defaults, receiver/grid defaults.
- `*.res`: run metadata, statistics, geometry references, warnings, assessment windows.
- `GeoRail.geo`: rail centerlines, segment splitting on parameter changes, basic per-segment parameters.
- `GeoObjs.geo`: building footprints and receiver points.
- `GeoWand.geo`: barrier geometry and per-point heights.
- `GeoTmp.geo`: elevation points and contour/break lines.
- `Höhen.txt`: text fallback parser for elevation points.
- `CalcArea.geo`: calculation area polygon.
- `TS03.abs`: train-type catalog.
- `RREC*`, `RGRP*`, `RMPA*`: receiver/group/partial results via `go-absolute-database`.

There is now also a staging loader:

- `LoadProjectBundle(projectDir)` parses the supported inputs in one call and returns a bundle suitable for later CLI import wiring.

## Verified mapping of SoundPLAN standard IDs

Current Aconiq mappings:

- `20490` -> `schall03` `phase18-baseline-preview` / `rail-planning-preview`
- `10490` -> `rls19-road` `2019` / `default`
- `30000` -> `iso9613` `1996-octaveband` / `point-source`

Any other enabled SoundPLAN standard must remain non-fatal and produce an explicit warning in the importer/report.

## Remaining parser gaps

Still open at the file-format level:

- `GeoObjs.geo`: type `0x03e9` building attributes and `:D1` address/name extraction.
- `GeoWand.geo`: `:D!` material and absorption properties.
- `GeoTmp.geo` plus `.dgm`: binary digital terrain model extraction.
- `RRAI` and `RRAD`: robust emission parsing for affected SoundPLAN versions, especially the v7.61 record-layout issue.
- `RRLK*` / `RRLK*.GM`: grid raster metadata and values.
- `.ntd`: immission point table parsing.

## Recommended implementation order

The lowest-risk path is:

1. Keep building a deterministic inspection/import-preparation layer in `soundplanimport`.
2. Convert supported SoundPLAN inputs into normalized model GeoJSON plus a separate import report artifact.
3. Only after that, wire `noise import --from-soundplan` to write those artifacts into `.noise/model/`.
4. Add `noise compare` once imported models can be run through the existing standards pipeline.

## Concrete next slices

Recommended near-term tasks:

1. Add structured SoundPLAN import report JSON derived from `ProjectBundle`.
2. Decide the first CRS rule: explicit flag, sidecar metadata, or bounded auto-detection heuristics.
3. Implement the first model conversion path:
   - buildings -> `kind=building`
   - barriers -> `kind=barrier`
   - receivers -> `kind=receiver`
   - rail centerlines -> `kind=source`, `source_type=line`
4. Encode all unresolved mappings as warnings instead of silently defaulting.
5. Add an integration test that asserts bundle counts and mapped standard selection for the sample project.

## Constraints to preserve

- Deterministic ordering of imported features and warnings.
- No silent fallback from unsupported standards to a different normative engine.
- Imported defaults must be explicit in output properties or provenance so cross-validation remains auditable.
