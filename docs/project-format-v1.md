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

- `noise init` creates `.noise` structure + v1 manifest.
- `noise status` displays run list, last status, and latest run log tail.
- `noise run --standard dummy-freefield` executes an offline E2E run, persists results in `.noise/runs/<run-id>/results`, and records provenance.
