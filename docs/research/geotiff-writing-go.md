# GeoTIFF Writing in Go: Evaluation

Status date: 2026-03-06

## Question

Should v1 use GeoTIFF directly or a custom binary + JSON raster container?

## Option 1: Pure Go GeoTIFF writers

Pros:

- No cgo runtime dependency.
- Better cross-platform build simplicity.

Cons:

- Feature coverage and geospatial metadata handling may be limited depending on library.
- Risk of edge-case compatibility gaps with professional GIS tooling.

## Option 2: GDAL via cgo

Pros:

- Mature and widely interoperable geospatial stack.
- Strong format support and metadata fidelity.

Cons:

- cgo packaging complexity across Linux/macOS/Windows.
- Heavier CI and release burden.

## Phase 6 Decision

For v1, use custom binary + JSON metadata.

Reason:

- Keeps portability high during CLI-first stages.
- Allows deterministic container semantics while GeoTIFF dependency strategy remains open.

## Revisit Trigger

Revisit when:

- External interoperability requirements prioritize direct GeoTIFF output.
- Dependency strategy and packaging constraints are explicitly accepted.
