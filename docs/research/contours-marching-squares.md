# Contours / Isolines (Marching Squares): Evaluation

Status date: 2026-03-06

## Question

How should contour/isoline generation be introduced for raster outputs?

## Baseline Approach

- Use Marching Squares over a regular grid.
- Keep contour generation separated from normative level calculations.
- Treat generated isolines as derived visualization artifacts.

## Selection Criteria for Library/Implementation

- Pure Go preferred for portability.
- Deterministic output ordering.
- Support for explicit no-data handling.
- Control over level definitions and tolerance behavior.

## Phase 6 Status

- Decision deferred to a later phase where contour export is implemented.
- Criteria documented now to keep implementation and QA aligned.
