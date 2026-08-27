# Lint Triage

Status: 2026-08-27 · `golangci-lint` 2.12.2 · config `/.golangci.yml` · entry point `just lint`

## Why this document exists

The backend did not compile for several months because of a module-path mismatch. `golangci-lint`
aborts the whole run on a typecheck error, so it reported a single issue and CI looked healthy.
With the build fixed, the linter reports **542 issues**. None of them are new: they are the backlog
that the broken typecheck had been hiding.

This document is the audit record for the triage. Every linter that is switched off in
`.golangci.yml` must have a row here with a reason, and every finding that is kept must appear
in the follow-up lists below.

## Headline numbers

|                              | Issues     |
| ---------------------------- | ---------- |
| Before triage                | **542**    |
| After config changes         | **112**    |
| Removed by disabling linters | 430 (79 %) |
| Removed by fixing code       | 0          |

**Be clear about what happened here: the reduction is entirely configuration.** No Go source was
touched. Three linters (`goconst`, `wsl_v5`, `noinlineerr`) accounted for 427 of the 430, and the
remaining 3 came from dropping `gocyclo`, which duplicates `cyclop`. The 112 findings that remain
are the real work, and `just lint` is **not green** until they are addressed.

## Baseline distribution (before)

| Linter      | Count |     | Linter                                                             |  Count |
| ----------- | ----: | --- | ------------------------------------------------------------------ | -----: |
| goconst     |   266 |     | predeclared                                                        |      5 |
| wsl_v5      |   120 |     | revive                                                             |      4 |
| noinlineerr |    41 |     | gocyclo                                                            |      3 |
| cyclop      |    31 |     | unparam                                                            |      3 |
| gocognit    |    16 |     | dogsled                                                            |      2 |
| perfsprint  |    11 |     | dupl                                                               |      2 |
| modernize   |     9 |     | nolintlint                                                         |      2 |
| gosec       |     8 |     | tagliatelle                                                        |      2 |
| intrange    |     6 |     | unconvert                                                          |      2 |
|             |       |     | unused                                                             |      2 |
|             |       |     | funlen, gocritic, nestif, nilnil, prealloc, recvcheck, staticcheck | 1 each |

The concentration is extreme: `internal/app/cli` alone carries 102 of the `goconst`, 83 of the
`wsl_v5` and 31 of the `noinlineerr` hits. That package is 16 450 LOC — about a third of the
backend — which PLAN.md Priority 7 already identifies as the root architectural problem.

## Per-linter decisions

### Disabled

| Linter        | Count | Decision    | Reason                                                                                                                                                                                                                                                                                                                                                                 |
| ------------- | ----: | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `goconst`     |   266 | **Disable** | Judged below — the repeated strings are almost all incidental.                                                                                                                                                                                                                                                                                                         |
| `wsl_v5`      |   120 | **Disable** | Pure whitespace and statement-cuddling style. It cannot detect a defect by construction: every finding is "add or remove a blank line". At 120 findings across two packages it would dominate every review diff while catching nothing. `wsl` (v1) was already disabled in favour of `wsl_v5`; both are now off, and the now-dead `wsl_v5` settings block was removed. |
| `noinlineerr` |    41 | **Disable** | Forbids `if err := f(); err != nil { … }`. That form is idiomatic Go, is used throughout the standard library, and is _better_ than the alternative because it scopes `err` to the branch that handles it. Adopting the linter would mean 41 mechanical rewrites that each widen a variable's scope. This is a minority style opinion, not a correctness rule.         |
| `gocyclo`     |     3 | **Disable** | Redundant. `cyclop` computes the same cyclomatic metric and is already enabled with an explicit `max-complexity: 15`. Keeping both means the same function is reported twice under two different thresholds. `cyclop` is the single cyclomatic gate.                                                                                                                   |
| `gomodguard`  |     0 | **Disable** | Deprecated since golangci-lint v2.12.0; see the migration note below.                                                                                                                                                                                                                                                                                                  |

