# Lint Triage

Status: 2026-08-28 · `golangci-lint` 2.12.2 · config `/.golangci.yml` · entry point `just lint`

## Why this document exists

The backend did not compile for several months because of a module-path mismatch. `golangci-lint`
aborts the whole run on a typecheck error, so it reported a single issue and CI looked healthy.
With the build fixed, the linter reports **542 issues**. None of them are new: they are the backlog
that the broken typecheck had been hiding.

This document is the audit record for the triage. Every linter that is switched off in
`.golangci.yml` must have a row here with a reason, and every finding that is kept must appear
in the follow-up lists below.

## Headline numbers

|                                             | Issues     |
| ------------------------------------------- | ---------- |
| Before triage (2026-08-27)                  | **542**    |
| After config changes                        | **112**    |
| After the resolution pass                   | **54**     |
| After the complexity pass                   | **0**      |
| Removed by disabling linters                | 430 (79 %) |
| Removed by fixing code                      | 112        |
| **Debt pass (2026-08-28) — newly enforced** |            |
| `wrapcheck` findings fixed in code          | **190**    |
| gosec G301 sites fixed in code              | **54**     |
| `goconst` findings fixed in code            | **49**     |
| `goconst` findings named-excluded           | **167**    |
| Still hidden after the debt pass            | **557**    |

