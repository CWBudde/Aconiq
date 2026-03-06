# Offline-Only Constraints (MVP)

Status date: 2026-03-06

## Product Mode

MVP is CLI-only and offline-first.

- Required command workflows are local (`noise init/import/validate/run/status/export/bench`).
- No HTTP server is required for MVP completion.
- Browser GUI is deferred to later phases.

## Networking Policy

- Core run path must not require internet access.
- External data fetches are optional tooling, never required for deterministic runs.

## Artifact Policy

- All run inputs and outputs are stored in the project folder.
- Provenance must be captured per run (standard/profile/parameters/input hashes).

## Deferred Capabilities

- `noise serve` and GUI workflows.
- Multiuser/server storage and remote execution.
- Online basemap dependencies in critical paths.
