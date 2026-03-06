# Geo Library Evaluation (Phase 1)

Status date: 2026-03-06

## Decision Goal

Select a portability-first geometry stack for early phases (import validation, basic geometry utilities, and spatial indexing) with clear licensing.

## Candidate Libraries (Go)

1. `github.com/paulmach/orb`
- Strengths: simple geometry types, GeoJSON utilities, practical ergonomics.
- Risks: not a full GIS engine; advanced operations may require complementary libraries.

2. `github.com/twpayne/go-geom`
- Strengths: robust geometry data model, WKB/WKT ecosystem support.
- Risks: additional complexity for simple workflows.

3. `github.com/tidwall/rtree` or equivalent R-tree implementation
- Strengths: lightweight spatial indexing for candidate pruning.
- Risks: must benchmark against project workload before locking choice.

## Phase 1 Recommendation

- Use a pure-Go geometry + index stack first (portability and no cgo requirement).
- Start with simple primitives and import validation needs.
- Re-evaluate before advanced GIS features (topology, contouring, heavy raster operations).

## Acceptance Criteria for Final Selection (Phase 4-6)

- License compatibility documented.
- Performance baseline measured on representative scenarios.
- Edge-case behavior validated with unit and fuzz tests.