#### `goconst` in detail

`goconst` is the single largest contributor, and the decision to drop it deserves the most
scrutiny, so here is the evidence.

The repeated strings fall into three groups:

1. **JSON output keys in map literals** (the highest-count hits): `"status"` × 16, `"run_id"` × 14,
   `"receiver_mode"` × 11, `"command"` × 8. These appear inside `map[string]any{…}` literals that
   serialise the CLI's `--json` output, e.g. `run_pipeline.go:731`. Hoisting them into
   `const runIDKey = "run_id"` makes the emission sites _less_ readable — you can no longer see the
   output shape by reading the literal — and it hides the fact that these keys are part of a public
   output contract that should be visible and greppable at every emission point.
2. **CLI flag / parameter names**: `"road_speed_kph"`, `"aircraft_thrust_mode"`,
   `"rail_curve_radius_m"` and about 40 more, each at exactly 3 occurrences (flag registration,
   parse, provenance metadata). Here `goconst` has a fair point in principle — a typo between
   registration and parse fails silently. But the fix is not a per-package constant; it is the
   parameter-descriptor refactor in PLAN.md Priority 7 that removes the triplication entirely.
   A constant per flag would add ~40 identifiers and leave the duplication in place.
3. **Genuinely constant-worthy**: `"Point"` × 8 and `"Polygon"` × 4 are GeoJSON geometry type
   tags that really should come from a shared `modelgeojson` constant. That is 12 findings out
   of 266.

Threshold tuning does not rescue it. Measured: `min-occurrences` 4 → 120 findings, 6 → 78, 8 → 53,
10 → 37. Raising the threshold makes the signal _worse_, because the highest-count strings are
precisely the JSON output keys from group 1 that we least want hoisted. There is no
"ignore map-literal keys" option.

**Decision: disable.** Recorded consequence: the group-3 GeoJSON tags and the group-2 flag-name
triplication are no longer detected. Both are already tracked in PLAN.md Priority 7, which is the
right place to fix them.

### Kept enabled — these are code work

Everything below stays on. Each is listed with file:line in the follow-up section.

| Linter        | Count | Assessment                                                                                                                                       |
| ------------- | ----: | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `cyclop`      |    31 | Real. Complexity gate. See hotspot list.                                                                                                         |
| `gocognit`    |    16 | Real. Complements `cyclop` (cognitive vs cyclomatic).                                                                                            |
| `perfsprint`  |    11 | Real but trivial: `fmt.Errorf` with no verbs → `errors.New`. Mechanical.                                                                         |
| `modernize`   |     9 | Real. `slices.Backward`, `min`/`max`, `slices.Contains`. Mechanical.                                                                             |
| `gosec`       |     8 | Mixed — two are worth a security look, one is a false positive. Detailed below.                                                                  |
| `intrange`    |     6 | Real. `for i := 0; i < n; i++` → `for i := range n`. Mechanical.                                                                                 |
| `predeclared` |     5 | Real and worth keeping: shadowing `min`/`max` is now actively confusing since Go 1.21 made them builtins.                                        |
| `revive`      |     4 | Real. `file-length-limit: 1500`. Directly matches the god-file split in PLAN.md P7.                                                              |
| `unparam`     |     3 | Real. Dead parameters and an always-nil error return.                                                                                            |
| `dogsled`     |     2 | Cosmetic. Accessor helpers discarding 3 of 4 return values.                                                                                      |
| `dupl`        |     2 | Real but low value — symmetric day/night blocks in the `dummy` reference standard.                                                               |
| `nolintlint`  |     2 | Real and valuable: two `//nolint:gosec` directives no longer suppress anything. Stale suppressions must be removed or they mask future findings. |
| `tagliatelle` |     2 | Real. Two JSON tags violate the project's own `json: snake` rule.                                                                                |
| `unconvert`   |     2 | Real. Redundant type conversions.                                                                                                                |
| `unused`      |     2 | Real dead code. Already listed in PLAN.md P7.                                                                                                    |
| `funlen`      |     1 | Real. 45 statements vs limit 40.                                                                                                                 |
| `gocritic`    |     1 | Real. if-else chain → switch.                                                                                                                    |
| `nestif`      |     1 | Real.                                                                                                                                            |
| `nilnil`      |     1 | Real API smell — see below.                                                                                                                      |
| `prealloc`    |     1 | Cosmetic, in a test.                                                                                                                             |
| `recvcheck`   |     1 | Style. Mixed pointer/value receivers is a real bug class in general, but the single hit here is benign — verified.                               |
| `staticcheck` |     1 | Real. `ST1005` capitalised error string.                                                                                                         |

