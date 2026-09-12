# Golden Test Conventions

Status date: 2026-03-06

## `testdata/` Conventions

- Golden snapshots live under the package that owns the tests, in `testdata/`.
- Naming format: `<scenario>.golden.<ext>` (for example `run-summary.golden.json`).
- Use stable, canonical serialization in tests (sorted keys and normalized indentation).

## Snapshot Update Workflow

1. Intentionally change behavior.
2. Run `just update-golden` from repository root.
3. Review changed files in `testdata/` for expected deltas.
4. Run `just test` and ensure tests pass without `UPDATE_GOLDEN`.
5. Commit code and updated snapshots together.

## Goldens read from outside the Go tree

`backend/internal/app/cli/testdata/parity/` is the one exception to "the package
that owns the tests owns its snapshots". Its `*.geojson` models and
`*.golden.json` levels are written by `parity_golden_test.go` and read by
`frontend/src/api/browser-parity.test.ts`, which checks that browser mode builds
the same scene out of the same model that the CLI does.

Two consequences follow, and no Go tool will warn about either:

- Renaming, moving or reshaping anything under `testdata/parity/` breaks a test
  in the other tree. Grep `frontend/` before you do.
- A deliberate change there moves a golden that a frontend test asserts, so
  `just update-golden` alone does not finish the job — `just fe-test-wasm` has to
  pass too.

`frontend/src/wasm/kernel-parity.test.ts` reads the RLS-19 acceptance goldens in
the same way, but only reads them; they stay owned by the acceptance runner.

## Guardrails

- Never update goldens blindly in unrelated refactors.
- If a snapshot change is surprising, investigate before accepting.