The 542 → 112 step was configuration only. The 112 → 54 and 54 → 0 steps are the resolution pass
and the complexity pass recorded at the end of this document, and both were code. The 2026-08-28
debt pass is recorded in [Debt pass — 2026-08-28](#debt-pass--2026-08-28); it converted 293
config-hidden findings into fixed code and left 557 hidden.

**Be clear about what happened at the original triage: the reduction was entirely configuration.**
No Go source was touched. Three linters (`goconst`, `wsl_v5`, `noinlineerr`) accounted for 427 of
the 430, and the remaining 3 came from dropping `gocyclo`, which duplicates `cyclop`. The 112
findings that remained were the real work, and `just lint` was **not green** until they were
addressed.

**Be equally clear about what is still hidden after the debt pass.** `just lint` is green, but the
green is still partly bought: 196 `wsl_v5`, 167 `goconst`, 97 gosec G304 and 94 `noinlineerr`
findings are excluded rather than absent. Each has a reason recorded below, and every one of those
reasons is a judgement that a future reader is entitled to overturn.

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

> **These counts are a 2026-08-27 snapshot and are now stale.** They are kept because the rest of
> the document reasons from them. Re-measured uncapped against the 2026-08-28 tree
> (`max-issues-per-linter: 0`, `max-same-issues: 0`, `uniq-by-line: false`):
>
> | Linter        | 2026-08-27 | 2026-08-28 | Note                                                       |
> | ------------- | ---------: | ---------: | ---------------------------------------------------------- |
> | `goconst`     |        266 |    **509** | 216 of them non-test; the codebase grew                    |
> | `wsl_v5`      |        120 |    **196** | still disabled                                             |
> | `noinlineerr` |         41 |     **94** | still disabled; it _grew_ during the debt pass — why below |
> | `gocyclo`     |          3 |          3 | unchanged; still disabled as redundant with `cyclop`       |
> | `wrapcheck`   |        190 |        190 | the 2026-08-27 estimate was exact; all 190 now fixed       |
>
> The `noinlineerr` growth from 41 → 94 is not codebase drift alone. Converting a bare
> `return f()` into the wrapped two-step form `if err := f(); err != nil { return fmt.Errorf(…) }`
> introduces exactly the idiom `noinlineerr` objects to, so the `wrapcheck` work below actively
> increased this count. That is a fair trade — the wrapping is load-bearing, the objection is not —
> but it should be visible rather than discovered later.

## Per-linter decisions

### Disabled

Counts are as of 2026-08-28, re-measured uncapped. The 2026-08-27 figure is shown after the arrow
where it changed, because the reasoning below was written against it.

| Linter        |     Count | Decision    | Reason                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------- | --------: | ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `goconst`     | 266 → 509 | ~~Disable~~ | **Superseded:** re-enabled with tuning on 2026-08-28 — see below. 49 fixed in code, 167 named-excluded, the rest removed by `ignore-tests`.                                                                                                                                                                                                                                                                                                                       |
| `wsl_v5`      | 120 → 196 | **Disable** | Pure whitespace and statement-cuddling style. It cannot detect a defect by construction: every finding is "add or remove a blank line". At 196 findings it would dominate every review diff while catching nothing. `wsl` (v1) was already disabled in favour of `wsl_v5`; both are now off, and the now-dead `wsl_v5` settings block was removed. **Reason stands.**                                                                                             |
| `noinlineerr` |   41 → 94 | **Disable** | Forbids `if err := f(); err != nil { … }`. That form is idiomatic Go, is used throughout the standard library, and is _better_ than the alternative because it scopes `err` to the branch that handles it. Adopting the linter would mean 94 mechanical rewrites that each widen a variable's scope. This is a minority style opinion, not a correctness rule. **Reason stands** — but note the count more than doubled, and the `wrapcheck` work is part of why. |
| `gocyclo`     |         3 | **Disable** | Redundant. `cyclop` computes the same cyclomatic metric and is already enabled with an explicit `max-complexity: 15`. Keeping both means the same function is reported twice under two different thresholds. `cyclop` is the single cyclomatic gate. **Reason stands.**                                                                                                                                                                                           |
| `gomodguard`  |         0 | **Disable** | Deprecated since golangci-lint v2.12.0; see the migration note below.                                                                                                                                                                                                                                                                                                                                                                                             |

#### `goconst` in detail

> **Superseded 2026-08-28.** `goconst` is enabled again, tuned. See
> [`goconst` — re-enabled with tuning](#goconst--re-enabled-with-tuning) below. The analysis in
> this section still holds and is what the tuning is built on; only the "disable" verdict changed.

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

#### `goconst` — re-enabled with tuning

The disable above bought a green run by making 509 findings invisible, which is the largest single
item in the "debts a green `just lint` hides" list. It is now **enabled** with the following
settings, and the genuinely constant-worthy findings have been fixed in code.

Re-measured against the 2026-08-28 tree (the codebase grew; the 266 above is stale):

| Configuration                              | Findings |
| ------------------------------------------ | -------: |
| defaults, including test files             |  **509** |
| `ignore-tests: true`                       |  **216** |
| after fixing the constant-worthy findings  |  **167** |
| after the three documented exclusion rules |    **0** |

Settings, in `linters.settings.goconst`:

- `ignore-tests: true` — test fixtures repeat scenario names and expected strings by construction.
  This alone removes 293 of the 509 and is not debt.
- `min-occurrences: 3` (the default, set explicitly). Raising it silences the actionable
  3-occurrence findings first, because the highest-count strings are exactly the JSON output keys
  we deliberately keep literal. Re-measured on the 2026-08-28 tree with `ignore-tests: true` and no
  exclusion rules, so directly comparable to the 167 above: `min-occurrences` 4 → 78, 6 → 60,
  8 → 44, 10 → 29. Every one of those reductions is achieved by dropping the parameter-name
  triplication that group 2 documents as the real finding, while leaving the JSON keys in place.
  The 2026-08-27 measurement (4 → 120, 6 → 78, 8 → 53, 10 → 37, against the 266 baseline) reached
  the same conclusion.

Three exclusion rules in `issues.exclusions.rules`, each scoped by path and — where the path alone
would be too blunt — by the string value as well, so a _new_ repeated string in the same file is
still reported. That property was verified with a probe planted inside an excluded file, not
assumed; see the debt pass below.

1. **OpenAPI 3 wire vocabulary** (`internal/api/httpv1/openapi.go`, 39 findings). `type`, `string`,
   `$ref`, `properties`, `responses` and the rest of the OpenAPI keyword set repeat because the
   spec is built as nested `map[string]any`. The spellings are fixed by the OpenAPI specification,
   so a constant buys no typo protection. Path-scoped; that file contains nothing but the spec
   builder.
2. **CLI and standard-module parameter names** (`run_options.go`, `run_options_beb.go`,
   `run_extract_rls19.go`, `import_soundplan.go`, and
   `{schall03,iso9613}/{model,propagation,indicators}.go`; 99 findings). Group 2 above, unchanged: PLAN.md Priority 7's
   parameter-descriptor refactor owns the structural fix. Scoped to those files _and_ to the
   parameter-name vocabulary (`road_*`, `rail_*`, `traffic_*`, `speed_*`, `grid_*`,
   `air_absorption_db_per_km`, …).
3. **JSON output-contract keys** (`internal/app/cli/`, `internal/api/httpv1/`,
   `internal/report/export/`; 29 findings). Group 1 above, unchanged: `run_id`, `status`,
   `command`, `output_hash`, `receiver_count`, … are a public output contract emitted from
   `map[string]any` literals. Scoped to the emitting packages _and_ to the enumerated key set.
   PLAN.md Priority 7 (typed response payloads instead of map literals) owns the structural fix.

**Fixed in code** (49 findings, group 3 of the original analysis plus everything else that was
genuinely constant-worthy):

| Constants introduced                                                                                                                                                                      | Home                                         |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `TypeFeatureCollection`, `GeometryType{Point,MultiPoint,LineString,MultiLineString,Polygon,MultiPolygon}`, `FeatureKind{Source,Building,Barrier,Receiver}`, `SourceType{Point,Line,Area}` | `internal/geo/modelgeojson/types.go`         |
| `ArtifactKind{ModelNormalizedGeoJSON,ModelDumpJSON,ModelValidationReport,RunResult*}`, `ArtifactKindRunResultPrefix`, `ArtifactID{ModelNormalized,ModelDump,ModelValidation}`             | `internal/domain/project/model.go`           |
| `errorCode{BadRequest,NotFound,InternalError}`                                                                                                                                            | `internal/api/httpv1/handler.go`             |
| `commandNameCompare`; `commandNameBench`, `benchRunID`; `sampleIndicator{Lden,Lnight}`                                                                                                    | `internal/app/cli/{compare,bench,export}.go` |
| `defaultScenarioID`, `defaultStandardProfile`                                                                                                                                             | `internal/io/projectfs/store.go`             |
| `taskStatus{Passed,Skipped}` (next to the existing `taskStatusFailed`); `evidenceClass{Synthetic,Derived}`, `provenance{Synthetic,Derived}`                                               | `internal/qa/acceptance/`                    |
| `c1Effect{Schiene,Reflexion}`                                                                                                                                                             | `internal/standards/schall03/tables.go`      |

The GeoJSON tags are now sourced from `modelgeojson` in every importer
(`osmimport`, `gpkgimport`, `fgbimport`, `citygmlimport`) and in `internal/app/cli`, replacing the
per-file `geometryTypeLineString`-style locals that had started to accumulate.

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

> **All resolved as of 2026-08-28.** Every row below was closed by the resolution pass (2026-08-27)
> or the complexity pass, and re-verified against the working tree on 2026-08-28. The table is kept
> as the record of what was found and why it mattered; individual rows carry a resolution note
> where what happened differs from what was recommended. Nothing here is outstanding work.

These are the findings that indicate a defect or a genuine smell rather than a style preference.
19 findings across 6 linters, plus the security items.

### Correctness / API smells

| File:line                                                           | Linter      | Assessment                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `backend/internal/io/soundplanimport/gridmap.go:458`                | recvcheck   | **Not a defect — verified 2026-08-27.** `gridMapStatAccumulator` mixes receivers: `add` (`:487`) takes a pointer and mutates; `finish` (`:503`) takes a value and only reads, returning a `GridMapValueStats`. No mutation is discarded. **Resolved 2026-08-28** — the optional consistency fix was taken: `finish` is a pointer receiver now (`gridmap.go:504`). |
| `backend/internal/app/cli/compare_raster.go:86`                     | nilnil      | **Resolved.** Was: `prepareSoundPlanRasterCompare` returned `(nil, nil)` for "there are no grid maps, nothing to compare", so every caller had to nil-check a non-error nil. It now returns `(*rasterComparePreparation, bool, error)` (`compare_raster.go:156`). The `(*T, bool, error)` option was taken over the sentinel.                                     |
| `backend/internal/geo/modelgeojson/validate.go:282`                 | staticcheck | **Resolved** in `67d17b2`. `ST1005`: capitalised error string. Cosmetic in isolation, but this is user-facing validation output, so the inconsistency was visible. (The remaining capitalised strings in this file start with the GeoJSON type name — `LineString`, `Polygon` — which `ST1005` correctly permits as identifiers.)                                 |
| `backend/internal/app/cli/root.go:107`                              | unused      | **Resolved.** `newPlaceholderCommand` deleted; no occurrence remains in the tree.                                                                                                                                                                                                                                                                                 |
| `backend/internal/standards/cnossos/road/emission.go:344`           | unused      | **Resolved.** `mustFinite` deleted; no occurrence remains in the tree.                                                                                                                                                                                                                                                                                            |
| `backend/internal/standards/cnossos/industry/propagation.go:126`    | unparam     | **Resolved.** The unused `cfg` parameter is gone; `sourceDistance` is now `(receiver, source)` (`propagation.go:126`).                                                                                                                                                                                                                                            |
| `backend/internal/api/httpv1/handler.go:304`                        | unparam     | **Resolved by naming, not by fixing.** The parameter is now `_` (`handler.go:314`), which silences `unparam` while preserving the handler shape. The underlying point stands and is unaddressed: query parameters and request context are still dropped. Threading a request context into the store is a Priority 7 item.                                         |
| `backend/internal/app/cli/compare_raster.go:551`                    | unparam     | **Resolved.** The always-nil error return was dropped; the function now returns `modelgeojson.Model` (`compare_raster.go:615`).                                                                                                                                                                                                                                   |
| `backend/internal/app/cli/status.go:69`, `:92`                      | unconvert   | **Resolved.** Redundant conversions removed.                                                                                                                                                                                                                                                                                                                      |
| `backend/internal/app/cli/modelio_helpers.go:68`                    | predeclared | **Resolved.** Renamed; no `max`/`min` shadowing remains in these files.                                                                                                                                                                                                                                                                                           |
| `backend/internal/app/cli/run_options.go:437`, `:457`               | predeclared | **Resolved.** Renamed.                                                                                                                                                                                                                                                                                                                                            |
| `backend/internal/standards/framework/framework.go:336`, `:341`     | predeclared | **Resolved.** Renamed. (Correction retained: these were in `cloneParameterSchema`, not `validateScalar` — `validateScalar` dereferences `parameter.Min`/`.Max` directly and shadowed nothing.)                                                                                                                                                                    |
| `backend/internal/standards/schall03/emission_v2.go:132`, `:147`    | nolintlint  | **Resolved.** Both stale `//nolint:gosec` directives removed; `emission_v2.go` now carries none.                                                                                                                                                                                                                                                                  |
| `backend/internal/qa/acceptance/rls19_test20/runner.go:126`, `:127` | tagliatelle | **Resolved.** Tags are `lr_day` / `lr_night` (`runner.go:128`). The fixture check was done and cleared — see "Notes worth keeping" below; the goldens are package-owned, not an external TEST-20 format.                                                                                                                                                          |
| `backend/internal/standards/dummy/freefield/freefield.go:67`, `:83` | dupl        | **Resolved** in the complexity pass. (Correction retained: they were the `default` and `highres` profile parameter tables, which differ only in grid resolution and chunk size — not two halves of a computation.)                                                                                                                                                |
| `backend/internal/app/cli/compare_raster.go:524`, `:529`            | dogsled     | **Resolved as recommended.** Both sites carry a targeted `//nolint:dogsled // only one bound is needed here` (`compare_raster.go:588,593`).                                                                                                                                                                                                                       |
| `backend/internal/io/gpkgimport/gpkgimport_test.go:70`              | prealloc    | **Resolved.** Cosmetic, in a test.                                                                                                                                                                                                                                                                                                                                |
| `backend/internal/io/soundplanimport/terrain_text.go:67`            | gocritic    | **Resolved.** if-else chain rewritten as a `switch` — which pushed `LoadTerrainData` from 15 to 16 on `cyclop`; see "Notes worth keeping" below.                                                                                                                                                                                                                  |
| `backend/internal/app/cli/import_soundplan.go:242`                  | nestif      | **Resolved** in the complexity pass.                                                                                                                                                                                                                                                                                                                              |
| `backend/internal/assessment/bimschv16/assessment.go:380`           | funlen      | **Resolved** in the complexity pass.                                                                                                                                                                                                                                                                                                                              |

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
| `backend/internal/io/soundplanimport/geowand.go:181`                                                          | G115 | **Verdict corrected 2026-08-27 — false positive.** The conversion is a same-width reinterpretation of a fixed-width field, so no value can be lost and no bound is meaningful. A bounds check was attempted and had to be reverted: it rejected `0xFFFFFFFFFFFFFFFF`, which real project files genuinely carry, and made one malformed record discard every barrier in the file. The consumer already handles it — `app/cli/import_soundplan.go:327` treats a negative `MaterialCode` as "not set". Resolved with a `//nolint:gosec` carrying that reasoning.                                        |
| `backend/internal/report/export/gpkg.go:207`                                                                  | G202 | **False positive.** `createSQL` concatenates `sanitizeColumnName(indicator)`, and `sanitizeColumnName` (`gpkg.go:860`) rewrites every character outside `[a-z0-9_]` to `_` and substitutes `"value"` for the empty string. Injection is not reachable. Resolve with `//nolint:gosec // column names pass sanitizeColumnName allow-list` rather than restructuring.                                                                                                                                                                                                                                   |

### Suppressed by the exclusion presets — flagged for review

The 2026-08-27 table below recorded what the `common-false-positives` / `legacy` presets were
hiding. It is superseded by the 2026-08-28 measurement that follows it; the original is kept
because the G301 decision was taken against it.

| Rule                                | Hidden (non-test), 2026-08-27 | Recommendation as written on 2026-08-27                                                                                                                                                                                                                                                                                                                                          |
| ----------------------------------- | ----------------------------: | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| G304 (file inclusion via variable)  |                        **47** | **Keep suppressed.** This is a file-format toolchain; opening a path the user named is the entire product. Enabling it would produce 47 unactionable findings. The path-traversal question is real but belongs to the HTTP surface, where G703 already covers it.                                                                                                                |
| G301 (directory permissions > 0750) |                        **44** | **Worth a decision, not a blanket suppression.** All 44 are `os.MkdirAll(..., 0o755)` on project and artifact directories. `0o755` is fine for a local CLI. But this is a consistent, mechanical pattern: either standardise on `0o750` and re-enable the rule, or record the `0o755` choice as a deliberate project convention here. Right now it is neither — it is invisible. |
| G101 (hardcoded credentials)        |                             4 | Test-only, in `io/osmimport/osmimport_test.go` — fixture strings that pattern-match as tokens. Correctly suppressed.                                                                                                                                                                                                                                                             |
| G204 (subprocess with variable)     |                             1 | Same site as the G702 above; resolve together.                                                                                                                                                                                                                                                                                                                                   |
| G404 (weak RNG)                     |                             1 | `geo/geometry_property_test.go` — property-test seeding. Correctly suppressed.                                                                                                                                                                                                                                                                                                   |
| G306 (WriteFile perms)              |                             1 | Test-only. Correctly suppressed.                                                                                                                                                                                                                                                                                                                                                 |

#### Re-measured 2026-08-28

`gosec` run standalone with `linters.default: none`, `enable: [gosec]` and **no exclusions block at
all** — so `//nolint` directives are still honoured but no preset or rule is. 114 findings survive:

| Rule | Count | Where                                                                   | Why it does not reach `just lint`                                |
| ---- | ----: | ----------------------------------------------------------------------- | ---------------------------------------------------------------- |
| G304 |    97 | 50 in `_test.go`, 47 in non-test code                                   | Exclusion presets. **Still hidden — the count doubled from 47.** |
| G115 |    10 | `geo/terrain/terrain_test.go` ×6, `io/gpkgimport` ×3, `io/fgbimport` ×1 | The `_test\.go` exclusion rule                                   |
| G101 |     4 | `io/osmimport/osmimport_test.go`                                        | The `_test\.go` exclusion rule                                   |
| G703 |     1 | `io/soundplanimport/terrain_text_test.go:80`                            | The `_test\.go` exclusion rule                                   |
| G404 |     1 | `geo/geometry_property_test.go:12`                                      | The `_test\.go` exclusion rule                                   |
| G306 |     1 | `io/soundplanimport/terrain_text_test.go:80`                            | The `_test\.go` exclusion rule                                   |

Two corrections to attributions that were assumed rather than checked:

- **All 17 non-G304 findings are in `_test.go` files, without exception.** They are suppressed by
  the `_test\.go` exclusion rule alone, not by `//nolint` directives. Verified by listing every
  finding, not by inferring from the counts.
- **The 13 `//nolint:gosec` directives in `geo/terrain/geotiff.go` are not part of this 114.** A
  `//nolint` suppresses the finding before it is ever reported, so those 13 do not appear in a run
  with no exclusions either. They are explicit, individually justified suppressions with reasons
  attached, `nolintlint` confirms every one is still doing work, and that is exactly how G115
  should be handled. (A `//nolint` on parsed-from-file offsets deserves a periodic re-read to
  confirm the bounds claims still hold, but the mechanism is correct.)

**G304 remains the honest embarrassment here.** 97 findings, 47 of them in non-test code, still
invisible. The reason has not changed — this is a file-format toolchain and opening a path the user
named is the product — but the number has doubled since it was first waved through, which is what
happens to a suppressed rule in a growing codebase. G301, whose count grew the same way, turned out
to be worth fixing; nobody has re-litigated G304 on the evidence.

**G301 is resolved and now enforced** — see the debt pass below.

## Complexity hotspots

47 findings (31 `cyclop`, 16 `gocognit`).

> **Verdict corrected 2026-08-27.** This section originally read "**Do not restructure these
> individually** — most of them dissolve once PLAN.md Priority 7 lands, and pre-emptive splitting
> would collide with it." That was wrong on both counts, and the complexity pass below resolved
> all of them without the P7 refactor. Nearly every finding here is a long _procedural_ function,
> not a structurally coupled one: flat guard-clause chains in `Validate()` methods, giant `RunE`
> closures in the cobra constructors, and per-format or per-geometry-type dispatch. Extracting
> named helpers is behaviour-preserving and does not collide with P7 — if anything it makes the
> P7 work smaller, because the god-file split it calls for is now done.

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
   exclusion rule. (`goconst` was re-enabled with tuning on 2026-08-28; see the section above.)
3. **Re-enabled `sqlclosecheck`** — see below.
4. **Annotated the `wrapcheck` exclusion** with a `FIXME(lint-triage)` pointing here.
   (Superseded 2026-08-28: the exclusion is gone, so the `FIXME` is gone with it. See the debt
   pass below.)
5. **Dropped the `legacy` exclusion preset** (2026-08-28) and replaced it with four explicit rules
   that replay everything it provided except gosec G301. See the debt pass below.

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

> **Superseded 2026-08-28 — and the recommendation below was wrong.** The exclusion is removed and
> all 190 findings are fixed. The sequencing argument ("wait for `domain/errors`") did not survive
> contact with the work: see
> [`wrapcheck` — resolved](#wrapcheck--resolved) in the debt pass. The analysis of _what_ the
> exclusion was hiding is accurate and is left as written.

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

## Resolution pass — 2026-08-27

Steps 1–3 of the plan below are done: **112 → 54**, and this time by changing code. The 54 that
remain are exactly the complexity and file-length bucket, which stays blocked on PLAN.md
Priority 7.

| Bucket                                             | Was | Now | How                                                                         |
| -------------------------------------------------- | --: | --: | --------------------------------------------------------------------------- |
| Mechanical (`perfsprint`, `modernize`, `intrange`) |  26 |   0 | `golangci-lint --fix` per linter, plus the `errors` imports it does not add |
| Real defects and smells                            |  26 |   0 | Individually, below                                                         |
| Security                                           |   8 |   0 | Two real fixes, six justified suppressions                                  |
| Complexity and file length                         |  52 |  54 | Untouched — see below for why the count moved                               |

### Security

- **G702** (`api/httpv1/handler.go`) — fixed, not suppressed. `createRunRequest` had **no
  validation at all**: an unauthenticated caller could set `--input`, `--model`, `--param` and the
  standard identifiers to any string. Added `createRunRequest.validate`, called before the
  executor: identifiers and parameter names must match a fixed pattern, paths must satisfy
  `filepath.IsLocal` (rejecting absolute paths and `..` escapes) and must not begin with `-`,
  which would otherwise be parsed as a flag. Covered by
  `TestCreateRunEndpointRejectsArgumentInjection`. gosec's taint analysis cannot see the
  validation, so the exec site keeps a `//nolint` that names the validating function.
- **G120** (`api/httpv1/handler.go`) — fixed. `r.Body` is now wrapped in `http.MaxBytesReader`
  before `ParseMultipartForm`, whose argument only ever bounded the in-memory buffer. The rule
  fires on the call regardless, so the suppression points at the wrapper.
- **G703** ×4 (`app/cli`) — suppressed with per-site reasons. In each case the written path is the
  project root or the requested bundle directory joined with a **fixed** file name; the tainted
  value gosec follows is a path that is only ever read.
- **G202** (`report/export/gpkg.go`) — suppressed. `sanitizeColumnName` is a strict allow-list
  (anything outside `[a-z0-9_]` becomes `_`), so no caller-controlled text reaches the SQL.
- **G115** (`io/soundplanimport/geowand.go`) — verdict corrected, see above.

### Notes worth keeping

- **`tagliatelle` was safe to fix.** The `LrDay`/`LrNight` keys appear only in
  `qa/acceptance/rls19_test20/testdata/ci_safe/*.golden.json`, which this package owns and
  regenerates; they are not an external TEST-20 format. Renamed to `lr_day`/`lr_night` and the 34
  golden files updated with them.
- **`unparam` on `handleRunsList`** was resolved by naming the parameter `_`, keeping the handler
  shape intact. Threading a request context into the store is a separate Priority 7 item.
- **The complexity count moved 52 → 54 and that is expected.** Two of the fixes traded one
  finding for another: rewriting the `gocritic` if-else chain in `io/soundplanimport/terrain_text.go`
  as a `switch` pushed `LoadTerrainData` from 15 to 16 on `cyclop`, and resolving the `nilnil`
  finding in `app/cli/compare_raster.go` added a boolean return. Both are better code; `cyclop`
  simply counts a `switch` case and an extra return differently. `prepareSoundPlanRasterCompare`
  went 19 → 18 in the same change.

## Complexity pass — 2026-08-27

The 54 findings left after the resolution pass were all complexity or file length. They were
resolved by refactoring, in six parallel passes partitioned by package so no two touched the same
file.

| Area                                                        | Findings | Shape of the fix                                                       |
| ----------------------------------------------------------- | -------: | ---------------------------------------------------------------------- |
| `app/cli` cobra constructors and god files                  |       10 | `RunE` closures extracted; `run_extract.go` and `run_options.go` split |
| `app/cli` import, compare and bench                         |       10 | Procedural phases extracted into named helpers                         |
| `standards/{cnossos,bub,buf,beb}` `Validate()` and emission |       15 | Guard chains grouped; two switch trees became lookup tables            |
| `standards/{framework,rls19,schall03}`                      |        5 | `validateScalar` split per kind; Tabelle 2 became a lookup table       |
| `geo/modelgeojson`, `report/*`, `engine`, `assessment`      |       10 | One helper per geometry type; marching-squares cases extracted         |
| `io/soundplanimport`, `qa/acceptance`                       |        4 | Per-input-source loader stages extracted                               |

Two further findings surfaced during the pass (`cyclop` on `runCompare`, `funlen` on
`prepareSoundPlanRasterCompare`). Both were pre-existing, not introduced: `golangci-lint` dedups
issues by line, so they had been hidden behind the finding reported on the same line. **The 54 was
a floor, not a total** — worth remembering when estimating any future lint backlog from a count.

### File splits

| File                      | Before | After | New files |
| ------------------------- | -----: | ----: | --------: |
| `run_extract.go`          |  3 087 |    89 |         6 |
| `rls19/road/road_test.go` |  3 375 | 1 139 |         2 |
| `run_phase8_test.go`      |  1 587 |   907 |         1 |
| `run_options.go`          |  1 604 | 1 188 |         2 |

This completes the "split the god files" item under PLAN.md Priority 7 for the two non-test files
the linter flagged. `run_persist.go`, `api/httpv1/handler.go`, `report/reporting/report.go` and
`app/cli/export.go` are still large but sit under the 1 500-line limit, so they remain P7 work
rather than lint findings.

### How the refactor was verified

Behaviour preservation was checked against a pristine worktree at the parent commit, not by
inspection alone:

- **69 golden snapshots byte-identical.** `UPDATE_GOLDEN` was never set. This is the primary
  evidence that the contour and marching-squares refactor did not perturb output.
- **843 tests, identical set.** `go test -list` output diffed against the baseline, so the four
  file splits provably did not drop, rename or duplicate a test.
- **Full suite passes**, `golangci-lint` reports **0 issues**, `treefmt` clean.
- The two normative lookup tables converted from `switch` statements were re-verified value by
  value against the baseline: all 24 RLS-19 Tabelle 2 coefficients and all 40 CNOSSOS road
  correction values are unchanged. In the CNOSSOS case the old `default:` branches also caught
  unknown vehicle classes, where a map miss now yields 0 — this is unreachable, because
  `vehicleClass` is unexported and the only call site passes one of four constants.

## Debt pass — 2026-08-28

The 2026-08-27 passes ended green, and the closing section below admitted the green was partly
bought: `wrapcheck` off for the whole backend, 430 findings removed by switching linters off, gosec
G301 suppressed by a preset. This pass paid down three of those and left the rest, honestly named.

| Item                     | Was                                    | Now                                                     |
| ------------------------ | -------------------------------------- | ------------------------------------------------------- |
| `wrapcheck`              | excluded for `internal/` — 190 hidden  | **enforced**; all 190 fixed in code, 0 suppressions     |
| gosec G301               | hidden inside the `legacy` preset — 54 | **enforced**; all 54 sites changed to `0o750`           |
| `goconst`                | disabled — 509 hidden                  | **enabled, tuned**; 49 fixed, 167 named-excluded        |
| `wsl_v5` / `noinlineerr` | disabled — 120 / 41 hidden             | still disabled — 196 / 94 hidden. Reasons unchanged     |
| `gocyclo`                | disabled — 3 hidden                    | still disabled, redundant with `cyclop`                 |
| gosec G304               | hidden by the presets — 47             | still hidden — **97**. Reason unchanged, number doubled |

### `wrapcheck` — resolved

The blanket `- linters: [wrapcheck] path: internal/` exclusion is deleted. All 190 sites are
wrapped as `fmt.Errorf("<operation>: %w", err)`. `just lint` is still at 0 issues with the
exclusion gone.

Origin of the 190, measured from the pre-fix JSON output:

| Origin                   | Count | Detail                                                                                                               |
| ------------------------ | ----: | -------------------------------------------------------------------------------------------------------------------- |
| In-module, cross-package |   115 | `internal/io/projectfs`, `internal/report/results`, `internal/standards/*`, `internal/engine`, `internal/assessment` |
| Standard library         |    62 | `database/sql` 21, `os` 18, `encoding/json` 14, `path/filepath` 3, `fmt` 2, `io` 2, `io/fs` 1, `strconv` 1           |
| Third party              |    13 | `github.com/gogama/flatgeobuf`                                                                                       |

Fixed in five disjoint shares, partitioned by package so no two touched the same file: `app/cli`
52; `report/export` + `geo` 29; `io` + `engine` + `api` + `qa` + `app/config` 26;
`standards/beb` + `standards/bub` 36; `standards/{buf,cnossos,rls19,schall03,iso9613}` 47.

**Zero `//nolint:wrapcheck` escapes were needed.** That is the result worth recording, because the
usual objection to enforcing `wrapcheck` is that sentinel pass-throughs have to be exempted. No
site turned out to be one. The two candidates were checked individually: `fgbimport`'s `io.EOF`
loop terminator and `engine/runner.go`'s `context.Canceled` handling were both already comparing
with `errors.Is`, which sees through `%w`, so wrapping was safe at both.

**The 2026-08-27 recommendation was wrong, and it is worth saying why.** It read: wait for the
`domain/errors` sentinel work in PLAN.md Priority 7, because "wrapping errors that are about to be
replaced by sentinels is wasted work". Two things were missed. First, `%w` preserves the
`errors.As(&AppError)` classification that drives the CLI exit codes, so the wrapping does not
conflict with typed errors — Priority 7 can still convert these sites later, on top of the
wrapping rather than instead of it. Second, the sequencing argument had no end date attached, so in
practice it meant "never": the exclusion had already survived one triage with a `FIXME` on it, and a
`FIXME` on a blanket exclusion is not a plan.

### gosec G301 — resolved and now enforced

All 54 `os.MkdirAll(..., 0o755)` call sites became `0o750` — 44 in non-test files, 10 in test
files. (55 `MkdirAll` sites exist; one was already `0o750`.) This settles the decision the
2026-08-27 table asked for and refused to take: standardise on `0o750`, do not record `0o755` as a
convention.

Enforcing the rule was harder than fixing the code, and the reason is worth recording. **G301 was
not suppressed by anything in this project's config.** It was inside the `legacy` exclusion preset,
whose `EXC0009` entry bundles G301 together with G302 and G307 under a single regex:

```
(G301|G302|G307): Expect (directory permissions to be 0750|file permissions to be 0600) or less
```

A preset entry cannot be partially disabled, so there was no way to enforce G301 while keeping
`legacy`. The fix: drop `legacy` from `exclusions.presets` and replay it as four explicit
`exclusions.rules` — EXC0004 (govet), EXC0005 (staticcheck SA4011), EXC0008 (gosec G104) and
EXC0009 **minus G301**. Only G301 changes behaviour; nothing else does.

Enforcement was proven, not assumed: a throwaway `os.MkdirAll("x", 0o755)` probe was planted and
produced the expected G301 finding, then removed.

**One measured aside that the replay rules deserve to have attached to them.** With `legacy`
dropped and nothing replayed at all, the tree is also at 0 issues — verified by running the full
config with the four replay rules deleted. None of EXC0004, EXC0005, EXC0008, G302 or G307
currently fires anywhere in the backend. The four rules are insurance against a future finding, not
load-bearing today. Anyone tempted to simplify the config by deleting them should know they are
deleting a guard, not dead weight.

### `goconst` — re-enabled with tuning

Recorded in full under
[`goconst` — re-enabled with tuning](#goconst--re-enabled-with-tuning) above, with the settings,
the three exclusion rules and the constants introduced. The short version: 509 → 216 by
`ignore-tests`, 216 → 167 by fixing 49 findings in code, 167 → 0 by three exclusion rules scoped by
path **and** by string value.

The value-scoping matters and was verified rather than assumed: a probe repeating a new string
three times was planted _inside_ an excluded file (`run_options.go`) and still produced a `goconst`
finding. A path-only exclusion would have swallowed it. The three rules exclude the vocabulary that
was argued about, not the files it lives in.

**167 findings are still excluded, and that is not a fix.** Groups 2 (99) and 3 (29) both point at
PLAN.md Priority 7 for the structural work — the parameter-descriptor refactor and typed response
payloads respectively. Until Priority 7 lands, the triplication and the untyped `map[string]any`
output contract are exactly as present as they were when the linter was off; the only thing that
changed is that they are now named, bounded and pointed at an owner. Group 1 (39, the OpenAPI
keyword set) is the one exclusion here that is genuinely permanent.

### What this pass did not touch

- **`wsl_v5`, 196 findings, still disabled.** Pure whitespace and cuddling style; it cannot detect
  a defect by construction. The reason stands at 196 exactly as it stood at 120.
- **`noinlineerr`, 94 findings, still disabled.** It forbids the idiomatic
  `if err := f(); err != nil`. The reason stands. The count grew from 41, and part of that growth
  is this pass's own doing — see the note under the baseline distribution.
- **`gocyclo`, 3 findings, still disabled.** Redundant with `cyclop`. The reason stands.
- **gosec G304, 97 findings, still hidden by the exclusion presets.** The reason stands — this is a
  file-format toolchain and opening a path the user named is the product — but the number has
  doubled since it was first waved through, and nobody has re-examined it on the current evidence.
  47 of the 97 are in non-test code.

## Where this leaves `just lint`

**Green, and the green means more than it did yesterday.** `golangci-lint` reports 0 issues across
`backend/...` with `wrapcheck` enforced, `goconst` enabled and gosec G301 enforced. `just lint` is a
hard merge gate.

It still overstates the coverage, and by how much is now measurable rather than vague:

| Still hidden  | Findings | Mechanism                            | Reason                                                              |
| ------------- | -------: | ------------------------------------ | ------------------------------------------------------------------- |
| `wsl_v5`      |      196 | `linters.disable`                    | Whitespace style; cannot detect a defect                            |
| `goconst`     |      167 | three named, path+value-scoped rules | 39 permanent (OpenAPI), 128 owned by PLAN.md Priority 7             |
| gosec G304    |       97 | `exclusions.presets`                 | Opening a user-named path is the product; 47 of the 97 are non-test |
| `noinlineerr` |       94 | `linters.disable`                    | Forbids idiomatic `if err := f(); err != nil`                       |
| `gocyclo`     |        3 | `linters.disable`                    | Redundant with `cyclop`                                             |

That is 557 findings a green run does not show you, down from 1 143 before this pass but not down
to zero and not on a path to zero. Three of those five rows are style opinions this
project has deliberately declined, and they are stable. The other two are not: `goconst`'s 128 and
G304's 97 are both waiting on work someone has to actually do, and both counts grow with the
codebase. A suppressed rule's count is a debt that accrues interest silently — G301 went 44 → 54
while nobody was looking, and G304 went 47 → 97.

The one thing that is no longer true of this config: **no linter is switched off across the entire
backend any more.** Every remaining exclusion names either a rule the project has argued about in
writing, or a path and a string value. That was the specific failure mode the `wrapcheck` exclusion
represented, and it is gone.

## Reproducing these numbers

```bash
just lint                       # current state
cd backend && golangci-lint config verify -c ../.golangci.yml
```

To measure a suppressed rule, run the linter standalone with `linters.default: none`, a single
`enable:` entry and no `exclusions.presets` — that is how the G304/G301, `goconst` and `wrapcheck`
counts in this document were obtained. Always with `max-issues-per-linter: 0`,
`max-same-issues: 0` and `uniq-by-line: false`; without those three the number you get is a
readable summary, not a count, and it will be too low.

**Run the measurement from `backend/`, not from the repo root.** From the root, `golangci-lint`
reports `directory prefix . does not contain main module` on stderr and then still prints
`0 issues.` — a false green that will make a suppressed rule look resolved. Every number in this
document was measured from `backend/`.

```bash
cd backend
cat > /tmp/m.yml <<'YAML'
version: "2"
run: {timeout: 10m}
issues: {max-issues-per-linter: 0, max-same-issues: 0, uniq-by-line: false}
linters:
  default: none
  enable: [gosec]        # or goconst, wsl_v5, noinlineerr, gocyclo, wrapcheck
YAML
golangci-lint run -c /tmp/m.yml ./...
```