## Follow-up: real-defect findings

These are the findings that indicate a defect or a genuine smell rather than a style preference.
19 findings across 6 linters, plus the security items.

### Correctness / API smells

| File:line                                                           | Linter      | Assessment                                                                                                                                                                                                                                                                                                                                     |
| ------------------------------------------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `backend/internal/io/soundplanimport/gridmap.go:458`                | recvcheck   | **Not a defect — verified 2026-08-27.** `gridMapStatAccumulator` mixes receivers: `add` (`:487`) takes a pointer and mutates; `finish` (`:503`) takes a value and only reads, returning a `GridMapValueStats`. No mutation is discarded. Fix for consistency if desired (make `finish` a pointer receiver), but there is no bug here to chase. |
| `backend/internal/app/cli/compare_raster.go:86`                     | nilnil      | **Real smell.** `prepareSoundPlanRasterCompare` returns `(nil, nil)` for "there are no grid maps, nothing to compare". Every caller must remember to nil-check a non-error nil. Fix: return `(*T, bool, error)` or a `ErrNoGridMaps` sentinel.                                                                                                 |
| `backend/internal/geo/modelgeojson/validate.go:282`                 | staticcheck | **Real.** `ST1005`: capitalised error string. Cosmetic in isolation, but this is user-facing validation output, so the inconsistency is visible.                                                                                                                                                                                               |
| `backend/internal/app/cli/root.go:107`                              | unused      | **Real dead code.** `newPlaceholderCommand`. Already listed in PLAN.md P7 "Delete dead code".                                                                                                                                                                                                                                                  |
| `backend/internal/standards/cnossos/road/emission.go:344`           | unused      | **Real dead code.** `mustFinite`. Already listed in PLAN.md P7.                                                                                                                                                                                                                                                                                |
| `backend/internal/standards/cnossos/industry/propagation.go:126`    | unparam     | **Real dead code.** `sourceDistance` never uses `cfg`. Already listed in PLAN.md P7 by exact line.                                                                                                                                                                                                                                             |
| `backend/internal/api/httpv1/handler.go:304`                        | unparam     | **Real.** `(Handler).handleRunsList` ignores `r`. For an HTTP handler that means query parameters and context are being dropped — check whether that is intended before deleting the parameter.                                                                                                                                                |
| `backend/internal/app/cli/compare_raster.go:551`                    | unparam     | **Real.** `appendSyntheticRasterReceivers` always returns a nil error, so callers are writing dead error-handling. Either drop the return or make it fallible.                                                                                                                                                                                 |
| `backend/internal/app/cli/status.go:69`, `:92`                      | unconvert   | **Real.** Redundant conversions; harmless but noise.                                                                                                                                                                                                                                                                                           |
| `backend/internal/app/cli/modelio_helpers.go:68`                    | predeclared | **Real.** Parameter named `max` shadows the Go 1.21 builtin.                                                                                                                                                                                                                                                                                   |
| `backend/internal/app/cli/run_options.go:437`, `:457`               | predeclared | **Real.** Parameters named `min`.                                                                                                                                                                                                                                                                                                              |
| `backend/internal/standards/framework/framework.go:336`, `:341`     | predeclared | **Real.** Variables `min` / `max` — in `validateScalar`, a function that does numeric range checks, so shadowing the builtins here is maximally confusing.                                                                                                                                                                                     |
| `backend/internal/standards/schall03/emission_v2.go:132`, `:147`    | nolintlint  | **Real.** Two `//nolint:gosec` directives are stale — gosec no longer fires there. Remove them; leaving them masks future G-rule hits on those lines.                                                                                                                                                                                          |
| `backend/internal/qa/acceptance/rls19_test20/runner.go:126`, `:127` | tagliatelle | **Real.** JSON tags `LrDay` / `LrNight` violate the configured `json: snake` rule; should be `lr_day` / `lr_night`. Confirm against the TEST-20 fixture format before changing — if the fixture demands PascalCase, this needs a targeted exclusion instead.                                                                                   |
| `backend/internal/standards/dummy/freefield/freefield.go:67`, `:83` | dupl        | **Real but benign.** Two 16-line duplicate blocks in the dummy reference standard. Low priority.                                                                                                                                                                                                                                               |
| `backend/internal/app/cli/compare_raster.go:524`, `:529`            | dogsled     | **Cosmetic.** `minYFromArea` / `maxYFromArea` each do `_, minY, _, _ := calcAreaBounds(area)`. Idiomatic enough; a targeted `//nolint:dogsled` is a reasonable resolution.                                                                                                                                                                     |
| `backend/internal/io/gpkgimport/gpkgimport_test.go:70`              | prealloc    | **Cosmetic**, in a test.                                                                                                                                                                                                                                                                                                                       |
| `backend/internal/io/soundplanimport/terrain_text.go:67`            | gocritic    | **Real.** if-else chain → switch. Owned by in-flight SoundPLAN work.                                                                                                                                                                                                                                                                           |
| `backend/internal/app/cli/import_soundplan.go:242`                  | nestif      | **Real.** Nested blocks, complexity 5.                                                                                                                                                                                                                                                                                                         |
| `backend/internal/assessment/bimschv16/assessment.go:380`           | funlen      | **Real.** `BuildExportEnvelope`, 45 statements vs 40.                                                                                                                                                                                                                                                                                          |

