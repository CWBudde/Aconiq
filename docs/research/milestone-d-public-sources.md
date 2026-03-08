# Milestone D Public Sources

Status date: 2026-03-08

This note summarizes the public sources collected for Milestone D and what they are
actually useful for. It is the committed counterpart to the local-only downloads under
`docs/research/external/`.

## Scope

Milestone D asks for:

- license-safe public validation / verification cases for:
  - `cnossos-road`
  - `cnossos-rail`
  - `cnossos-industry`
  - `cnossos-aircraft`
- license-safe validation scenarios or reference totals for:
  - `bub-road`
  - `buf-aircraft`
  - `beb-exposure`
- deterministic acceptance fixtures under `internal/qa/`

The final bullet is now implemented in code. The remaining open work is mostly about
finding public evidence that can justify or benchmark outputs.

## Collected Public Sources

### 1. EUR-Lex: Commission Directive (EU) 2015/996

Links:

- https://eur-lex.europa.eu/eli/dir/2015/996/oj/eng
- https://eur-lex.europa.eu/legal-content/EN/TXT/PDF/?uri=CELEX%3A32015L0996

What it gives us:

- Official normative CNOSSOS method text under Annex II.
- Normative scope for:
  - road traffic
  - railway traffic
  - industrial noise
  - aircraft noise
- A source to cite when documenting:
  - required indicators
  - period structure
  - domain boundaries
  - methodological terminology

What it does not give us directly:

- A ready-to-run public benchmark dataset.
- Module-level expected numeric outputs for small synthetic scenarios.

Implication:

- This is the primary source for Milestone E and for scope statements in phase docs.
- It does not by itself close the Milestone D validation-case checkboxes.

### 2. Publications Office / JRC CNOSSOS-EU publication

Links:

- https://op.europa.eu/en/publication-detail/-/publication/80bca144-bd3a-46fb-8beb-47e16ab603db/language-en
- direct PDF download via the Publications Office download handler

What it gives us:

- Public explanatory background around CNOSSOS-EU.
- Context that can help map terminology and structure to the legal text.

What it does not give us directly:

- A maintained public verification suite with machine-readable expected outputs.

Implication:

- Useful supporting reference.
- Still not enough to mark the CNOSSOS Milestone D “public validation / verification cases” items done.

### 3. EEA in-depth noise topic

Link:

- https://www.eea.europa.eu/en/topics/in-depth/noise

What it gives us:

- Public overview of environmental noise reporting context in Europe.
- A stable high-level reference for why END/CNOSSOS outputs matter.

What it does not give us directly:

- Scenario-level validation inputs or expected outputs.

Implication:

- Good contextual citation, not a benchmark source by itself.

### 4. EEA NOISE service / public portal

Link:

- https://www.eea.europa.eu/data-and-maps/data/external/noise-observation-and-information-service

What it gives us:

- Public portal entry point for European noise maps and related viewing workflows.
- Potential path to country/city-level reference totals or published map comparisons.

What it does not give us directly:

- A stable small-scenario fixture catalog.
- Guaranteed machine-readable extracts suitable for deterministic regression tests.

Implication:

- This is a candidate upstream source for public reference totals.
- Additional manual harvesting is still required before claiming Milestone D completion for CNOSSOS modules.

### 5. Eionet / CDR END data reporting help and annex PDFs

Links:

- https://cdr.eionet.europa.eu/help/aqd/noise
- linked annex PDFs captured locally in `docs/research/external/eea/noise/`

What it gives us:

- Public reporting guidance for END datasets and data flow annexes.
- Strong hints about:
  - what is reported publicly
  - aggregation structure
  - dataset categories and reporting mechanics

What it does not give us directly:

- A turnkey standards-validation fixture set.
- Explicit import-rights conclusions for all raw datasets we might want.

Implication:

- Useful for Milestone D candidate evidence discovery and for Milestone F packaging/import questions.
- More extraction work is needed from the annex documents before any extra checkbox can be closed.

### 6. Umweltbundesamt noise maps page

Link:

- https://www.umweltbundesamt.de/themen/laerm/umgebungslaermrichtlinie/laermkarten

What it gives us:

- Public German strategic-noise-mapping context.
- A public Germany-specific anchor for BUB/BUF/BEB-related mapping evidence searches.

What it does not give us directly:

- Import-rights clearance for all relevant German source datasets.
- Public raw benchmark cases for BUB or BUF modules.

Implication:

- Useful public anchor.
- Not enough yet to close Milestone F rights questions or additional Milestone D validation items.

## What We Can Mark Done From This

Already done in code:

- deterministic acceptance fixtures under `internal/qa/`
- central acceptance catalog and runner

Already supportable from current repo evidence:

- CNOSSOS Road now has a public, attributable road-evidence note at the
  reference-total level:
  - `docs/research/cnossos-road-public-reference-totals.md`
- CNOSSOS Rail now has a public, attributable rail-evidence note at the
  reference-total level:
  - `docs/research/cnossos-rail-public-reference-totals.md`
- CNOSSOS Industry now has a public, attributable industry-evidence note at the
  reference-total / exposure-share level:
  - `docs/research/cnossos-industry-public-reference-totals.md`
- CNOSSOS Aircraft now has a public, attributable aircraft-evidence note at the
  reference-total level:
  - `docs/research/cnossos-aircraft-public-reference-totals.md`
- BUF Aircraft now has a public, attributable mapping-aircraft evidence note at
  the reference-total level:
  - `docs/research/buf-aircraft-public-reference-totals.md`
- BEB Exposure now has a public, attributable exposure-evidence note at the
  reference-total level:
  - `docs/research/beb-public-reference-totals.md`
- BUB Road, BUF Aircraft, and BEB Exposure have repo-authored synthetic acceptance fixtures
  and source inventories.

## What Remains Open

Still open in `PLAN.md` after this research pass:

- no additional CNOSSOS Milestone D evidence items remain open

Reason:

- CNOSSOS Road and CNOSSOS Rail are covered only at the public reference-total
  level, not as scenario-level deterministic fixtures.
- CNOSSOS Industry is covered only at the public reference-total /
  exposure-share level, not as a scenario-level deterministic fixture.
- CNOSSOS Aircraft is covered only at the public reference-total level, not as a
  scenario-level deterministic fixture.

## Recommended Next Steps

1. Mine the EEA NOISE portal and country reporting outputs for downloadable map layers, summaries, or
   published totals that can be cited and normalized into acceptance evidence.
2. Read the captured CDR annex PDFs and extract a short note describing which data flows might contain
   reusable public totals for each CNOSSOS domain.
3. If stronger evidence is needed later, add standard-specific notes for
   scenario-level or fixture-ready public aircraft validation inputs.
4. Track any future move from public reference totals to deterministic public
   fixtures as a separate conformance milestone, not as unfinished work in
   Phases 10 to 13.
