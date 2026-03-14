# RLS-19 Road + TEST-20 — public research notes (license-safe)

Status date: 2026-03-06

This note summarizes **publicly accessible** material relevant to implementing the Germany planning-track road noise method **RLS-19** and validating via **TEST-20**, without embedding restricted normative text/tables.

## Scope & legal context

- 16. BImSchV requires computing the road **Beurteilungspegel** separately for:
  - **Day:** 06:00–22:00
  - **Night:** 22:00–06:00
- The legal update ties the calculation to **RLS-19** (sections referenced in the regulation).

Source (public): https://dserver.bundestag.de/btd/19/184/1918471.pdf

## Acceptance suite: TEST-20

BASt provides a download page for TEST-20 and a **conformance declaration form**.

Sources (public):

- Landing page: https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Unterseiten/test20.html
- TEST-20 tasks PDF: https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-aufgaben.pdf?__blob=publicationFile&v=1
- Conformance form PDF: https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Downloads/test20-konformitaet.pdf?__blob=publicationFile&v=1

Implementation implications (high level):

- TEST-20 covers **emission tasks** (`E*`) and **immission tasks** (`I*` + `K*`).
- The conformance form enumerates the full task set expected for a “conform” implementation.
- TEST-20 distinguishes a “reference” setting vs a “check”/fine-grained setting to ensure the implementation is stable under different source splitting granularities.
- TEST-20 includes explicit **reflection** coverage (e.g., a dedicated reflection-conditions task).

Versioning note:

- Publicly available copies may differ by publisher/channel (e.g., BASt-hosted vs DIN-hosted). The project should pin which version is used for QA gating and record it in provenance/conformance artifacts.

## Model structure (emission vs propagation)

A publicly accessible overview article describes the **conceptual split** between:

- **Emission**: build a lane/direction-specific emission representation (length-related sound power) from vehicle groups, speeds, surface, gradient, junction, and a multiple-reflection surcharge.
- **Propagation**: compute receiver levels using segment-based road representation, with handling for shielding/terrain and **up to two reflections**.

Source (public practitioner overview):

- https://www.ingenieur.de/fachmedien/laermbekaempfung/verkehrslaerm/richtlinien-fuer-den-laermschutz-an-strassen-rls19/

## Input derivation (traffic data → RLS-19 inputs)

Public handouts illustrate workflows for deriving RLS-19-compatible inputs from traffic datasets (e.g., DTV), including:

- splitting into the RLS-19 vehicle groups (Pkw / Lkw1 / Lkw2 / Krad)
- day/night separation
- conversion to “maßgebende stündliche Verkehrsstärken” style quantities

Sources (public examples):

- Berlin: https://www.berlin.de/sen/uvk/_assets/verkehr/verkehrsdaten/umrechungsfaktoren-von-verkehrsmengen/rechenbeispiel.pdf
- RP Darmstadt: https://rp-darmstadt.hessen.de/sites/rp-darmstadt.hessen.de/files/2023-06/22_02laermkennwerte_rls2019_0.pdf

Implementation implications (high level):

- The `rls19/road` module should accept direct per-period traffic inputs, but it is useful to provide **optional helpers** to derive those from DTV and locally applicable factors.
- Helper logic should be isolated and clearly labeled as “input preparation”, not normative core.

## Repo-specific constraints

- Compliance boundaries: do not embed restricted standard text/tables verbatim; keep normative coefficients/tables **data-driven** and loadable from an external data pack.
- Determinism policy: stable ordering, deterministic splitting, stable reductions, float64 internally, rounding only at defined output boundaries.

See:

- `docs/preflight/compliance-boundaries.md`
- `docs/policies/determinism.md`

## TEST-20 version tracking

| Version | Date       | Source             | Notes                               |
| ------- | ---------- | ------------------ | ----------------------------------- |
| 2.1     | July 2025  | BASt download page | Added K5 reflection-conditions task |
| 2.0     | March 2021 | BASt download page | Initial release with RLS-19         |

### Authoritative version for CI gating

The CI-safe suite is derived from public TEST-20 v2.1 task _categories_ (not task data).
The `ci_safe_suite.json` manifest records `suite_version: derived-ci-safe-v1` and
`evidence_class: license-safe derived fixture suite`.

When running in `local-suite` mode against actual TEST-20 extractions, the suite manifest
must record the exact TEST-20 version used.

### Update handling

When a new TEST-20 version is published:

1. Check BASt download page for updated PDF
2. Compare task inventory (any new E*/I*/K\* tasks?)
3. Add corresponding CI-safe derived scenarios for new categories
4. Update `ci_safe_suite.json` manifest `suite_version` field
5. If a developer has the new TEST-20, update local-suite extractions and re-run
6. Document the version change in this table
