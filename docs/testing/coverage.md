# Test coverage

Coverage is measured on both targets, reported on every pull request, and held
to a floor. The floor is **advisory**: it fails no required status check and
blocks no merge. This document is where the numbers are ratcheted.

## What is measured

| Target   | Command                 | Reported by                            | Floor lives in                              |
| -------- | ----------------------- | -------------------------------------- | ------------------------------------------- |
| Backend  | `just test-coverage`    | `scripts/coverage-report.sh`           | `COVERAGE_FLOOR` in `go-ci.yml`             |
| Frontend | `just fe-test-coverage` | `frontend/scripts/coverage-report.mjs` | `thresholds` in `frontend/vitest.config.ts` |

Both jobs write their report to the run's job summary and to a sticky pull-request
comment — one comment per target, edited in place rather than appended, so it
always shows the current commit.

Everything a developer sees locally is produced by the same scripts CI publishes.
There is no CI-only measurement.

## The denominators are complete

This is the property that makes the percentages mean anything, and on both
targets it takes a deliberate setting to get.

**Backend.** `go test ./...` emits a coverage entry for every package named on
the command line, whether or not it has a test file. A package with no tests is
therefore counted at 0% rather than silently omitted — untested code drags the
percentage down instead of hiding from it.

**Frontend.** The test runner's default is the opposite: with `coverage.include`
unset, the denominator is "files some test happened to import", so every untested
module drops out of it and the headline describes the tested subset rather than
the app. `vitest.config.ts` pins `include: ["src/**/*.{ts,tsx}"]` for exactly
that reason. That default only ever errs in the flattering direction, which is
why it is pinned rather than inherited.

## No backend scope filter, and that is a measured decision

The sibling MeKo repositories filter the Go profile before reporting it, because
the same "every package on the command line" behaviour puts a large generated
tree into the denominator at zero count. Measured here on 2026-09-12:

- 0 files carrying a `Code generated ... DO NOT EDIT` header
- 0 mock/fake/testutil support files
- 47 packages, 43 with a test file
- 16,584 statements across 219 files, 0 duplicate blocks

There is nothing for a filter to remove, so the raw profile **is** the scoped
one. That is not assumed: `scripts/coverage-check.sh` fails with exit 2 if a
generated or support file ever appears, names the files, and points at the header
of `scripts/coverage-report.sh`. The moment that check fires is the moment this
repository needs a filter.

## Exit codes, and why there are three

`scripts/coverage-check.sh` distinguishes two failures that need different
responses:

| Exit | Meaning                                                                                    |
| ---- | ------------------------------------------------------------------------------------------ |
| 0    | Coverage meets the floor, or `COVERAGE_FLOOR` is unset (report only)                       |
| 1    | Coverage is below the floor — a real regression, go look for tests                         |
| 2    | The profile or the configuration is not trustworthy — do not believe the percentage at all |

Exit 2 exists because a broken measurement reads as a _low_ percentage rather
than as an error. A truncated profile reports 0%; a profile with no `mode:`
header would be parsed as valid; a non-numeric `COVERAGE_FLOOR` is coerced to 0
by awk and passes everything. Reporting any of those as "below the floor" sends
someone looking for missing tests that are not missing.

`just coverage-check` cannot preserve the distinction — just has a single failure
status — so `go-ci.yml` calls the script directly and reads `$?`.

`-covermode=atomic` is required, not preferred. The engine runs a worker pool and
much of the suite runs in parallel; a lost non-atomic counter write undercounts
without emitting any error.

## Ratchet ledger

Move a floor **up** when the measured number has risen and held. Never move one
down to make a red check green — a drop is either a regression to fix or a
deliberate trade to record here with its reason. Floors are set two points under
the measured baseline: enough headroom that ordinary run-to-run variance never
trips them, tight enough that a real regression does.

### Backend

| Date       | Commit    | Measured | Floor | Note                                                                                         |
| ---------- | --------- | -------- | ----- | -------------------------------------------------------------------------------------------- |
| 2026-09-12 | `87da006` | 77.3%    | 75    | First measurement. 12,822 / 16,584 statements, 219 files, 43 / 47 packages with a test file. |

Weakest packages at the baseline, for whoever goes looking for the cheapest wins:
`internal/standards/cnossos/road` 56.7% (305 statements), `internal/io/fgbimport`
66.3% (338), `internal/geo/terrain` 69.1% (366),
`internal/standards/framework` 70.7% (304), `internal/app/cli` 71.8% — the last
carrying 4,333 statements, a quarter of the tree.

### Frontend

| Date       | Commit    | Statements | Branches | Functions | Lines | Floors (l/s/f/b)  |
| ---------- | --------- | ---------- | -------- | --------- | ----- | ----------------- |
| 2026-09-12 | `87da006` | 73.6%      | 83.9%    | 77.9%     | 73.6% | 71 / 71 / 75 / 81 |

8,460 / 11,496 statements over 62 test files. By area: `src/pages` 81.7%,
`src/ui` 76.3%, `src/api` 82.8%, `src/model` 85.2%, `src/map` **37.3%**
(1,765 statements), `src/wasm` 41.4%, `src/layouts` 42.1%.

`src/map` is the one large gap and it is a known one: the MapLibre code needs a
real WebGL context, `map.test.tsx` stubs `MapView` out entirely, and `use-draw`
and `model-layers` have no tests at all. `PLAN.md` Priority 8 Phase F tracks it.

The frontend numbers are measured **with the WASM kernel built** and
`ACONIQ_REQUIRE_WASM=1`, exactly as the `frontend-coverage` job runs. Without the
kernel the four parity suites skip, `src/wasm/` and much of `browser-backend.ts`
leave the covered set, and the percentage drops — a floor measured one way and
enforced the other is a floor that moves on its own.

## Locally

```bash
just test-coverage        # backend profile + backend/coverage.html
just coverage-report      # render the Markdown CI publishes
just coverage-check       # report only
COVERAGE_FLOOR=75 just coverage-check   # enforce

just fe-test-coverage     # frontend, applies the thresholds itself
just fe-coverage-report   # render the Markdown CI publishes
```

The frontend HTML report lands in `frontend/coverage/index.html`.
