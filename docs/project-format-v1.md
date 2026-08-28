# Project Format v1 (Local)

Status date: 2026-03-06

## Storage Strategy Decision

Chosen for v1: **Option B**

- JSON-only metadata initially
- File-based artifacts and run files
- SQLite deferred to a later migration

Rationale:

- Keeps Phase 3 simple and portable across Linux/macOS/Windows.
- Reduces schema/tooling complexity during CLI-first MVP phases.
- Preserves a clear migration path toward SQLite if query/performance needs increase.

## On-Disk Layout

`<project-root>/`

- `.noise/project.json` : project manifest (v1)
- `.noise/runs/<run-id>/run.log` : run-local log
- `.noise/runs/<run-id>/provenance.json` : run provenance manifest
- `.noise/artifacts/` : generated artifacts (reserved)
- `.noise/logs/` : shared logs (reserved)

## Manifest Schema (`.noise/project.json`)

Top-level fields:

- `manifest_version` (int)
- `project_id` (string)
- `name` (string)
- `crs` (string)
- `storage` (`kind`, `notes`)
- `scenarios[]` (scenario definitions)
- `runs[]` (run records with status/log/provenance pointers)
- `artifacts[]` (artifact references)
- `migrations` (current/supported version and migration history)
- `created_at`, `updated_at` (RFC3339 timestamps)

Core entities in v1:

- `Project`
- `Scenario`
- `Run`
- `StandardRef`
- `ArtifactRef`

## CLI Behavior (Phase 8 baseline)

- `aconiq init` creates `.noise` structure + v1 manifest.
- `aconiq status` displays run list, last status, and latest run log tail.
- `aconiq run --standard dummy-freefield` executes an offline E2E run, persists results in `.noise/runs/<run-id>/results`, and records provenance.

## Provenance Manifest

`provenance.json` records:

- resolved standard identity (`context`, `id`, `version`, `profile`)
- normalized run parameters
- optional module-specific metadata (for example data-pack version or reporting precision)
- input file hashes
- the standard-data digest (`standard_data`)
- generation timestamp and tool identity

### `standard_data`

`standard_data` identifies which module, at which evidence tier, with which
coefficient tables produced the run:

```json
"standard_data": {
  "algorithm": "sha256",
  "digest": "<hex>",
  "evidence_tier": "normative",
  "tables": [{ "name": "anlage2/tabelle-09-bruecken", "digest": "<hex>" }]
}
```

The top-level `digest` covers the standard ID, the evidence tier and every
table below it, so two runs whose digests agree used byte-identical coefficient
data. The per-table digests make a mismatch attributable to one table rather
than to the whole module. Table entries are sorted by name and every map is
hashed with its keys sorted, so the digest is independent of Go map iteration
order.

The field is omitted for a module that carries no coefficient data at all —
`dummy-freefield` computes from its run parameters alone.

Coefficient tables are deliberately **not** entries in `input_hashes`: that map
is defined as input-file path to SHA-256, and every entry in it is rendered as
an "Input files" row in generated reports. Only a coefficient artifact that
genuinely is a hashed file on disk belongs there.