### Mechanical (safe, low-risk, but 26 findings)

`perfsprint` (11), `modernize` (9), `intrange` (6). All are `fmt.Errorf`→`errors.New`,
`for i := 0; i < n; i++`→`for i := range n`, and `slices.*` adoptions. **`golangci-lint run --fix`
handles all of these**, but 20 of the 26 sit in `internal/io/soundplanimport` and
`internal/io/fgbimport`, which are being edited concurrently. Run the fixer package-by-package
once that work lands, not repo-wide.

## Security findings

### Kept and reported (8)

| File:line                                                                                                     | Rule | Assessment                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------------------------------------------------------------------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `backend/internal/api/httpv1/handler.go:456`                                                                  | G702 | **Worth a real look.** The HTTP handler shells out to the `aconiq` binary with request-controlled argv: `--param <key>=<value>` for every key in `req.Params`, and `--input <path>` for every entry in `req.InputPaths`. It uses `exec.CommandContext` with an argv slice and no shell, so this is not classic shell injection — but an unauthenticated caller choosing arbitrary `--input` paths and arbitrary flag-shaped `--param` values is a genuine exposure surface. Validate `req.Params` keys against the standard's declared parameter set and constrain `InputPaths` to the project root. |
| `backend/internal/api/httpv1/handler.go:743`                                                                  | G120 | **Real, minor.** `r.ParseMultipartForm(maxTerrainUploadBytes)` bounds only what is buffered _in memory_; the request body itself is unbounded. Wrap `r.Body` in `http.MaxBytesReader` for an actual upload cap.                                                                                                                                                                                                                                                                                                                                                                                      |
| `backend/internal/app/cli/export_assessment.go:65`, `export.go:549`, `import.go:656`, `modelio_helpers.go:45` | G703 | **Low risk, CLI context.** Path traversal via taint analysis on user-supplied paths. In a CLI the user already has the invoking user's filesystem rights, so this is not a privilege boundary. Same code paths reached through the HTTP API _are_ a boundary — resolve alongside the G702 item above.                                                                                                                                                                                                                                                                                                |
| `backend/internal/io/soundplanimport/geowand.go:181`                                                          | G115 | **Real.** `uint64` → `int64` conversion on a value read from an untrusted SoundPLAN file. Needs a bounds check, not a `//nolint`. Owned by in-flight SoundPLAN work.                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `backend/internal/report/export/gpkg.go:207`                                                                  | G202 | **False positive.** `createSQL` concatenates `sanitizeColumnName(indicator)`, and `sanitizeColumnName` (`gpkg.go:860`) rewrites every character outside `[a-z0-9_]` to `_` and substitutes `"value"` for the empty string. Injection is not reachable. Resolve with `//nolint:gosec // column names pass sanitizeColumnName allow-list` rather than restructuring.                                                                                                                                                                                                                                   |

