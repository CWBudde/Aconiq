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

Two directories are exceptions to "the package that owns the tests owns its
snapshots". Both are written by a Go test and read by a frontend test:

- `backend/internal/app/cli/testdata/parity/` — `*.geojson` models and
  `*.golden.json` levels, written by `parity_golden_test.go` and read by
  `frontend/src/api/browser-parity.test.ts`, which checks that browser mode
  builds the same scene out of the same model that the CLI does.
- `backend/internal/report/results/testdata/csv-parity/` —
  `receiver_table.golden.json` (the input), `receiver_table.golden.csv` (the
  bytes) and `float_spelling.golden.json`, written by
  `receiver_table_csv_test.go` and read by
  `frontend/src/model/receiver-csv.parity.test.ts`, which checks that the
  browser CSV builder emits what `encoding/csv` emits. See
  `docs/result-containers-v1.md`, section "Receiver table CSV — byte contract".

Two consequences follow for each, and no Go tool will warn about either:

- Renaming, moving or reshaping anything under those directories breaks a test
  in the other tree. Grep `frontend/` before you do.
- A deliberate change there moves a golden that a frontend test asserts, so
  `just update-golden` alone does not finish the job — `just fe-test-wasm` has to
  pass too.

`frontend/src/wasm/kernel-parity.test.ts` reads the RLS-19 acceptance goldens in
the same way, but only reads them; they stay owned by the acceptance runner.

## Guardrails

- Never update goldens blindly in unrelated refactors.
- If a snapshot change is surprising, investigate before accepting.
