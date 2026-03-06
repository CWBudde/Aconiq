# Risk Register

Status date: 2026-03-06

| ID | Risk | Likelihood | Impact | Mitigation | Owner | Status |
|---|---|---|---|---|---|---|
| R-001 | CRS/projection accuracy vs portability tradeoff (pure Go vs PROJ/cgo) causes platform divergence | Medium | High | Decide CRS strategy in early phases; add cross-platform numeric comparisons | TBD | Open |
| R-002 | Paywalled or redistribution-restricted standards/test packs block automated conformance | High | High | Separate public vs private acceptance suites; keep legal provenance notes | TBD | Open |
| R-003 | Wails v3 maturity risk for desktop packaging | Medium | Medium | Keep Wails optional/deferred; preserve API-first architecture | TBD | Open |
| R-004 | Deterministic parallel reduction is harder than expected for large runs | Medium | High | Define deterministic reduction policy and test 1 vs N worker equivalence early | TBD | Open |
| R-005 | Geo stack dependency licensing or maintenance issues | Medium | Medium | Evaluate libraries with license/maintenance criteria before adoption | TBD | Open |
| R-006 | Cross-platform filesystem/path differences break project portability | Medium | Medium | Normalize paths and add Windows/macOS/Linux smoke tests | TBD | Open |
| R-007 | Performance targets for city-scale runs are missed without early benchmarking | Medium | High | Introduce benchmark scenarios and tracking before normative modules scale up | TBD | Open |
| R-008 | Input data quality (invalid geometries/CRS metadata) causes silent calculation errors | High | High | Build strict validation pipeline and reject ambiguous imports | TBD | Open |

## Update Policy

- Add new risks when discovered.
- Re-score likelihood/impact at phase boundaries.
- Close risks only with evidence (test coverage, documented decision, or benchmark proof).