### Suppressed by the exclusion presets — flagged for review

The `common-false-positives` / `legacy` presets currently hide these. Measured by re-running gosec
standalone without exclusions:

| Rule                                | Hidden (non-test) | Recommendation                                                                                                                                                                                                                                                                                                                                                                   |
| ----------------------------------- | ----------------: | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| G304 (file inclusion via variable)  |            **47** | **Keep suppressed.** This is a file-format toolchain; opening a path the user named is the entire product. Enabling it would produce 47 unactionable findings. The path-traversal question is real but belongs to the HTTP surface, where G703 already covers it.                                                                                                                |
| G301 (directory permissions > 0750) |            **44** | **Worth a decision, not a blanket suppression.** All 44 are `os.MkdirAll(..., 0o755)` on project and artifact directories. `0o755` is fine for a local CLI. But this is a consistent, mechanical pattern: either standardise on `0o750` and re-enable the rule, or record the `0o755` choice as a deliberate project convention here. Right now it is neither — it is invisible. |
| G101 (hardcoded credentials)        |                 4 | Test-only, in `io/osmimport/osmimport_test.go` — fixture strings that pattern-match as tokens. Correctly suppressed.                                                                                                                                                                                                                                                             |
| G204 (subprocess with variable)     |                 1 | Same site as the G702 above; resolve together.                                                                                                                                                                                                                                                                                                                                   |
| G404 (weak RNG)                     |                 1 | `geo/geometry_property_test.go` — property-test seeding. Correctly suppressed.                                                                                                                                                                                                                                                                                                   |
| G306 (WriteFile perms)              |                 1 | Test-only. Correctly suppressed.                                                                                                                                                                                                                                                                                                                                                 |

**Note on `geo/terrain/geotiff.go`:** this file carries 13 `//nolint:gosec` directives for
`uint*`→`int` conversions in TIFF header parsing. These are _not_ preset-suppressed — they are
explicit, individually justified suppressions with reasons attached, and `nolintlint` confirms
every one of them is still doing work. That is exactly how G115 should be handled and needs no
change. (A `//nolint` on parsed-from-file offsets does deserve a periodic re-read to confirm the
bounds claims still hold, but the mechanism is correct.)

## Complexity hotspots

47 findings (31 `cyclop`, 16 `gocognit`). **Do not restructure these individually** — most of them
dissolve once PLAN.md Priority 7 lands, and pre-emptive splitting would collide with it.

The worst 20, by metric:

