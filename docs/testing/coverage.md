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
There is no CI-only measurement — but note what that does and does not buy. Both
sides running the same script is exactly why the frontend figure could be wrong in
both places at once: the script measures whatever tree it is pointed at, and
neither side generated `src/i18n/` before running it. What saved the local number
until now was `fe-ci` happening to run `fe-typecheck` first. The recipes that run
the suite now depend on `fe-i18n` by name, so the generated code is a stated
prerequisite rather than a side effect of recipe order.

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

| Date       | Commit    | Measured | Floor | Note                                                                                           |
| ---------- | --------- | -------- | ----- | ---------------------------------------------------------------------------------------------- |
| 2026-09-12 | `87da006` | 77.3%    | 75    | First measurement. 12,822 / 16,584 statements, 219 files, 43 / 47 packages with a test file.   |
| 2026-09-17 | `f7e0885` | 71.2%    | 75    | **Below the floor**, and honestly so — no measurement defect here. 12,464 / 17,512 statements. |
| 2026-09-17 | `317fdfa` | 78.9%    | 76    | Shortfall closed and the floor ratcheted. 13,825 / 17,512 statements.                          |

The backend drop is real and it is not spread evenly. The tree grew by ~930
statements between those rows while the covered count barely moved, and almost all
of the new mass is one package: `internal/io/soundplanimport` now carries **1,365
statements at 14.4%**, ~1,168 of them uncovered — on its own about the size of the
whole shortfall against the floor. It does not appear in the baseline's
weakest-packages list below because most of it did not exist yet. The floor was
not lowered; the tests were the fix, and `PLAN.md` Priority 13 had already asked
for them ("Add unit tests for all parsers").

That is what the third row is. `soundplanimport` went 14.4% → 94.3% on synthetic
fixtures alone — its 62 licensed-fixture tests still skip, so the number holds in
CI — and the ledger's own nominated cheapest wins went with it:
`internal/standards/framework` 70.7% → 98.7%, `internal/geo/terrain` 69.1% →
90.7%, `internal/io/fgbimport` 66.3% → 90.5%, `internal/app/config` 82.4% → 100%,
`internal/app/logging` 0% → 95.5%. The floor moves 75 → 76, two points under the
new measurement.

Weakest packages at the baseline, for whoever goes looking for the cheapest wins:
`internal/standards/cnossos/road` 56.7% (305 statements), `internal/io/fgbimport`
66.3% (338), `internal/geo/terrain` 69.1% (366),
`internal/standards/framework` 70.7% (304), `internal/app/cli` 71.8% — the last
carrying 4,333 statements, a quarter of the tree.

### Frontend

| Date       | Commit    | Statements | Branches | Functions | Lines | Floors (l/s/f/b)  |
| ---------- | --------- | ---------- | -------- | --------- | ----- | ----------------- |
| 2026-09-12 | `87da006` | 73.6%      | 83.9%    | 77.9%     | 73.6% | 71 / 71 / 75 / 81 |
| 2026-09-17 | `f7e0885` | 81.8%      | 87.0%    | 83.3%     | 81.8% | 71 / 71 / 75 / 81 |
| 2026-09-17 | `317fdfa` | 86.8%      | 87.8%    | 86.1%     | 86.8% | 84 / 84 / 84 / 85 |

10,706 / 13,087 statements over 83 test files. By area: `src/results` 99.7%,
`src/pages` 94.4%, `src/run` 90.0%, `src/model` 89.4%, `src/(root)` 85.7%,
`src/import` 84.7%, `src/api` 82.7%, `src/ui` 77.0%, `src/map` **62.6%**
(1,953 statements), `src/wasm` 45.2%, `src/layouts` 41.7%.

**Every frontend number CI published between those two rows was wrong, and badly.**
The `frontend-coverage` job ran `bun run test:coverage` with no `compile:i18n`
before it. `src/i18n/` is gitignored Paraglide output that nothing else in that
job writes — `frontend-ci` only has it because `typecheck` compiles it as a side
effect — so on a fresh checkout every module importing `@/i18n/messages` failed to
resolve. That took the whole UI half of the suite out of the **numerator** while
leaving it in the denominator. The published figure was **35.8%** against a true
81.8%, with `src/pages` and `src/import` reading 0.0% where they are in fact 94.4%
and 84.7%, and the comment blamed the floors.

It is the same defect as `48b63b2`, which added the `compile:i18n` step to
`frontend-ci` for `tsc`; `frontend-coverage` never got it. The floors were never
the problem, and they are only being moved now because the fourth row measured a
tree with `src/map` covered: 62.6% → 94.0%, the last genuinely thin area, closing
the file that carried 326 uncovered statements on its own. **The lesson generalises: a job that consumes
generated output has to generate it, and inheriting it from another job's side
effect is not a mechanism.** Anything new that runs the suite needs that step by
name.

`src/map` is the one large gap and it is a known one: the MapLibre code needs a
real WebGL context, `map.test.tsx` stubs `MapView` out entirely, and `use-draw`
and `model-layers` have no tests at all. `PLAN.md` Priority 8 Phase F tracks it.

The frontend numbers are measured **with the WASM kernel built** and
`ACONIQ_REQUIRE_WASM=1`, exactly as the `frontend-coverage` job runs. Without the
kernel the four parity suites skip, `src/wasm/` and much of `browser-backend.ts`
leave the covered set, and the percentage drops — a floor measured one way and
enforced the other is a floor that moves on its own.

"The kernel" is **two** files, and `kernel-node.ts` needs both:
`frontend/public/aconiq.wasm` and `frontend/public/wasm_exec.js`. With only one
of them in place the 49 parity tests skip and the suite still reports green over
what it never ran, because `kernelSkipReason` is fatal only under
`ACONIQ_REQUIRE_WASM`, which CI sets and a local checkout or worktree does not.
Read the skip count, not the exit code. `just wasm-build` writes both — it
builds the kernel and copies `wasm_exec.js` out of
`$(go env GOROOT)/lib/wasm/` — so a tree that has one and not the other got
them from somewhere else.

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
