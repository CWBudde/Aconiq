# Phase 20 Reporting v1 (Offline)

Status date: 2026-03-06

This phase introduces reproducible offline report generation from exported run artifacts.

## Implemented

- `noise export` now assembles a richer bundle:
  - run log (`run.log`)
  - run provenance (`provenance.json`)
  - run result artifacts under `results/` (receiver table, raster metadata/binary, run summary)
  - model dump copy (when available) under `model/model.dump.json`
- Report generation package: `backend/internal/report/reporting`
  - deterministic context assembly from bundle artifacts
  - report templating outputs:
    - `report-context.json`
    - `report.md`
    - `report.html`
- Required report sections are included in both Markdown and HTML:
  - Input overview
  - Standard ID + version/profile + parameters
  - Maps/images (from raster metadata, when available)
  - Tables (receiver indicator statistics)
  - QA status (suite list; baseline fallback when no suite artifacts exist)
- Export summary (`export-summary.json`) now records generated report files.
- Project manifest now records report artifacts:
  - `export.report_context_json`
  - `export.report_markdown`
  - `export.report_html`

## Notes

- This phase delivers the "HTML-only MVP" branch of Phase 20.
- PDF rendering is intentionally deferred to a dedicated Typst phase (`Phase 20b`) in `PLAN.md`.