| Function                                            | File:line                                | Metric           |
| --------------------------------------------------- | ---------------------------------------- | ---------------- |
| `newExportCommand`                                  | `app/cli/export.go:46`                   | cognitive **90** |
| `executeFormatExports`                              | `app/cli/export.go:681`                  | cognitive **89** |
| `buildSoundPlanModelAndReport`                      | `app/cli/import_soundplan.go:129`        | cognitive **60** |
| `validateGeometry`                                  | `geo/modelgeojson/validate.go:123`       | cognitive 48     |
| `GenerateContours`                                  | `report/export/contour.go:38`            | cognitive 48     |
| `TestRegistryResolvesDummyFreefield`                | `standards/registry_test.go:5`           | cyclomatic 38    |
| `marchingSquares`                                   | `report/export/contour.go:133`           | cognitive 38     |
| `newStatusCommand`                                  | `app/cli/status.go:17`                   | cognitive 40     |
| `newValidateCommand`                                | `app/cli/validate.go:15`                 | cognitive 40     |
| `TestImportSoundPlanWritesNormalizedModelAndReport` | `app/cli/import_soundplan_test.go:14`    | cyclomatic 46    |
| `parseCnossosRoadRunOptions`                        | `app/cli/run_options.go:522`             | cognitive 34     |
| `LoadRailOperationSummaries`                        | `io/soundplanimport/railops.go:35`       | cognitive 34     |
| `runCompare`                                        | `app/cli/compare.go:107`                 | cognitive 33     |
| `compareSoundPlanReceiverTables`                    | `app/cli/compare.go:413`                 | cognitive 33     |
| `joinSegments`                                      | `report/export/contour.go:268`           | cognitive 33     |
| `finalizeSoundPlanRasterCompare`                    | `app/cli/compare_raster.go:217`          | cognitive 32     |
| `LoadProjectBundle`                                 | `io/soundplanimport/projectbundle.go:30` | cognitive 32     |
| `(IndustrySource).Validate`                         | `standards/cnossos/industry/model.go:75` | cognitive 31     |
| `bub/road.Validate`                                 | `standards/bub/road/model.go:87`         | cyclomatic 25    |
| `cnossos/road.Validate`                             | `standards/cnossos/road/model.go:91`     | cyclomatic 25    |

Two structural observations:

- **`app/cli` carries 16 of the 20.** `newExportCommand` at cognitive 90 and `executeFormatExports`
  at 89 are three times the threshold. PLAN.md P7 already schedules `app/cli/export.go` (1 005
  lines) for splitting, and notes the 562-line dispatch switch at `run_pipeline.go:143` that
  carries `//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx` — i.e. the worst function in the
  codebase is not even in this list because it is already suppressed wholesale.
- **12 of the 31 `cyclop` hits are `Validate` methods** in `standards/*/model.go` and
  `standards/*/propagation.go`, all in the 16–25 range, all the same shape: a long flat sequence of
  independent field range checks. That is not tangled control flow, it is a schema written as code.
  The right fix is the shared validation-descriptor approach implied by PLAN.md P7's "extract a
  shared acoustics core", not manual splitting. If P7 slips, a targeted exclusion for
  `standards/.*/(model|propagation)\.go` with this rationale is defensible — but only then.

The 4 `revive file-length-limit` hits (`run_extract.go` 3 087 lines, `run_options.go` 1 604,
`run_phase8_test.go` 1 587, `rls19/road/road_test.go` 3 375) are the same story and are named
verbatim in PLAN.md P7 "Split the god files".

## Config changes made

1. **`gomodguard` → `gomodguard_v2`.** `gomodguard` is deprecated as of golangci-lint v2.12.0 and
   emitted a warning on every run. `gomodguard` is now explicitly disabled and `gomodguard_v2`
   explicitly enabled. There was no `gomodguard` settings block to migrate — the linter was only
   active via `default: all` with stock configuration — so nothing was lost. Verified with
   `golangci-lint config verify`, and the deprecation warning is gone from stderr.
2. **Disabled `goconst`, `wsl_v5`, `noinlineerr`, `gocyclo`** with the reasons in the table above.
   Removed the now-dead `wsl_v5` settings block and dropped `goconst` from the `_test.go`
   exclusion rule.
3. **Re-enabled `sqlclosecheck`** — see below.
4. **Annotated the `wrapcheck` exclusion** with a `FIXME(lint-triage)` pointing here.

