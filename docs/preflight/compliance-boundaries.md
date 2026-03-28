# Compliance Boundaries

Status date: 2026-03-28

## Scope Categories

### 1. Code (our implementation)

- Licensed under MIT. See `LICENSE`.
- All direct dependencies are permissive (MIT, BSD-3-Clause, BSD-2-Clause, Apache-2.0, ISC). No copyleft dependencies.
- Dependency license scanning runs in CI via `go-licenses` (`just license-check`). Restricted, forbidden, and unknown license types are gated.
- A machine-readable license report CSV is generated and uploaded as a CI artifact on every run (`just license-report`).
- The `NOTICE` file lists all third-party dependencies with license types and is maintained manually. CI scanning is the authoritative check.

### 2. Standards texts and annexes

Many normative documents are paywalled or redistribution-restricted. The following rules apply:

- The code may implement algorithms from lawfully obtained standards, but cannot embed full normative text unless redistribution rights are explicit.
- Internal references to formulas use equation numbers and short descriptions, not verbatim reproduction.
- Normative coefficient tables that qualify as amtliches Werk (official works not subject to copyright per §5 UrhG) may be embedded directly in code, with a citation comment. This applies to Schall 03 (Anlage 2 zu §4 der 16. BImSchV) and similar statutory instruments.
- For ISO and DIN standards (copyrighted works), coefficient values are derived from lawfully obtained copies and referenced by table/equation number without reproducing the full table layout.

### 3. Shipped standards modules and their compliance status

#### Normatively complete (full emission + propagation + assessment)

| Module | Standard | Conformance declaration | Notes |
|--------|----------|-------------------------|-------|
| `rls19/road` | RLS-19 (2019) | `docs/conformance/rls19-konformitaetserklaerung.md` | Incl. Korrekturblatt 2/2020. Multi-edge shielding (Eq. 16 C>0) not yet implemented. |
| `schall03` | Schall 03 (Anlage 2 zu §4 16. BImSchV) | `docs/conformance/schall03-konformitaetserklaerung.md` | All normative formulas Gl. 1-36 and tables 1-18. Section 9 measurement data deferred. |

#### Assessment layers (consume results from propagation modules)

| Module | Standard | Conformance declaration | Notes |
|--------|----------|-------------------------|-------|
| `assessment/talaerm` | TA Lärm (26.08.1998, amended 01.06.2017) | `docs/conformance/ta-laerm-konformitaetserklaerung.md` | Nr. 6.1-6.9 implemented. Nr. 7.1 (emergencies) and Nr. 7.3 (DIN 45680 low-frequency) out of scope. |
| `assessment/bimschv16` | 16. BImSchV | — | Threshold comparison and reporting for explicit receivers. Scope definition and formal conformance declaration pending. |

#### Baseline/preview modules (scaffolded, not yet conformance-claimed)

| Module | Standard | Status |
|--------|----------|--------|
| `iso9613` | ISO 9613-2 | Point-source propagation scaffold only. No conformance claim. |
| `cnossos/road` | CNOSSOS-EU Road | Baseline implementation. No conformance claim. |
| `cnossos/rail` | CNOSSOS-EU Rail | Baseline implementation. No conformance claim. |
| `cnossos/industry` | CNOSSOS-EU Industry | Baseline implementation. No conformance claim. |
| `cnossos/aircraft` | CNOSSOS-EU Aircraft | Baseline implementation. No conformance claim. |
| `bub/road` | BUB Road | German mapping of CNOSSOS Road. No conformance claim. |
| `bub/rail` | BUB Rail | German mapping of CNOSSOS Rail. No conformance claim. |
| `bub/industry` | BUB Industry | German mapping of CNOSSOS Industry. No conformance claim. |
| `buf/aircraft` | BUF Aircraft | German aircraft noise model. No conformance claim. |
| `beb/exposure` | BEB Exposure | Dwelling/population exposure aggregation. No conformance claim. |
| `dummy/freefield` | — | Non-normative demonstrator for E2E testing. |

Modules without a conformance declaration must not be presented as normatively validated to third parties.

### 4. Test and acceptance data

- Public, license-safe test data is preferred and is versioned in-repo under `testdata/` directories.
- Restricted test suites (e.g., TEST-20 for RLS-19) are stored outside the public repo and referenced by local path. CI must not depend on private datasets for required checks in public branches.
- Golden test snapshots serve as regression anchors and are updated intentionally via `just update-golden`.

### 5. Generated artifacts and examples

- Example projects in this repo must be synthetic or clearly redistribution-safe.
- No third-party basemap or dataset is vendored unless terms permit it.
- Report templates and generated PDFs use only bundled or open-source fonts and assets.

## License scanning details

### Ignored packages in `go-licenses` scanning

One package requires an `--ignore` flag due to a detection limitation, not a license concern:

| Package | Actual license | Reason for ignore |
|---------|---------------|-------------------|
| `modernc.org/mathutil` | BSD-3-Clause | License file exists but `go-licenses` fails to detect it (confidence threshold issue). |

This package is documented in the `NOTICE` file and verified manually.

### Policy

The CI license check (`just license-check`) disallows the following license types:
- **restricted** (GPL, AGPL)
- **forbidden** (proprietary, non-OSI)
- **unknown** (undetectable)

Allowed types: notice (MIT, BSD, Apache, ISC), permissive, reciprocal (MPL-2.0), unencumbered (public domain).
