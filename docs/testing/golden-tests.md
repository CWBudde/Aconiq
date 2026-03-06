# Golden Test Conventions

Status date: 2026-03-06

## `testdata/` Conventions

- Golden snapshots live under the package that owns the tests, in `testdata/`.
- Naming format: `<scenario>.golden.<ext>` (for example `run-summary.golden.json`).
- Use stable, canonical serialization in tests (sorted keys and normalized indentation).

## Snapshot Update Workflow

1. Intentionally change behavior.
2. Run `scripts/update-golden.sh` from repository root.
3. Review changed files in `testdata/` for expected deltas.
4. Run `scripts/test-go.sh` and ensure tests pass without `UPDATE_GOLDEN`.
5. Commit code and updated snapshots together.

## Guardrails

- Never update goldens blindly in unrelated refactors.
- If a snapshot change is surprising, investigate before accepting.