### `sqlclosecheck` — the disable was wrong

`.golangci.yml` disabled `sqlclosecheck` with the comment _"no SQL in this project"_. That is
factually incorrect. Two packages use `database/sql` with `modernc.org/sqlite`:

- `backend/internal/io/gpkgimport/gpkgimport.go` — three `db.QueryContext` calls (lines 34, 155, 178)
- `backend/internal/report/export/gpkg.go` — GeoPackage writer

`sqlclosecheck` catches unclosed `*sql.Rows`, which leaks connections and, on SQLite, holds file
locks. That is exactly the defect class this project can hit.

**Action taken: re-enabled.** Measured result: **0 findings** — the existing code closes its rows
correctly. So this costs nothing today and guards the pattern from here on. `rowserrcheck` was
also measured at 0 and remains enabled via `default: all`.

### `wrapcheck` — flagged, not changed

`.golangci.yml` excludes `wrapcheck` for `path: internal/`. Since all backend code lives under
`backend/internal/`, this disables the linter completely — the exclusion comment says "too noisy in
internal packages", which in practice means "off". The project has an error-wrapping policy that is
therefore entirely unenforced.

Measured cost of removing the exclusion: **190 findings**.

**Recommendation: keep the exclusion for now, but not permanently, and not silently.** 190 findings
is too much to absorb in a triage pass, and — more importantly — fixing them mechanically would be
premature. PLAN.md P7 records "690 inline `errors.New` strings in non-test code and zero
package-level sentinels" and schedules typed/sentinel errors in `domain/errors`. Wrapping errors
that are about to be replaced by sentinels is wasted work. The sequencing should be:

1. Land the `domain/errors` sentinel work (PLAN.md P7).
2. Replace the blanket `path: internal/` exclusion with per-package exclusions.
3. Remove them package by package as each is converted.

The exclusion now carries a `FIXME(lint-triage)` comment with the finding count so it cannot stay
invisible.

## Where this leaves `just lint`

**Not green.** 112 findings remain, and 430 of the 542 went away because linters were switched off,
not because anything was fixed.

Rough shape of the remaining work:

| Bucket                                                                      | Findings | Notes                                                            |
| --------------------------------------------------------------------------- | -------: | ---------------------------------------------------------------- |
| Complexity (`cyclop`, `gocognit`, `revive` file-length, `funlen`, `nestif`) |       52 | Blocked on / dissolved by PLAN.md P7. Do not hand-fix.           |
| Mechanical, auto-fixable (`perfsprint`, `modernize`, `intrange`)            |       26 | `--fix` handles these; 20 are in packages under concurrent edit. |
| Real defects and smells                                                     |       26 | Listed above. Individually small.                                |
| Security                                                                    |        8 | Two need judgement (G702, G120), one is a false positive.        |

Realistic path to a merge gate:

1. Run `--fix` per package for the 26 mechanical findings once the SoundPLAN work lands. → 86 left.
2. Fix the 26 real-defect findings — most are one-line. Several are already PLAN.md P7 items. → ~60.
3. Resolve the security items: fix G120, validate the G702 inputs, `//nolint` the G202 false
   positive with a reason. → ~53.
4. The remaining ~53 are complexity and file-length. These _are_ PLAN.md Priority 7. Until it
   lands, either accept a non-green `just lint` or add a single time-boxed exclusion block that
   names P7 and is deleted with it.

Step 4 is the honest blocker. `just lint` cannot become a hard merge gate until Priority 7 lands
or an explicitly temporary, documented exclusion is added for the complexity linters.

## Reproducing these numbers

```bash
just lint                       # current state
cd backend && golangci-lint config verify -c ../.golangci.yml
```

To measure a suppressed rule, run the linter standalone with `linters.default: none`, a single
`enable:` entry and no `exclusions.presets` — that is how the G304/G301 and `wrapcheck` counts in
this document were obtained.
