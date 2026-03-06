# Project Migration Strategy

Status date: 2026-03-06

## Versioning Model

- Manifest version is explicit (`manifest_version`).
- Backend supports a bounded version range (`current_version`, `latest_supported_version`).
- Migration history is tracked in manifest metadata.

## Migration Rules

1. Migrations are one-way, explicit steps (`vN -> vN+1`).
2. No silent in-place conversion during unrelated commands.
3. Before migration, create a backup/snapshot of project metadata.
4. Migration code must be deterministic and idempotent where possible.
5. Each migration step must include tests and sample fixtures.

## Operational Workflow

1. Load manifest.
2. If version is current, continue.
3. If version is older, run ordered migration steps.
4. Persist migrated manifest with appended migration record.
5. If version is newer than supported, fail with actionable error.

## Planned v1 -> v2 Direction

Primary expected trigger:
- Move metadata storage from JSON-only to SQLite when query and scale needs justify it.

Compatibility approach:
- Keep core domain entities stable (`Project`, `Scenario`, `Run`, `StandardRef`, `ArtifactRef`).
- Introduce adapters so CLI/app layers are storage-agnostic.
