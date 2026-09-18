# PLAN.md — Aconiq Roadmap

Status: 28 August 2026

This file tracks **what is still ahead**. Completed work is recorded in git history, in
`docs/conformance/`, and in the per-module baseline notes under `docs/`. Nothing here is a
status report.

Ordering principle: correctness and evidence come before features. A calculation that is wrong
by 23 dB is not improved by a nicer report template.

The "Phase QR" (quality remediation) block added in `9ec398d` has been absorbed here. Its
sub-phases map onto the new numbering as: QR-1 → Priority 0, QR-2 → Priorities 2 and 4,
QR-3 → Priority 1 (determinism), QR-4 → Priority 3, QR-5 → Priority 7, QR-6 → Priorities 5
and 9. Commit messages referencing `QR-N` remain resolvable through this table.

## Strategic positioning

Aconiq is an auditable, deterministic noise calculation and automation platform, not a GUI clone
of CadnaA, SoundPLAN, or IMMI. Core differentiators:

- Deterministic, reproducible runs with artifact provenance and golden-test regression.
- CLI-first plus local API for automation, CI/CD integration, and batch workflows.
- Open standards modules as plug-ins with explicit compliance boundaries per norm.
- Offline-first project format with full traceability from inputs to standard/profile to outputs.

The path to DACH adoption runs through four gates, in order:

1. **Numbers that are right, and demonstrably so.** Currently the weakest link.
2. Legal clarity and compliance boundaries.
3. Real-world CRS and interoperability support.
4. DACH-specific assessment and reporting workflows.

## Clarifications

- Offline-only is acceptable for the near-term MVP. The CLI is primary; the local API and
  browser GUI are secondary.
- Input data support covers GeoJSON, GeoPackage, FlatGeobuf, CSV, CityGML, OSM/Overpass, and
  GeoTIFF terrain.
- All named standards remain long-term targets, but they do not all carry the same delivery
  priority — and, per the tier table below, they do not all carry the same evidence.
- The frontend stack remains React + TypeScript + Vite + Bun + shadcn/ui. Frontend _polish_ is
  not on the critical path; frontend _correctness_ is, because the shipped Run page throws.

## Guiding principles

- Separate generic acoustics/geometry/compute core from standards modules.
- Treat quality assurance as a product feature, not a cleanup task.
- Publish conformance boundaries, tolerances, known deviations, and evidence per normative module.
- Keep the project format local-first; multiuser/server remains optional future work.
- **Never let a module's name assert more than its evidence supports.**

## Working definitions

- **Project**: folder with manifest, inputs, and artifacts.
- **Scenario**: input model plus standard selection plus parameters.
- **Run**: one calculation of one scenario against one receiver set with a fixed standard
  version/profile.
- **Standards module**: implementation of emission, propagation, indicators, and tables for a
  specific standard and version/profile.
- **Evidence tier**: how much a module's output may be relied upon. A required descriptor field
  since Priority 4; see the tier table above.

## Standards evidence tiers

The registry exposes 13 standards. They are not peers, and since Priority 4 the code says so:
`EvidenceTier` is a required field on `framework.StandardDescriptor`, enforced by `Validate()`, so
an unlabelled module cannot be registered at all. This table is the declared state, and
`internal/standards/standards_test.go` pins it value by value.

| Module                                | Target tier            | Reality today                                                                |
| ------------------------------------- | ---------------------- | ---------------------------------------------------------------------------- |
| `rls19-road`                          | normative              | Real Eq. 4/6 structure and coefficients; length weighting fixed (`4142444`)  |
| `schall03`                            | normative              | Anlage-2 tables correct, and `aconiq run` now calls them (`schall03_engine`) |
| `iso9613`                             | normative              | Table 2/3 verbatim correct; three defects fixed (`775c7f5`)                  |
| `talaerm`, `bimschv16`                | normative (assessment) | Threshold tables and logic sound                                             |
| `beb-exposure`                        | preview                | Aggregation logic reasonable; consumes preview levels                        |
| `cnossos-road/rail/industry/aircraft` | **scaffold**           | No directive coefficients. Invented base levels, no octave bands             |
| `bub-road`                            | **scaffold**           | Re-parameterised clone of the CNOSSOS scaffold                               |
| `bub-rail`, `bub-industry`            | **scaffold**           | Pure aliases over `cnossos/*`                                                |
| `buf-aircraft`                        | **scaffold**           | Same compute path as `cnossos/aircraft`; seven descriptor defaults differ    |
| `dummy-freefield`                     | test fixture           | Intentional                                                                  |

Two claims in this table were overstated and are corrected above. `bub-rail`/`bub-industry` are
aliases for all _acoustic_ purposes but carry their own descriptor and `ExportResultBundle`, and
`buf-aircraft` is byte-identical to `cnossos/aircraft` only in `compute.go`/`emission.go` — seven
descriptor default parameter values differ, and those do move numbers for imported sources.
Scaffold-tier modules now require an explicit `--experimental` opt-in on `aconiq run`; the
boundaries are published in `docs/conformance/cnossos-umfangserklaerung.md` and
`docs/conformance/beb-umfangserklaerung.md`.

---

# Gate 1 — Trustworthy build and numbers

## Priority 0 — Restore the build and the safety net

Nothing else on this roadmap is verifiable until this is done. Every automated gate the project
owns was blind for months: one unresolvable module poisons whole-graph resolution, and
`golangci-lint` aborts an entire run on a single typecheck error.

### Landed

Kept as a checklist only because these are the gates everything else depends on. Detail is in the
commit messages; the consequences each one exposed are open items below.

- [x] **Module-path mismatch fixed** (`622a8ce`). `go.mod` required `cwbudde/go-overpass` at a
      pseudo-version whose own `go.mod` declared the path as `MeKo-Christian/go-overpass`.
      Upstream moved _back_ to `cwbudde`, so the fix was `require` → `v0.1.0` plus a matching
      import, not the reverse. `go build` and `go vet` are green; `internal/app/cli`,
      `internal/api/httpv1`, `cmd/aconiq` and `internal/io/osmimport` compile and test again.
- [x] **The one test the restored build exposed** (`804ec3d`). The heuristic raster receiver test
      in `internal/app/cli` asserted a geometrically unreachable `x = 0` — the zero value returned
      alongside `ok=false`. The test was wrong, not the code, so raster comparison output is
      unchanged. The boundary weakness it revealed is tracked under Priority 13.
- [x] **CI tooling breakers** (`b96e319`, Phase QR-1): `bin/noise` → `bin/aconiq` in `go-ci.yml`
      and `CONTRIBUTING.md`, `mkdir -p docs/api` before the OpenAPI export, the `treefmt` install,
      `golangci-lint-action` v6 → v7 (v6 pins the v1 line, which refuses a `go 1.25.0` target and
      cannot read the v2 `.golangci.yml`), and the Go 1.24+ → 1.25+ correction.
- [x] **Formatting** (`0a234e2`). Five files were unformatted on `main`, so `check-formatted` went
      red before any other gate could run. `just check-formatted` now exits 0.
- [x] **Frontend typecheck no-op fixed** (`f2b60f7`). See the open item below for the 55 errors it
      turned out to be hiding.
- [x] **Lint triage** (`0a13e80`). 542 → 112, audited in `docs/lint-triage.md`. The reduction was
      configuration, not fixes.
- [x] **Lint resolution pass.** 112 → 54, and this time by changing code: 26 mechanical, 26 real
      defects, 8 security. Two of the security findings were real fixes rather than suppressions —
      `createRunRequest` reached `exec.CommandContext` with **no validation at all**, so an
      unauthenticated caller could choose arbitrary `--input`/`--model` paths, and the terrain
      upload's request body was unbounded. See `docs/lint-triage.md` for the per-finding record,
      including two verdicts the pass disproved.
- [x] **Lint gate hardened.** `issues.uniq-by-line` is now off, so a finding can no longer hide
      behind another on the same line — the default that made the complexity backlog look smaller
      than it was. Dropped the justfile's `--timeout=2m`, which silently overrode
      `.golangci.yml`'s `timeout: 5m`: a cold CI run must fail on findings, not on a timeout only
      the justfile knew about. Verified the gate actually trips by planting a deliberate finding
      (`just lint` exits 1). Both changes are free today — the tree is at 0 either way.
- [x] **Lint complexity pass.** 54 → **0**. The triage's own verdict ("do not restructure these
      individually — they dissolve once Priority 7 lands") turned out to be wrong: these were long
      _procedural_ functions, not structurally coupled ones, and extracting named helpers resolved
      every one without the P7 refactor. Verified against a pristine worktree — 69 golden
      snapshots byte-identical, the 843-test inventory unchanged, and the two normative lookup
      tables re-checked value by value. This also lands P7's "split the god files" for
      `run_extract.go` (3 087 → 89 lines) and `run_options.go`.
- [x] **Lint audit pass** (`7babf60`). `exclusions.presets` is gone; the 41 `errcheck` findings it
      hid are fixed in code, so `errcheck` is enforced at 0 with no exclusion. gosec G304 keeps its
      exclusion as one explicit named rule. The gate's composition is now what
      `docs/lint-triage.md` says it is — nothing is suppressed by a set nobody enumerated. Two
      beliefs it disproved: `wsl_v5` has been enforced since `4b0566c`, not disabled, and a
      preset's contents count as debt even when no table names them.
- [x] **Unguarded fixture tests** (`bda9767`). Eight tests in `io/soundplanimport` hard-failed on a
      clean checkout over a gitignored fixture; they now skip with the resolved path and reason.
- [x] **CI tool versions pinned** (`e183c68`), matched to the local toolbox so `just go-ci` and CI
      agree on what "passing" means.
- [x] **`-race` and `govulncheck` wired into CI** (`e183c68`, `73f06c7`), as separate parallel jobs
      since both are slow and independent.
- [x] **`just ci` reconciled with `go-ci.yml`** (`e183c68`). Every gate now invokes a `just`
      recipe; the two remaining non-recipe steps produce artifacts rather than gate, and say so in
      both places.
- [x] **Repository hygiene** (`91ffcc1`): the committed `tsbuildinfo`, the empty
      `backend/cmd/noise/` and root `.codex`, plus gitignores for `.trunk/`, `playwright-report/`
      and `test-results/`.
      **This entry's first claim was wrong and is corrected here: `backend/wasm` is still tracked.**
      `91ffcc1` never touched it; `f614faa` (#19) is the last commit that did, and the file is
      neither deleted nor gitignored. It is now 4.0 MB. Worse, it is a live footgun rather than dead
      weight: `go build ./cmd/wasm` run from inside `backend/` writes its output straight over it,
      so any contributor who builds the kernel the obvious way silently stages a 4 MB binary diff.
      Deleting it and gitignoring the path is the fix, and it is a decision about a tracked artifact
      rather than a cleanup, so it is named here rather than done in passing.
- [x] **The 55 frontend type errors the honest typecheck exposed** (`be19871`). All fixed; the
      project type-checks clean. Four were the shipped crashes predicted in Priority 8
      (`run.tsx:969`, `run.tsx:890`, `map-view.tsx:88-89`, `new-feature-dialog.tsx:136`), and the
      rest were `noUncheckedIndexedAccess` / `exactOptionalPropertyTypes` fallout plus a missing
      `window.Go` ambient declaration. No `!`, `as`, or `@ts-ignore` was used to close any of
      them. Two latent defects fell out of the work and are recorded below.
- [x] **The 49 `eslint .` findings** (`e55e366`). `bun run lint` passes, so **`frontend-ci.yml`
      is green** — it runs typecheck, lint, test and build, and not `bundle-check`, which still
      fails on the chunk budget in Priority 8. Most of the 29 `restrict-template-expressions`
      findings were `String(...)` wraps, but three groups were fixed by hoisting a duplicated
      expression instead (the Overpass bbox appeared four times in one query, the OSM feature id
      four times in one function, `Date.now()` three times in one export). Two
      `no-unnecessary-condition` findings were pointing at type lies rather than dead guards, and
      the guard was the correct half in both: `readState` cast untrusted `localStorage` JSON to a
      complete `BrowserBackendState`, and `sanitizeOverpassGeometry` declared a non-nullable
      element type while filtering an untrusted Overpass response. `no-misused-spread` was a real
      bug — in `api/client.ts`, `...init` came after the header merge and replaced `headers`
      wholesale, dropping `Accept: application/json` from every request that set headers of its
      own. `require-await` is disabled file-wide in `browser-backend.ts` with a reason: those
      methods must stay `async` so a synchronous `throw` reaches callers as a rejected promise.
      `scripts/check-bundle-size.mjs` had never been linted at all — it sits outside every
      tsconfig, so the project service failed to parse it.
- [x] **Frontend package-manager consolidation finished.** The repo had a second, npm-managed
      island at the root — `package.json`/`package-lock.json` pinning `@axe-core/playwright`, an
      untracked `node_modules/`, `playwright.config.ts` starting the dev server with
      `cd frontend && npx vite`, and `just fe-e2e` running `npx playwright test` — while every
      workflow and every `fe-*` recipe uses Bun. `just fe-e2e` worked on one machine only because
      that root `node_modules/` happened to exist and `npx` fell back to an unpinned npm download.
      `@axe-core/playwright` moved into `frontend/package.json`, `playwright.config.ts` and `e2e/`
      moved under `frontend/`, `fe-e2e` became `cd frontend && bun run test:e2e`, and both root
      files are gone. Two claims in the item as written were already stale: `@playwright/test`
      **was** declared in `frontend/package.json` and pinned in `bun.lock`, and a
      `"test:e2e": "playwright test"` script already existed — `@axe-core/playwright` was the sole
      genuinely root-only dependency.
      Moving the suite under `frontend/` put it inside `eslint .`'s reach for the first time, so
      rather than adding an ignore, a `tsconfig.e2e.json` now covers `e2e/` and
      `playwright.config.ts`: the suite is type-checked and type-aware-linted, where previously it
      was checked by **nothing** (no tsconfig included it, and no eslint config exists at the
      root). That immediately caught one real `exactOptionalPropertyTypes` violation —
      `workers: process.env.CI ? 1 : undefined` is now a conditional spread. The move also pulled
      `e2e/*.spec.ts` into vitest's default include, so `vitest.config.ts` now pins
      `include: ["src/**/*.{test,spec}.{ts,tsx}"]`; the unit-test count is unchanged at 112.

- [x] **Lint debt pass** (`4b0566c`). The three exclusions that switched a linter off across the
      whole backend are gone and the 293 findings they hid are fixed rather than re-suppressed:
      `wrapcheck` (190 sites wrapped, zero `//nolint` escapes), gosec **G301** (54 `MkdirAll`
      sites `0o755` → `0o750`; it sat inside the `legacy` preset's EXC0009 bundled with G302/G307
      and was not separable, so `legacy` was replayed as four explicit rules), and `goconst`
      (509 → 0, of which 49 by fixing code). **No linter is switched off across the entire backend
      any more.** The remaining hidden counts are named and bounded in the open item below.

- [x] **The tree builds without private-repo credentials** (`1cbda38`).
      `github.com/cwbudde/go-absolute-database` was private, not missing — which is why gating
      `.abs` reading behind a build tag would not have helped, since an unresolvable `require`
      poisons whole-graph resolution regardless. The repository is public now and `go.mod`
      requires `v0.1.2`. Cold-cache `go mod download` against the public proxy succeeds without
      credentials; all three first-party dependencies are public.

- [x] **Branch protection is on for `main`.** Enabled as a repository **ruleset**
      (`default-branch`, id `21682143`), not through the classic branch-protection API. That
      matters for anyone checking it: `gh api .../branches/main/protection` still answers
      **404**, which means "no classic protection", not "unprotected" — read
      `gh api repos/CWBudde/Aconiq/rules/branches/main` instead. It requires all four checks and
      a pull request, blocks deletion and force-pushes, and has no bypass actors, so it binds the
      owner too. `strict_required_status_checks_policy` is deliberately off.

- [x] **The merge gate is green again** (`48b63b2`). Both checks that failed the moment the
      ruleset went live were infrastructure, not debt. Two traps worth not rediscovering:
      `actions/setup-go` resolves `go-version-file` to go.mod's `go` directive and **ignores
      `toolchain`**, which bites only binaries built by `go install tool@version` (everything run
      inside `backend/` re-execs under `GOTOOLCHAIN=auto`); and `frontend/src/i18n/` is gitignored
      Paraglide output that only the Vite plugin generated, so `tsc` in CI saw nothing until a
      `compile:i18n` CLI step was added. That step's project, outdir and strategy must stay in
      sync with `paraglideVitePlugin` in `vite.config.ts`. The frontend carries **no** residual
      type debt — the old "55 TypeScript errors" framing was wrong, and Priority 8 inherits none.

- [x] **`frontend-ci` no longer deadlocks on path filtering** (`48b63b2`, `585f2e0`). A required
      check that is skipped by path filtering never reports, so a docs-only pull request would
      have waited forever; its `pull_request` trigger is now unfiltered. `push` was narrowed to
      `[main]` in `go-ci.yml`, `frontend-ci.yml` and `repo-hygiene.yml`, which also ended every
      workflow running twice per pull request. The deadlock fix is **not yet exercised**: PR #5
      touches `frontend/`, so it would have run under the old filter too — the first genuinely
      docs-only pull request is the real test.

- [x] **The formatting gate is read-only** (`68b72a5`). `just check-formatted` mirrors the files a
      commit could contain into a temporary directory, formats the copy and diffs it back;
      `treefmt --fail-on-change` formats in place and is no longer used as a check. Anything that
      grows a formatting step inherits the same obligation — treefmt writes wherever it is pointed.
- [x] **One pinned toolchain** (`68b72a5`). `tools.versions` is the only place a tool version is
      written. Every workflow reads it through `.github/actions/toolchain`, and
      `just install-tools` installs exactly those versions, including the two awkward cases:
      treefmt, which no `go install` will accept (a fixture path in its module zip contains an
      emoji), and govulncheck, which has to be built with the toolchain `backend/go.mod` names.
      `just fmt` and `just check-formatted` refuse to run on a formatter mismatch and treefmt no
      longer gets `--allow-missing-formatter`, so neither a wrong nor a missing formatter can pass
      silently — which is what had let prettier 3.9.6 rewrite files the pinned 3.8.0 calls correct.
      Live constraints: bump a version in `tools.versions` and nowhere else, and keep `TOOLS_BIN`
      ahead of other copies on `PATH`, because the runner image ships its own shellcheck.
      This also closed the `golangci-lint` skew item, on a false premise: `.trunk/` is gitignored
      and was never tracked, so its 2.11.4 pin is one developer's local tooling and never was a
      third pin in the repository. The scanners configured there are still unused — Priority 9.
- [x] **`govulncheck` blocks, and a daily scan finds advisories before they block** (`d36c544`).
      The decision, the rejected alternative and the red-button procedure are in
      `docs/policies/vulnerability-scanning.md`: it reports reachable calls rather than mere
      presence, so it gates merges, and the job now also runs daily on `main` and files an issue on
      failure. Live constraint: govulncheck has no ignore file and none is to be added — an
      advisory with no fixed version has to be handled by not reaching the symbol, written up in
      that policy.

- [x] **The lint debt is down to what is declared** (`8884ea3`, and the run-module dispatch).
      **244** findings are still suppressed, every one named and defended in
      `docs/lint-triage.md`: `goconst` 62 (39 permanent, 23 owned by Priority 7's typed response
      payloads), `noinlineerr` 104 and `gocyclo` 4 (both declined in writing), gosec G304 47
      non-test, and 27 `//nolint` directives — 18 named `gosec`, 8 genuine `dupl` coefficient
      tables in `schall03/beiblatt1.go`, one `tagliatelle`. From 557 when this item opened, and the
      reduction was code rather than configuration: no linter is disabled tree-wide and no
      exclusion rule was added to close it.
      **`internal/app/cli` no longer carries a single complexity or duplication suppression.** Its
      four remaining directives are `gosec`, each naming its rule and its reason.
      Live constraint: `.golangci.yml`'s disable list holds 19 linters, and the per-rule table in
      `docs/lint-triage.md` is the only place a count belongs — neither `AGENTS.md` nor
      `docs/policies/formatting.md` quotes one, on purpose.

### Open

Nothing. Every gate this priority set out to restore is green, required, and reproducible from
a `just` recipe, and the debt each one exposed is either fixed or named and defended in
`docs/lint-triage.md`. New work belongs under the priority it serves.

## Priority 1 — Fix known numeric defects

The twelve defects found by review against normative sources are fixed (`4142444`, `775c7f5`,
`bbc2350`), and so are the four items their review left open: the Eisenbahn substitute speed, the
Fahrbahnart zero value, the `+1` flow guard, and the compensated-summation policy gap. Each was
re-verified against the source text before the change, every affected golden was regenerated, and
the per-item magnitudes are recorded in those commit messages and in the conformance declarations.
What remains here is what those fixes did not settle.

The `+1` flow guard is gone: `schall03/emission.go` computed `10 lg(n + 1)`, the same spurious
+3.0 dB at 1 train/h that `ac33895` removed from `cnossos/road`, `cnossos/rail` and `bub/road`. It
now uses `10 lg(n)` with an explicit zero-flow branch returning the `-999` silence sentinel. This
sits on the data-pack path, which is the path the CLI actually runs (Priority 2), so it moves real
output: the preview goldens dropped 0.4–1.0 dB.

### 1.1 Schall 03: the Nr. 5.3.2 substitute speed — closed

The Eisenbahn half was closed first: `resolveEffectiveSpeed` applied `max(v, 50)` unconditionally,
which Nr. 4.3 does not prescribe. The Straßenbahn half is now closed too — a `TrackSegment` can
declare the Weichen, Kreuzungen und Haltestellen the substitution is scoped to, and the compute path
splits it at their ±25 m zone boundaries.

One live constraint follows from that and governs anything built on top: the split is **opt-in**. A
segment declaring no features keeps the substitution over its whole length, because reading silence
as "no substitution" would lower levels for every existing model without the modeller having said
so. Declaring features is what buys the normative extent. `permanently_slow` and declared features
are mutually exclusive. See `docs/conformance/schall03-konformitaetserklaerung.md` deviation 6.

### 1.2 Schall 03 wire format: ordinals are refused — closed

`FahrbahnartType`, `SFahrbahnartType`, `SurfaceCondType` and `WallSurfaceType` are renumbered so the
reference row carrying no correction is the zero value. The hazard that created — a file written
against the old numbering being silently misread — is closed by refusing ordinals on the wire
entirely: all four are read and written as names from `schall03/vocabulary.go`, and a bare JSON
number is an error naming the accepted values.

The live constraint: **these enums are ordinals of Anlage 2 tables and move when a reference row
moves, so no wire format may carry them.** That is why no schema-version field or migration entry
was needed here, and why a new table-backed enum must join the vocabulary rather than being
serialised as an int.

### 1.3 Compensated summation — decided: implement, and say where

`docs/policies/determinism.md` §3 asked for "a stable strategy (for example pairwise or compensated
summation)" without saying which reductions it meant, and every reduction in `standards/` was a
plain `sum +=`. Resolved by narrowing the policy to a testable rule and then satisfying it:
`internal/numeric.CompensatedSum` (Neumaier) is now used wherever the term count scales with model
size — the Schall 03 subsegment integrators and their reflected-path twins, `EnergeticSumLevels`,
`sourceSegmentLengthM`, RLS-19's `polylineLength` — and wherever terms alternate in sign
(`cnossos/industry`'s shoelace area). Fixed-length reductions (eight octave bands, the vehicle
classes of a train, a correction-table row) are exempt and the policy now says so. Every golden
stayed byte-identical, which is the expected result: the change buys accuracy headroom, not a
different answer.

### 1.4 Fixture blind spots — closed

Seven fixtures were added for the cases the suite could not see. Each was verified to reach the path
it claims: `TestLateralDiffractionCanDominate`, `TestThreeDiffractionEdgesAreSelected` and
`TestReflectiveBarrierReachesDrefl` pin the three barrier geometries, and the b1 geometry was
re-checked to confirm its lateral A_bar is capped at 20 dB in every band, so the assertion is not
vacuous.

| fixture                     | closes                                                    |
| --------------------------- | --------------------------------------------------------- |
| `b3_lateral_diffraction`    | lateral path per-band cheaper than the top path (Gl. 18)  |
| `b4_three_edge_barriers`    | three diffraction edges survive the rubber band (Bild 6)  |
| `b5_reflective_barrier`     | reflective source-side edge at d_s ≤ 5 m — Gl. 20 D_refl  |
| `e4_bruecke_feste_fahrbahn` | bridge combined with Feste Fahrbahn (Nr. 4.6 suppression) |
| `e3_langsame_strecke`       | 40 km/h Eisenbahn line — non-`v₀`, no substitute speed    |
| `s3_langsamfahrstelle`      | Nr. 5.3.2 `permanently_slow` exception, end to end        |
| `iso9613/point_cmet`        | non-zero `c0_met` across the 10(h_s + h_r) threshold      |

The ISO 9613-2 fixture places two sources (5 m and 30 m) over four receivers from 60 m to 1000 m,
so C_met is exactly zero at the near receiver, active for one source only at 150 m, and active for
both further out. `LpAeq_LT` and `LpAeq_DW` now differ in the golden, which they did not in any
previous fixture.

This entry previously asked for an _intermediate_ reflective barrier, which would have produced a
byte-identical golden: `multiEdgeGeometry` reads `first.Barrier` alone, so only the
nearest-to-source surviving diffraction edge can carry D_refl. `b5_reflective_barrier` therefore
puts the reflective wall 2 m from the track (d_s = 3.61 m, inside Gl. 20's 5 m limit) and leaves the
walls behind it absorbing. Two further traps are load-bearing for any future D_refl fixture: D_refl
is read only on the top-diffraction branch, and `ComputePathBarrierAttenuation` takes the per-band
minimum against the lateral Gl. 18 path, whose D_z caps at 20 dB — so a scene screened much harder
than this one hides the correction behind that cap.

### 1.5 RLS-19 — closed

The premise this section used to carry, that the RLS-19 text is unavailable, was wrong: the FGSV
PDFs sit under `interoperability/RLS-19/` (gitignored — local reference material, never checked
in), and the 2019 edition there already incorporates Korrekturblatt 2/2020. Tabellen 2–8 and
Eqs. 7a–7c, 8, 9, 10, 12–15 have been read against the module. Seven defects were found and fixed;
`docs/conformance/rls19-konformitaetserklaerung.md` rows C2–C8 carry them with magnitudes.

Two constraints for the next pass over this or any other scanned standard. `pdftotext -layout` is
not sufficient evidence — it drops terms from stacked fractions and flattens the crossed-out cells
of Tabelle 4a, and two of the five defects were invisible in the extraction while one apparent
discrepancy turned out to be an extraction artefact. Render the page and read it. And extracted
text stays in a scratchpad: RLS-19 is FGSV-published and not clearly amtliches Werk, so cite
section and equation numbers rather than copying table text into the repo, which is what the
`no-third-party-data` gate protects.

Two live constraints follow from the last two fixes and govern anything built on top.

**A table cell that does not exist is not a zero.** `SurfaceCorrection` collapsed three unrelated
causes to `0` — Kräder, an unknown surface, and a cell Tabelle 4a crosses out — and the NaN that
distinguished the third was thrown away on the way out. `lookupSurfaceCorrection` now returns
`(value, ok)` and `RoadSource.Validate` refuses an out-of-band pairing; nine of the seventeen
surfaces can produce that error. The rule generalises: a lookup that can fail must say so to its
caller rather than pick a plausible number, and the refusal must be decided by the **same** lookup
the compute path reads, or the two drift.

**An omitted field is not a default, and a table ordinal is not a wire format.** The Parkplatz path
had both failures at once: `vehicle_type` was a Tabelle 6 ordinal tagged `omitempty`, so an
omission meant Pkw at 0 dB, and an omitted movement rate meant `N = 0`, which is the silence
sentinel. Both Parkplatztypen are now named strings with an explicit unset value that `Validate`
refuses, and the rates are pointers so an explicit `0` stays legal while an omission does not.
This is P1.2's rule reaching a second module: **a new table-backed enum joins a string vocabulary
rather than being serialised as an int**, and a field whose zero value is a legitimate input needs
a representation for "not stated".

Closing the parking half required making the path reachable at all — it was library-only while the
conformance declaration called it implemented — so `aconiq run` now accepts `rls19_parking_*` on an
`area` source feature.

**A mirror source is a source.** Nr. 3.5 says so in one clause — "Bei der Schallquelle kann es sich
auch um eine Spiegelschallquelle handeln" — and Gl. 11 then gives it the whole chain,
`D_div + D_atm + max{D_gr; D_z}`. Reflected paths were taking only the first three terms, so a
barrier across a mirrored path did nothing; three CI-safe fixtures were over-predicted, by up to
3.26 dB. The same clause is why Parkplätze reflect at all: Gl. 3 names `D_RV1,j` and `D_RV2,j` "für
die Parkplatzteilfläche j", and the deviation declaring otherwise was wrong on the text and wrong
about the code, which was point-based all along. The rule generalises: **a new propagation path
takes the entire §3.5 chain, or the reason it does not belongs in the conformance document with the
sentence from the standard that permits it.** Watch for the trap that made this more than a one-line
fix — a mirrored ray crosses its own reflector by construction, and a building is barrier and
reflector at once.

### 1.6 The EPSG:25832 ↔ 4326 round trip loses metres, and the map save path runs it — closed

`geo.TransverseMercator` — the Krüger n-series in Karney's formulation, to sixth order in the third
flattening — is substituted for the library's projection on every transverse-Mercator code the
package supports (25831-25834, 31466-31469, 32632, 32633), leaving its datums and Helmert shifts
untouched. The `25832 → 4326 → 25832` residuals that ran to 8.53 m are now under 4e-9 m, and the
save path this section was named for is unchanged and lossless.

Four constraints are live, and the first two are not about coordinates at all.

**`3/2` in a floating-point expression is `1`.** The whole defect was
`math.Pow(1-e²sin²φ₁, 3/2)` in the library's inverse: an untyped integer constant expression, so
the radius of curvature in the meridian came out 0.21% large. Go will not warn, `go vet` will not
warn, and the same file writes `2.0` and `4.0` correctly a few lines away. Any exponent, divisor or
coefficient written as a bare ratio in numeric code is suspect until someone has checked its type.

**A round trip cannot tell you which direction is wrong, and a clean one tells you nothing.**
`TestEPSGTransform_Roundtrip` started in degrees, so it ran inverse∘forward, and on that ordering
the error largely cancels — it passed at `0.0001°` throughout. Tightening it would not have caught
this: the bug was one-sided, the forward agreeing with PROJ to sub-millimetre, which is exactly why
the round trip hid it. Each direction is now asserted **on its own** against PROJ 9.8.1 reference
vectors (`internal/geo/testdata/`, `crs_reference_vectors_test.go`), and that is the shape any
future transform work takes. The round-trip tests remain, tightened, as a cheap floor — not as
evidence.

**The vectors are the fixture; PROJ is not a dependency.** `testdata/proj-reference-vectors.json`
was generated once by `cs2cs` and checked in with its provenance and its generator
(`testdata/README.md`). Nothing in `just go-ci` calls PROJ, and nothing should: adding it would
trade a reviewable 52-row fixture for a toolchain pin on every developer and CI runner.

**The DHDN Gauss-Krüger codes are still ~1.8 m from PROJ, and that is the datum, not the
projection.** `wgs84` carries one Helmert set for DHDN where PROJ 9.8.1 prefers the BETA2007 grid.
The projection on the same Bessel ellipsoid is pinned to a micrometre, so the two are separable and
the tests separate them. Closing the datum gap means shipping a grid file; nobody has asked.

Two routes are closed rather than untried. Correcting the constant is not enough: rebuilt with
`1.5`, the library's inverse is still 0.0963 m out at the western edge of zone 32, one-way against
PROJ — Snyder truncation, and this time almost entirely in the **longitude**, which `R1` never
touched. And "move to a maintained binding" is not an open option: a cgo PROJ binding is ruled out
by `backend/cmd/wasm`, which must compile to `js/wasm`, while `wroge/wgs84` v2 has replaced this
series with Krüger but is alpha-only (`v2.0.0-alpha.20`), with a changed API and at 4th order
rather than 6th. Reported upstream as https://github.com/wroge/wgs84/issues/30, cited from
`transversemercator.go` so the next reader knows why the projection is ours.

### 1.7 A geographic project CRS made every level wrong, and it was the default — closed

`aconiq run` projects a geographic project CRS into an ETRS89 / UTM zone taken from the model's own
centre before anything reads a coordinate, and records `project_crs` and `compute_crs`.

**The bullet this closes called it the auto receiver grid's defect. It was not**: every module
measures distance with `geo.Distance`, so the units error was the whole compute path — it
reproduced with custom receivers and reached `rls19-road`, on a model that validated without a
warning. `aconiq init` defaults `--crs` to EPSG:4326, so this was the out-of-the-box path.

Five constraints are live:

- **Only zones 31-34 exist**, and a site outside them is refused rather than projected into the
  nearest zone that does. ETRS89 is the target for EPSG:4326 too: the datum difference is a
  near-rigid sub-metre shift that changes no distance, where a WGS84 / UTM table would reach only
  zones 32 and 33.
- **Results are in the compute CRS**, not the project CRS. This bullet used to rest on 1.6's lossy
  round trip; that argument is gone, and the constraint stands on the stronger one: transforming
  results back would move coordinates nothing computed from, and label them with a CRS they were
  not computed in. Which is also why an export bundle labels the model GeoJSON and the results
  separately, and why contour GeoJSON is reprojected to WGS84 rather than labelled (RFC 7946 fixes
  its CRS).
- **A projected project is not touched**, evidenced by the 13 digest goldens moving by exactly the
  two new provenance keys and nothing else.
- **Coordinates embedded in properties move with the geometry.** RLS-19 directional sources carry
  their own centerline and Schall 03 track features their own point, and both are consumed as
  geometry. `modelgeojson.propertyGeometries` is the list; a new coordinate-bearing property has to
  be added to it or it silently stays in degrees.
- **Terrain is wrapped, not resampled.** The DTM stays in the project CRS and compute-CRS queries
  are transformed back per lookup, because `terrainElevationAt` turns an out-of-grid miss into an
  elevation of 0 without saying so.

`aconiq compare-raster` deliberately does **not** reproject: it compares against SoundPLAN rasters
in the project's own CRS.

### 1.8 RLS-19 measured the propagation path above sea level — closed

`h_m` in Gl. 14 is the mean height of the path **above the ground**. The module computed the mean
path elevation correctly and then subtracted an average terrain elevation that `computeTerrainAvgZ`
answered with **0** whenever no declared slope edge crossed the path — and the CLI never populates
`cfg.Terrain`, so that was every path of every run. On a project carrying elevations `h_m` became the
height above sea level, `D_gr` went far negative, Gl. 14's `minimum 0` clamped it, and the
Bodendämpfung disappeared without a word: up to +4.8 dB, measured at +4.39 dB end to end on a site
400 m up. Recorded as C11 in the conformance declaration.

Four constraints follow, and the first is the general lesson.

**Zero is an elevation, not an absence.** A lookup that cannot answer must say so rather than return
a plausible number — the same rule P1.5 drew from RLS-19's `SurfaceCorrection`, reached here by a
different route. `computeTerrainAvgZ` now returns `(value, ok)`.

**Where nothing is known, the ground under each end is the ground at that end.** That fallback —
not DTM sampling — is the core of the fix, and it is bit-identical to the old output when elevations
are 0. That identity is why 843 tests and 94 acceptance fixtures were blind to this: every fixture
but two sits at elevation 0. Only `i8_ascending` and `i9_receding` moved, by −0.5732 dB.

**A DTM must be read for its shape, not its datum.** The ordinary project has a DTM and 2D GeoJSON
sources, so the model's declared ground is 0 while the DTM says 400. Sampling absolute elevations
would have produced `h_m ≈ −197` and ~41 dB of invented attenuation — a worse defect than the one
being fixed. `geo/terrain.MeanRiseAboveChord` therefore returns the mean rise of the terrain above
the chord joining its own endpoint samples, and the module adds that to the chord between the ground
elevations it already knows.

**Schall 03 has the same bug, and this paragraph said the opposite.** The claim rested on reading
`schall03.meanPathHeight(hg, hr)` as operating on heights above ground without tracing where `hg`
comes from: it is `elevation_m + heightAboveSO`, and `elevation_m` is an absolute Z — that is what
`aconiq import --from-soundplan` writes — while `receiver.HeightM` is a height above ground. The
correction is the useful part, because the module is worse off than RLS-19 was. Four terms read those
heights, not one, and the two obvious ones cancel: `A_gr,B` loses its ground attenuation (too loud)
while `d` inflates `A_div` (too quiet), netting −2.73 dB in free field. The one that does not cancel
is `ComputePathBarrierAttenuation` — a source at an apparent 404 m clears every wall, so **shielding
disappears and a receiver behind a Schallschutzwand reads +7.0 dB high**. Reproduced and fixed
against `main` in its own pull request, deliberately not stacked on this branch.
**A near-cancelling total is how this stayed invisible.** Anyone measuring only the receiver level
would have read −2.7 dB and moved on; the components have to be measured separately.
Deviation 4 is a different, weaker thing and remains open: the flat-ground special case of `S/d`.
`MeanRiseAboveChord` is what closes it — add the sampled rise to `(h_g + h_r)/2` — and it is
deliberately left undone, because it lives on this branch and the datum fix was based on `main`.

Cost: a DTM-attached run is ~28 % slower on the propagation path (25 m nominal sampling, capped at
64 intervals). Only projects that import a DTM pay it; a geographic project pays more, because every
sample goes through `terrainInComputeCRS`.

## Priority 2 — Make the CLI run the normative code

**Closed.** `aconiq run --standard schall03` reaches `ComputeNormativeReceiverLevelsWithScene`.
The gap this priority described was not a code-quality issue but a mismatch between what the
conformance declaration claimed and what the binary did: `run_pipeline.go` called
`ComputeReceiverOutputs`, whose `BuiltinDataPack()` supplies invented spectra
(`{73,76,80,84,87,85,81,76}`, a scalar `GroundAttenuationDB: 1.2`), while the real Anlage-2 chain
was reached from exactly one caller outside the package — the acceptance runner.

The two chains are now named and separated by a `schall03_engine` run parameter. `auto` (the
default) resolves to the normative chain when the model carries `schall03_operations` and **fails
otherwise** rather than degrading silently; `preview` is an explicit opt-in that logs a warning and
stamps `baseline-preview-no-normative-tables`. Normative inputs come from a `schall03_*` GeoJSON
vocabulary (Zugart or Fz composition, Fahrbahnart, Streckenhöchstgeschwindigkeit, Tabelle 8 surface
measures, bridge type, station and Langsamfahr flags, water fraction), documented in
`docs/geojson-schema-v1.md`. The vocabulary is strings, deliberately: the enum ordinals are table
row positions and P1.2 already renumbered them once.

Three consequences fell out of the work:

- The provenance contradiction is gone, and not by picking one side. `model_version` and
  `compliance_boundary` are no longer stamped at `CreateRun` time, because the engine is not known
  until the model is read; `projectfs.Store.MergeRunProvenanceMetadata` completes the manifest once
  it is. `phase20-normative-eisenbahn-strecke-v1` — which was being stamped on preview runs — is
  replaced by `anlage2-2014-strecke-v1` and `baseline-preview-datapack-v1`.
- The data-pack question is answered: the out-of-repo pack is **not** the distribution mechanism
  for the normative tables and never was. Anlage 2 is an amtliches Werk under §5 UrhG and its
  coefficients are already embedded (`beiblatt1.go`, `beiblatt2.go`, `beiblatt3.go`, the Tabellen).
  `LoadDataPack` stays only so an operator can substitute measured data behind the preview boundary.
- The Schienenbonus statements in `docs/conformance/schall03-konformitaetserklaerung.md` are
  reconciled with the code. "+5 dB retained für Straßenbahnen" and "−5 dB nur für den
  Streckenanteil" were both wrong; K_S = 0 dB on both sides since 2015 / 2019. Gl. 35-36 keeps the
  term on the Strecke side only, at value zero.

**The receiver table's row order is the model's order, in both targets, and `output_hash` is
computed over it.** Changing either side's ordering is therefore a hash change. Browser runs already
in IndexedDB keep their old rows and their old hash, so re-running the same model does not reproduce
a stored run; `PERSISTED_STATE_VERSION` stayed at 1 deliberately, because the stored documents still
parse and discarding a user's model and runs over a row order is the larger harm.

### Open

The geographic refusal is no longer written out twice. `crstransform.GeographicRefusal` is the
one place that sentence exists; `resolveTarget` and `app/cli/run_crs.go` both call it, and
`cli.TestGeographicRefusalReadsTheSameOnAllThreeSurfaces` compares CLI, API and kernel byte for
byte at both refusal sites. Four things it leaves live:

- **The CLI names the CRS as `geo.ParseCRS` canonicalises it**, not as the caller typed it. That
  is what makes a byte-for-byte comparison possible at all, and the parity test pins it — reverting
  to the raw string fails with `cli: "project CRS epsg:4326 …"` against `api: "… EPSG:4326 …"`.
- **A byte-for-byte comparison cannot be made on the CLI's `error.Error()`.**
  `domainerrors.AppError.Error()` renders `"<Op>: <Msg>: <Err>"`, so the rendered strings can never
  be equal. The test compares `AppError.Msg`, and asserts the kind and op separately.
- **`aconiq run` renders the cause twice** — the shared sentence already ends with
  `": " + err.Error()`, and `AppError.Error()` appends the same cause again. Pre-existing, left
  unchanged because fixing it changes user-visible CLI output and is owed its own commit.
- The endpoint's own two constraints stand. **Its message is unprefixed**, alone among this
  package's handlers, and only a test keeps it that way. And **it reads no project**, which is what
  keeps it on the right side of the invariant that governs the map: it is a projection _of_ the
  model, never a source _for_ it.

- [ ] **Property geometry is unreachable in browser mode.** `rls19_directional_sources` and
      `schall03_track_features` carry coordinates in the project CRS inside a feature's properties
      (`geo/modelgeojson/reproject.go`'s `propertyGeometries`), and the batched `aconiq.transform`
      contract takes a flat coordinate array, so browser mode cannot move them. `compute-crs.ts`
      refuses such a model rather than projecting around it — leaving them in degrees inside a
      metric model puts a directional source millions of metres from its own receivers. Reaching
      them needs either a nested-batch transform contract or a second collection pass that walks
      the property trees; neither is worth building until a browser-mode model can express one.
- [ ] **Browser terrain would be queried in the compute CRS.** `terrainAtGridCenter`
      (`cmd/wasm/main.go`) queries the DTM at the receiver centroid, which is now a UTM metre pair,
      while the GeoTIFF stays in whatever CRS it was written in. The CLI answers this with
      `newTerrainInComputeCRS`; the kernel has no equivalent. It is latent — browser mode never
      calls `loadTerrain` — and becomes live the moment it does.
- [ ] **`aconiq compare` still runs the preview chain, by explicit opt-in.** The SoundPLAN import
      produces the `rail_*` preview vocabulary only, so `compare_test.go` and `cmdoutput_test.go`
      now pass `--param schall03_engine=preview` rather than reaching it by accident. The ~25 dB
      delta in Priority 3 is therefore still a preview-chain number and P2 no longer explains it.
      Mapping SoundPLAN rail geometry onto `TrackSegment`/`TrainOperation` is Priority 13, and it
      is now the blocker for the only real validation evidence this project has.
- [ ] **Rangier- und Umschlagbahnhöfe are library-only.** `rangierbahnhof.go` and the whole
      Beiblatt 3 source catalogue have no GeoJSON representation and no run-pipeline branch, so
      Gl. 35-36 combined assessment is unreachable from a run. The conformance declaration now says
      so under "Reachability from the CLI"; it needs a `schall03_yard_*` vocabulary to stop being
      true.
- [ ] **Obstacles are not mirrored into a reflection's unfolded frame.** Found while fixing the
      order-≥2 origin, which is closed: `ReflectionPath.EffectiveSource` is now the last bounce's
      image source, so the diffraction check and `TotalDist` describe one ray, and a barrier across
      an order-2 path moved a measured 0.5549 dB where it had been worth **exactly 0.0000 dB**.
      What is still approximate is the other half. `ComputePathBarrierAttenuation` receives the
      unfolded source but the panels at their **real** coordinates, so only the final leg — last
      reflection point to receiver — is exact; every earlier leg is tested against unmirrored
      obstacles, at every order including the first. Declared as deviation 3 in the conformance
      declaration. Closing it means mirroring the obstacle set per leg, which is a larger change
      than the origin fix was, and the sign of the error has not been established.
- [ ] **Grid receivers land inside building footprints.** `run_receivers.go` does no building
      masking, so in auto-grid mode a receiver inside a footprint is now shielded by the one ring
      edge its ray crosses instead of standing free. RLS-19 behaves the same way, so this is
      consistent rather than novel, but it is a visible output change on urban grid runs and the
      value at such a point is not an Immissionsort.
- [ ] **The SoundPLAN import produces no DTM, so the Schall 03 ground-datum fix does not reach it.**
      `import_soundplan.go` records only the _name_ of the file the elevation data came from
      (`GeoTmp.geo`, `Höhen.txt`, `.dgm`) in its import report, and registers no `artifact-terrain` —
      which is the only thing `run_pipeline.go` looks for. A SoundPLAN project therefore carries an
      absolute `elevation_m` (the rail's `ZTrack`) against a ground plane at Z = 0: the datum mix,
      unfixed, for exactly the projects that motivated fixing it. A normative run in that state now
      warns in `run.log`, and the gap is declared in `CHANGELOG.md` and in entry 10 of the
      Konformitätserklärung — **but the numbers are still wrong**. Closing it means converting
      SoundPLAN contour lines, elevation points and `.dgm` files into a terrain artifact the run
      pipeline can load. Found by a review bot, not by the tests, and the claim it falsified was this
      project's own: the fix was announced as reaching "every SoundPLAN-imported project".
- [ ] **Schall 03 lateral diffraction is single-edge in the ground plane.** Wall panels sharing an
      `ObstacleID` are one obstacle, and a lateral path may round only a free end of an open
      obstacle or the outermost silhouette vertex of a closed one. **An empty `ObstacleID` is its
      own obstacle with both endpoints free** — that compatibility default is what keeps every
      pre-existing scene byte-identical, and anything that emits `BarrierSegment`s must keep it.
      The silhouette rule is an approximation with a known sign: the taut string may touch two
      corners per side, so the modelled detour is shorter, `z` and `A_bar` come out smaller and the
      level louder — on the safe side. Multi-edge lateral diffraction (`e` in the ground plane,
      `C₃`, `DzCapDouble`) would only make results quieter; it is deviation 8 in the conformance
      declaration, and `diffractedEdgeRunLength` / `pathDifferenceNonParallel` already exist.
- [ ] **Terrain, explicit reflectors and per-direction sources are still CLI-only.** Browser mode
      builds road sources, barriers, buildings and Parkplätze, and `wasm/types.ts` now mirrors the
      terrain and reflector types because the kernel accepts them — but no model can express them
      in the browser, so the parity fixtures cannot cover them.
- [ ] **No terrain on the Schall 03 propagation path.** `elevation_m` is per segment and h_m falls
      back to the flat-ground special case (deviation 4 in the conformance declaration), even when
      the project carries a DTM the RLS-19 path already reads.

## Priority 3 — Establish real validation evidence

**The comparison now asserts, it matches real pairs, and it fails.** `compare_test.go` ran the
whole `init → import --from-soundplan → compare` pipeline against the reference project and never
read a single delta field. It does now (`ac33895`), and the pairs it reads are now the right ones
(#37). Against the real fixture Aconiq reads systematically high on
**every one** of the 30 matched receivers:

| indicator | mean abs | p95 abs | max abs | exceeding |
| --------- | -------- | ------- | ------- | --------- |
| LrDay     | 28.131   | 37.084  | 37.193  | 30 / 30   |
| LrNight   | 26.808   | 35.360  | 35.507  | 30 / 30   |

**Do not compare these to the 24.479 / 23.068 that stood here before.** Those were means over 60
pairs the matcher invented by list position, on an Aconiq side that was the map label layer rather
than the immission points. The mean going up by 3.7 dB is not a regression; it is the first number
in this table that measures anything. The maximum came down, from 39.745 to 37.193.

This is the single most important number in this file. Note the shape of it: ~27 dB high, on a
Schall 03 rail project, measured against `BuiltinDataPack()`'s invented spectra. Priority 2 has
since wired the CLI to the normative Anlage-2 chain, but **the comparison still does not reach
it**: the SoundPLAN import produces only the preview `rail_*` vocabulary, so `compare` now opts
into the preview engine explicitly. **The delta above is a preview-chain number and nothing should
be concluded from it. Priority 13's `TrackSegment` mapping is what makes it measurable.**

**The reference project is a Schall 03 _1990_ model, not an Anlage-2-2014 one**, which bounds what
this comparison can ever claim. `Project.sp` selects `[RAIL] SELECTED=20490` and its own description
says _„Häkchen 5 dB Schienenbonus setzen"_; `TS03.abs` holds the 1990 train library (`ICE (v<=250)`,
`D / FD-Zug (2000)`, `Eilzug (2000)`, `Güterzug (Fernv.)` …), whose `ZugArt` ordinals are exactly the
ones `railops.go:231-278` hardcodes. `soundplanimport.go:165` parses `RAILBONUS` and nothing consumes
it. So even after Priority 13's `TrackSegment` mapping lands perfectly, the delta measures the
1990 → 2014 method change **plus** the Aconiq implementation, with the 5 dB Schienenbonus inside it:
a residual of a few dB against this fixture is not evidence of a defect, and agreement to 0.5 dB
would be evidence of something wrong. Two further consequences for Priority 13: a 1990 `TS03` row
carries a single A-weighted base level and **no Fz decomposition**, so `FzComposition` has to come
from a declared editorial lookup table rather than from the data; and `Fahrbahn`, `BridgeType` and
`Surface` arrive only as dB surcharges (`DFb` +2 dB Betonschwellen, `DBue` +3 dB), which have no
categorical 2014 counterpart. A 2014 reference project is what would make this comparison a
conformance measurement.

- [ ] Tighten the thresholds. What is in the test today is a deliberately loose regression
      bound (mean*abs ≤ 34 dB, max_abs ≤ 45 dB) labelled in the source as \_not* a tolerance, sitting
      alongside exact self-consistency assertions that do hold the compare command to a real
      standard. Once Priority 2 and Priority 13 are done, this must come down by a large factor.
- [ ] Get reference data into CI — submodule or Git LFS, licence permitting — so the comparison
      runs. Today `interoperability/` is gitignored and the SoundPLAN tests skip in CI. The
      plumbing for it is in place: `internal/qa/fixtures.SoundPLANProjectDir` is the single place
      that resolves the fixture, honours `ACONIQ_SOUNDPLAN_FIXTURES` so CI can mount the data
      anywhere, and discovers it locally by looking for the one directory under
      `interoperability/` containing a `Project.sp` — which is what removed the customer project
      name from tracked source. What is left is the licence question and the CI wiring itself.

- [ ] Replace the `(to be filled from conformance report)` placeholders in
      `docs/conformance/rls19-konformitaetserklaerung.md` — all 20 TEST-20 task rows and every
      Max-delta column are blank, under a document titled _Konformitätserklärung_. The document now
      states plainly that they stay empty until the official task set is run, which is honest but
      not evidence.
- [ ] Set tolerances that mean something: ~0.5 dB against a reference tool, not 1e-6 dB against
      yourself. Keep the 1e-6 dB comparison, but rename it what it is — a determinism check.

### Landed

- [x] **Receiver matching is exact, and `ordinal` is gone** (#37). The defect was in the import,
      not the matcher: two `GeoObjs.geo` object types were swapped, so map text captions became
      receivers and the Immissionsorte were discarded. Four things it leaves live:
  - A SoundPLAN Immissionsort is **a column of receivers, one per floor**, keyed by
    `(soundplan_obj_id, soundplan_floor)` — what `RREC*.abs` is itself indexed by. A duplicate key
    on either side is a hard `KindValidation` error; floor 0 marks a point that could not be
    expanded, and never matches.
  - `aconiq compare` reads **one** result run, chosen by the geometry the _model_ carries against
    what each `.res` records, and overridable with `--soundplan-run`. `RSPS0011` and `RSPS0021` are
    the same receivers without and with the noise barrier, differing by up to 8 dB.
  - The matching tests must stay runnable **without** the licensed fixture, in
    `compare_match_test.go`. The `t.Skip` that fires in CI is how a matcher that never matched
    anything shipped.
  - Synthetic raster receivers take their height from the bundle's `RLKHEIGHT`, not from the
    model's first receiver, which made every raster number a hostage to the receiver import.

- [x] **The raster path selects one grid map too** (#40). Four things it leaves live:
  - **The barrier signal alone does not select one raster run.** It selects two. The reference
    project holds the same site four times — without and with `GeoWand.geo`, each at a 4 m and a
    2 m grid — so the second discriminator is the grid height in the `GNM<spacing>:<height>` token
    of the run's `RunCommands`, matched against `RLKHEIGHT`. Any future "just reuse the receiver
    selector" reasoning has to account for that.
  - The evidence travels on `GridMapMetadata` (`geometry_files`, `run_layout`), written by
    `LoadGridMapMetadata` from the `*RunResult` it already holds, rather than being re-read from a
    `.res` whose name is guessed from a result subfolder. **An import report written before those
    fields existed carries no evidence**, so a project has to be re-imported for the choice to be
    made on anything but name order; without it the run is picked by name and warned about.
  - `soundplan_runs` and `soundplan_raster_run_count` stay the number of grid maps **discovered**.
    Exactly one is compared, and which one is reported beside them — shrinking the count to 1 would
    have hidden the other three rather than explaining them.
  - **Absent geometry evidence and contradicted geometry evidence are different branches.** A model
    that provably describes none of the imported grid maps — an edited `--model` that adds a barrier
    the bundle never computed with — must not fall through to a height match and report
    `grid_height_match`; it selects, warns and reports `geometry_contradicted`. And the synthetic
    receivers take the **selected run's** `run_layout.height_m`, not the project's `RLKHEIGHT`,
    because `--soundplan-grid-run` and the geometry signal can both land on a run computed at
    another height.

## Priority 4 — Honest standards labelling

**Closed.** The registry no longer offers 13 standards as peers. `framework.EvidenceTier`
(`normative` | `preview` | `scaffold` | `test-fixture`) is a **required** field on
`StandardDescriptor`, enforced by `Validate()` — which `NewRegistry` calls — so a module cannot be
registered without a deliberate tier decision, and `standards_test.go` pins the assignment and the
registry's exact membership in both directions.

The tier is derived once, from the resolved descriptor, and carried into every artifact a third
party reads: `provenance.json` (stamped centrally in `buildRunProvenanceMetadata`, so the ten
per-module `ProvenanceMetadata` functions were not touched and the free-text `compliance_boundary`
they return now has a machine-readable companion rather than a competitor), `run-summary.json`, the
`aconiq run` banner and `run.log`, the `--json` run payload, `aconiq status`, the Markdown/HTML/Typst
reports, the export bundle summary, and `GET /api/v1/standards`. Reports and the export summary read
the **as-run** tier from provenance rather than re-resolving it, so a later re-tiering cannot rewrite
what an archived bundle says; `aconiq status` deliberately reports the _current_ tier and says so.

Scaffold-tier standards — eight of the thirteen — are gated behind `--experimental`, refused before
`CreateRun` so a refused run persists nothing. The API gates in the handler rather than parsing the
subprocess's exit code, so the refusal carries a real error envelope; both sites call the same
`RequiresExperimentalOptIn()`, so they cannot disagree.

Three things fell out of the work:

- **`bub-rail` and `bub-industry` are wired into the run pipeline.** They were registered and
  unreachable — `run_pipeline.go`'s `default:` branch. Wiring them cost less than deregistering them
  would have cost in trust, and it removed duplication rather than adding it: the shared
  rail/industry options and persist paths were factored, three now-unused `//nolint:dupl`
  suppressions went with them, and `run_persist.go` lost 63 lines net while gaining a parameter and
  a key. `TestEveryRegisteredStandardCompletesARun` drives every registered ID end to end, so the
  `default:` branch cannot silently acquire a new occupant.
- **The aircraft modules keep their IDs.** Renaming `cnossos-aircraft` / `buf-aircraft` would break
  every manifest that names them, and the package name is not the only place the disclosure can
  live. CNOSSOS-EU defines no aircraft method at all — Directive 2015/996 Annex II is road, rail and
  industry; aircraft is ECAC Doc 29 — and that now appears in the descriptor strings, in the scope
  declaration, and behind the `--experimental` gate. Revisit only if a real ECAC Doc 29 module is
  ever built, at which point the rename has a destination.
- **`docs/conformance/` is where the disclosures live now.** The honest limitations were buried in
  `docs/phase1[0-6]-*-baseline.md`, named after internal sprint numbers. They are lifted into two
  German scope statements — explicitly _not_ Konformitätserklärungen — with the phase files left as
  historical baselines carrying a pointer.

### Open

- [ ] **The kernel's standards list is hand-maintained, and must stay honest.**
      `internal/wasmkernel.Standards()` is the single declaration of what the WASM build can run —
      descriptor and entry-point name together — and `aconiq.standards()` publishes it through the
      same `descriptorjson` encoding `GET /api/v1/standards` uses. It deliberately does **not**
      import `internal/standards`: `NewRegistry` links all thirteen modules, and the kernel would
      then advertise twelve it has no entry point for. So adding a standard to the kernel is two
      edits, and `TestEveryStandardHasItsEntryPoint` reads `cmd/wasm/main.go` to catch the one that
      is forgotten. A generated registration would remove the coupling; nothing needs it yet.
- [ ] **Browser mode's model extraction is still RLS-19-only, and now says so.** The refusal by
      name is gone: `startRun` asks `kernel.standards()`, so it cannot disagree with the list the
      same kernel serves the run page. What remains is a second, differently worded gate — the
      kernel publishes the standard but this build has no extraction for it — which is the
      TypeScript twin of `TestEveryStandardHasItsEntryPoint` and is what a second kernel entry
      point will meet. Closing it is Phase F's "move RLS-19 extraction into the kernel", after which
      the dispatch disappears rather than growing an arm.
      Live constraint: **`RunSpec.standardId` is `string`**, so no compiler-checked exhaustiveness
      is available and the `default` arm is what carries the refusal. Giving it a union would
      re-declare which standards exist, which is the duplication this removed.
- [ ] **`Headline()` is English-only.** The CLI banner and the report row are English, but the
      reports are the artifact a German authority reads, and the assessment modules already emit
      German. Decide whether the report row should be localised, and against which message source.
      (The third item — folding the tier into the standard-data digest — is closed: the evidence tier is
      an input to the `standard_data` hash, so re-tiering a module moves the digest. See Priority 5.)

---

# Gate 2 — Trustworthy product

## Priority 5 — Provenance integrity and release engineering

**Closed, except the tag itself.** Build identity landed earlier (`0a3b11a`); this pass closed the
rest. `provenance.json` carries a `standard_data` digest — SHA-256 per named coefficient table plus
an overall digest, with the evidence tier as an input to the hash, so re-tiering a module moves it
and two runs whose digests agree used byte-identical coefficient data. It is a dedicated field, not
an `input_hashes` entry, for the reason this file already gave: that map is input-file path →
SHA-256 and every entry renders as an "Input files" row. The encoding is reflection-based rather
than JSON because RLS-19's Tabelle 4 stores `NaN` for "nicht anwendbar", which `encoding/json`
refuses outright, and because a table holding unexported fields would otherwise encode as `{}`.
Digests for all three normative modules are pinned by tests.

The `phaseNN` model versions are gone. `phase18-baseline-preview` → `2014-anlage2` is a user-visible
`--version` value; the rest were `phaseNN-preview-vN` → `baseline-preview-<subject>-vN`, appearing
only in artifacts. No golden moved — no snapshot had ever captured a model version.

Release engineering exists: `CHANGELOG.md`, `docs/policies/releases.md`, `.goreleaser.yaml`,
`.github/workflows/release.yml` and issue templates. The versioning rule is the decision worth
knowing about, and it is not stock SemVer: **a change to a `normative`-tier module's computed levels
is a breaking change**, regardless of direction or size, and including changes that are unambiguously
corrections — an archived run is evidence someone may hold under a permit application. Scaffold- and
test-fixture-tier numbers get the opposite rule and carry no stability promise at all. That is
enforceable rather than aspirational because the release workflow runs `just update-golden` and
fails on a dirty tree.

**Until the first tag, norm-closeness outranks golden stability.** The rule above protects an
archived run someone may hold under a permit application, and `git tag` is still empty, so there is
no such run yet — `CHANGELOG.md` already says as much. A numeric change that brings a normative
module closer to its standard is therefore taken now rather than deferred, and every one still
carries a **numeric** changelog entry and a conformance-document update in the same change. That is
what makes the shift deliberate rather than silent, and it is what the rule starts enforcing at the
first tag.

### Open

- [ ] **Cut the first release.** Everything needed is in place and `git tag` is still empty, which
      is deliberate — tagging is a maintainer decision, not an agent's. Until a tag exists,
      `SECURITY.md`'s "supported version" is `main` and every artifact's `tool_version` is a
      `git describe` string rather than something a reader can resolve to a download.
- [ ] **Enable Issues on the repository.** The templates are committed; the setting is a GitHub
      admin action nobody in the repository can perform.

## Priority 6 — Security hardening

**Closed.** The threat model was untrusted third-party files plus a local API a browser talks to.
The file half closed in `a8f3bc2`; both remaining halves closed in this pass.

**The API.** Three controls now sit in front of the router: a `Host` allowlist (loopback plus the
host part of `--listen`), which is what closes DNS rebinding — the attacker's own name still travels
in the `Host` header, which is exactly what an Origin check cannot see; a media-type requirement on
every body-reading endpoint, which closes the CORS simple-request path that let a cross-origin
`text/plain` `fetch` reach `exec.CommandContext` with attacker-chosen argv; and a required
`X-Aconiq-Client` header on every state-changing method, whose value is never checked because its
presence is the proof a preflight happened. A bearer token (`--api-token`, `ACONIQ_API_TOKEN`) is
opt-in on top: the three controls close the browser vectors, and a mandatory token answers a
different threat model. Request bodies are bounded per endpoint; both file-serving handlers go
through `filepath.IsLocal` + `os.OpenInRoot`; `overpass_endpoint` is an https allowlist.

Two premises in the old text were wrong and are worth recording. `MaxBytesReader` was **not** absent
— the terrain upload had one; the real defect was narrower and worse, that `ParseMultipartForm` was
handed the 50 MB _body_ cap as its _maxMemory_ argument, so a 50 MB upload buffered whole in RAM.
And no gosec finding was removed: G120 had to stay suppressed because gosec does not model
`MaxBytesReader` and reports the call whatever its argument.

**G304: verdict re-litigated, and it stands.** The case for reopening was that the count had doubled
from 47 to 97. Both prior measurements undercounted — golangci-lint caps at three findings per rule
unless `max-same-issues: 0` is set. The true figure is 108, of which **47 are non-test, exactly the
number when the rule was first waved through**. All growth is in `_test.go` files, covered by a
separate exclusion that was never part of the question. That is a growing test suite, not creeping
exposure, and the opposite of G301. Recorded with the evidence and with what would overturn it in
`docs/lint-triage.md`. **G702** at the run executor stands unchanged and is removed outright by
Priority 7's "move the run pipeline out of `app/cli`".

**FlatGeobuf.** The `Verify`-versus-`recover()` question answered itself: flatbuffers v23.5.26 ships
no verifier — `grep "Verif"` over its `go/` package returns nothing — and the generated flatgeobuf
tables carry no per-table helpers either. So a per-element `recover()`, scoped to one header-field
read and one feature decode, producing typed errors, and re-panicking untouched when the raise site
is Aconiq's own code. `FuzzReadFGB` then found three crashers, **none of them a vtable panic** — all
three were unbounded allocations, including a 65-byte input requesting 1 TiB. The premise that the
target would only find unfixable library panics was simply false.

### Open

- [ ] **Report the three `gogama/flatgeobuf` v1.0.0 bugs upstream.** `DataRem`,
      `FileReader.readFeature` and `PropReader.ReadBinary` each size a `make` from an unvalidated
      file field. All three are worked around on our side; they are still library bugs, and the
      workaround is code this repo would rather not own.
- [ ] **A token and `EventSource` do not compose.** A browser cannot set headers on an SSE
      connection, so `/api/v1/events` is unreachable when `--api-token` is set; the same applies to
      the bare URL handed to the DOM by `getArtifactContentURL`. Nothing uses either today, so
      nothing is broken — but a query-parameter or cookie escape hatch is needed before they are.
- [ ] **`--listen 0.0.0.0:8080` now serves loopback only.** A wildcard bind names no host to add to
      the allowlist. This is deliberate and documented in `serve --help`, but if binding a real
      interface is wanted, it needs an explicit `--allowed-hosts` flag.

## Priority 7 — Architecture: make standards actually pluggable

`AGENTS.md` claims standards are plug-in modules. The dispatch is now one: `runModuleTable` in
`app/cli` maps a standard id onto a `runModule`, nine of the thirteen built from a single generic
`receiverRunModule[Opt, Src, Out]`, and `TestRunModuleTableMatchesTheRegistry` pins the table
against the registry in both directions. The 562-line `switch` and its five-linter suppression are
gone, and `run_pipeline.go` is 238 lines rather than 887.

What is not done is the half that makes the claim true. `framework.StandardDescriptor` still
carries only metadata: the contract lives in `app/cli` because the halves it binds do — option
parsing, extraction and persistence are all `app/cli` files, and `framework` cannot import them
without a cycle. `internal/app/cli` is 13 014 non-test LOC, and adding a standard still means
editing several of its files rather than one package of its own.

- [ ] **Finish the shared acoustics core.** `internal/acoustics` owns the END indicator model and,
      since this pass, `EnergySum`: the seven byte-identical `energySumDB` copies
      (`rls19/road`, `cnossos/road|rail|industry|aircraft`, `bub/road`, `buf/aircraft`) are one
      function with one pair of sentinels, `SilenceDB` and `SilenceThresholdDB`. Every golden is
      byte-identical, so the deduplication is provably not a behaviour change.
      **Two claims this entry used to make were wrong.** `cnossos/road` _does_ skip
      `level <= -900`; it was behaviourally identical to `rls19/road`, and the only variation across
      the seven was whether the numbers were named constants or inline literals. And the two
      sentinels do **not** both reach `results.ReceiverTable`: `finiteOrSilence` converts `-Inf` to
      `-999` at schall03's own output boundary, so only `-999` ever lands in a receiver table.
      What is genuinely left is smaller and sharper than "nine copies":
  - [ ] **Compensate `EnergySum`, and re-cut five goldens deliberately.** The switch to
        `numeric.CompensatedSum` is one line, and it is **not** free — which is the finding that
        matters here, because Priority 1.3 observed compensation leaving every golden unchanged and
        **that does not generalise to these call sites.** These reductions really do scale with
        model size (`rls19/road/propagation.go` sums one contribution per source, per reflection
        path and per line-source subsegment), so compensation recovers real low bits: measured at
        **1-2 ulps, max 2.8e-14 dB**, moving `rls19-road`, `cnossos-road`, `cnossos-rail`,
        `bub-road` and `bub-rail`. Receiver tables persist `float64` at full round-trip precision
        and the run digests hash every byte, so a negligible delta is still a golden diff.
        Until it is taken, `EnergySum` accumulates with a plain `+=` — byte for byte what the seven
        copies did — and says so in its doc comment, with
        `TestEnergySumLosesTermsAnUncompensatedSumMustLose` pinning the terms the naive sum drops so
        the follow-up has a test to invert.
        **Note what this costs under Priority 5's versioning rule**: `rls19-road` is normative-tier,
        and a change to a normative module's computed levels is a breaking change there regardless
        of direction or size. This is a release decision, not a cleanup.
  - [ ] **Decide the two outliers, or declare them.** `bimschv16.energySumDB` has no NaN/Inf guard
        and no silence threshold; `schall03.EnergeticSumLevels` works in `-Inf` internally and
        returns NaN on a `+Inf` term rather than skipping it. Both now carry a comment saying why
        they differ. Converging either changes numbers, so neither belonged in a refactor. "One
        sentinel" is a two-module question now, not a nine-module one.
  - [ ] **One `Level` type** is still untouched.
- [x] **The four Schall 03 normative propagation kernels are one.** `subsegmentContrib`
      (`compute.go`) is the single implementation of Gl. 6, 8–16, parameterised by `dRho`
      (0 direct, the Gl. 28 wall absorption loss reflected), a `rayOrigin` (the real subsegment
      point direct, the fully unfolded image source reflected, read only when barriers exist) and
      a possibly-empty barrier set. The four entry points keep their names and signatures, so no
      caller and no test moved, and the Gl. explanations — which had survived in one copy each —
      now sit on the kernel. The four line-source integrators were the same duplication a second
      time and collapsed with it, into `eachSubsegment` + `directRayTerms`/`reflectedRayTerms` +
      `lineSourceLevel`. Net −126 LOC. Two things it leaves live.
      **The line numbers this entry used to carry were stale** and cost the next reader a search:
      `compute.go:61` was inside `buildVehicleInputs` and two of the four cited lines landed in doc
      comments. Cite a function name, not a line, for anything that will outlive one commit.
      **The per-band accumulation is deliberately not compensated**, while the per-subsegment sum
      above it is. That is not an oversight: the inner reduction runs over at most 3 Teilquelle
      heights × `NumBeiblattOctaveBands` terms, the fixed-length case
      `docs/policies/determinism.md` §3 exempts; the outer one grows with the model. The asymmetry
      is now stated on the kernel so it is not “fixed” by someone who reads only half of it.
- [ ] **Move the module contract into `framework` and register implementations.** The dispatch
      exists and the switch is gone; what is still CLI-side is the contract. A standard's four
      halves — its options (`run_options.go`), its extraction (`run_extract_*.go`), its compute
      (its own package) and its persistence (`run_persist.go`) — are bound in `runModuleTable`
      instead of in the module itself, so `standards.NewRegistry` still registers bare descriptors.
      This bullet and "move the run pipeline out of `app/cli`" below are one piece of work in two
      orders: move the per-standard halves into `internal/standards/modules`, and the contract
      follows them into `framework`.
      The output oracle stays what it is: `run_results_digest_test.go` runs every registered
      standard end to end twice and pins a SHA-256 over every file under
      `.noise/runs/<id>/results/`, plus the normalized `run-summary.json` and `provenance.json`,
      against `testdata/digest/<standard>.golden.json`. Only four fields are normalized away
      (`created_at`, `run_id`, `generated_at`, `tool_version`) and each must still be present, so
      the harness cannot quietly stop pinning something. **Do not regenerate those goldens during
      the move** — a diff there is the refactor changing output, not a snapshot needing an update.
      The acceptance fixtures do not cover this: they import the standards packages directly and
      never reach the CLI run path.
- [ ] **Move the run pipeline out of `app/cli`** into `internal/engine` (or `internal/app/run`) as
      `Run(ctx, store, req) (RunResult, error)`. Today `api/httpv1` reaches it by fork/exec'ing its
      own binary (`handler.go:408-478`, parsing exit code 2 back into a typed error) — fork/exec
      used as dependency inversion. Delete `newCLIProcessRunExecutor` once this lands.
- [ ] **Serialise manifest read-modify-write.** `POST /api/v1/model`, `POST /api/v1/runs` and
      the two import handlers each `Load` → mutate → `Save` `.noise/project.json` with no lock, and
      the `aconiq run` subprocess writes the same manifest from another process, so two concurrent
      requests can drop each other's change. A `sync.Mutex` shared by the handlers would cover the
      in-process half (`Handler` uses value receivers, so it has to be a pointer field or a
      package-level lock); the cross-process half needs a file lock or waits for the run pipeline
      to move in-process (item above), after which the mutex is the whole fix.
- [ ] **Route the SoundPLAN import through `Store.SaveModel`.** `persistSoundPlanArtifacts`
      (`import_soundplan.go`) still writes the three model files itself because it adds a fourth
      artifact ref in the same manifest save; let `SaveModel` take extra refs so `.noise/model/`
      has one writer.
- [ ] **Generalise the engine.** `engine/runner.go:20,485` hard-codes `dummy/freefield`, so all ten
      real standards run single-threaded from the CLI, bypassing chunking, caching and
      cancellation — which makes the "identical output regardless of worker count" guarantee
      vacuous for everything a user would actually run. Parameterise on a
      `Kernel func(ctx, []Receiver) ([]ReceiverResult, error)`.
- [ ] Include the resolved standard tuple in the chunk cache key (`runner.go:549-573` omits it) —
      harmless only while the engine is single-standard.
- [ ] **Thread `cmd.Context()` end to end.** `run_pipeline.go:192` and `bench.go:395` pass
      `context.Background()`, so Ctrl-C during a long grid calculation does nothing; the engine's
      full `context.Canceled` handling (`runner.go:210-244`) is unreachable. **No `Compute*` in any
      standards module takes a context** — 10 modules, zero cancellation. `report/export/gpkg.go`
      calls `context.Background()` 7 times on the export path for the same reason.
- [ ] Fix the feeder-goroutine leak: `runner.go:346-389` returns on error without cancelling, so
      if every worker has exited the feeder blocks forever on `jobs <- chunk`. Use
      `errgroup.WithContext` or `defer cancel()`.
- [ ] Collapse the mechanical duplication — ~4 300 non-test LOC, about 9 % of the backend.
      Live constraint for anything touching `bub/road`: it aliases `cnossos/road`'s `RoadSource`,
      whose `Validate` accepts only the CNOSSOS categories, so BUB sources must be validated
      through `bubroad.ValidateSource`; the struct's JSON tag is `road_category` for both
      standards, while the CLI parameter stays `road_function_class`.
  - [ ] `buf/aircraft` → alias package over `cnossos/aircraft`. `compute.go` and `emission.go`
        are **byte-identical**; `propagation.go` differs by one constant. `bub/rail` and
        `bub/industry` already demonstrate the correct 211-LOC alias pattern. **−1 050 LOC.**
  - [ ] Lift `ComputeReceiverOutputs` (12 copies), `ProvenanceMetadata` (10) and
        `geometricDivergence` (8). `PeriodLevels`/`ReceiverIndicators`/`ComputeLden` are done — the
        END directive's defining formula had six byte-identical copies and now lives once in
        `internal/acoustics` — and `ExportResultBundle` is down from 12 copies to 4: the eight END
        modules delegate to `acoustics.ExportENDBundle`, and what is left (`rls19`, `schall03`,
        `iso9613`, `beb`) publishes a different indicator set, so it is a second shared bundle
        rather than the same one.
  - [ ] Replace the 11 `persist*RunOutputs` and 10 `hash*Outputs` clones with two generics.
  - [x] ~~Merge `extractCnossosAircraftSources` / `extractBUFAircraftSources`.~~ Done in the
        extraction pass: one `buildAircraftSource` serves both, and the BUF path maps the result
        across. **This item's premise was wrong** — the old directive's "the source/output types
        differ" was _true_. `cnossosaircraft.AircraftSource` and `bufaircraft.AircraftSource` are
        field-for-field identical but are distinct Go types whose nested `AirportRef` and
        `MovementPeriod` are declared per package, so the compiler rejects a conversion between
        them; the line cited above converts the _options_ type, which is a different thing. The
        mapping disappears when `buf/aircraft` becomes an alias package, above.
  - [ ] Consolidate 7 copies of `writeJSONFile`/`writeJSON`.
  - [ ] Three END runs omit `reporting_precision_db` from their run summary — `cnossos-industry`,
        `bub-industry` and `buf-aircraft` — while the other five write it. The collapse into
        `endPersistSpecs` preserved the difference rather than fixing it, because the digest goldens
        pin the summary and a behaviour change does not belong inside a refactor. Decide which way
        it goes and regenerate the three goldens deliberately.
- [ ] Move `internal/report/results` to `internal/results` — every standards module imports it,
      so compute currently depends on the reporting tree.
- [ ] Replace `context.Value` dependency injection (`app/cli/root.go:127-149`) with an explicit
      `app` struct.
- [ ] Give `domain/errors` real reach into the domain. It appears in 20 files, essentially all at
      the CLI/API boundary, so classification is retrofitted at the edge instead of carried from
      where the failure occurs. There are 690 inline `errors.New` strings in non-test code and zero
      package-level sentinels; add typed/sentinel errors for the recurring conditions and classify
      at the source. This is what makes the exit-code taxonomy testable (Priority 3).
- [ ] Fix the reachable panic on user input: `report/export/conversion.go:14` `mustUint16` is
      called with the user-supplied project CRS's EPSG code (`geotiff.go:337,345`).
- [ ] Split the god files. Re-measured: `api/httpv1/handler.go` (1 358), `app/cli/export.go`
      (1 244), `report/reporting/report.go` (1 192), `run_options.go` (1 016), `run_persist.go`
      (914). `run_extract.go` and `run_pipeline.go` are done — 3 087 → 36 and 887 → 238 — and
      nothing now exceeds the project's own configured `revive file-length-limit: 1500`, so the
      remaining question is readability rather than a breached limit.
- [ ] `extractCnossosIndustrySources` silently drops a supported source type. Its geometry switch
      has no `default` arm, so a type listed in the standard's `SupportedSourceTypes` that is
      neither `point` nor `area` yields no sources and no error. Preserved and documented by the
      extraction pass rather than changed inside a behaviour-preserving refactor; decide whether it
      should be an error.
- [ ] Delete dead code: `newPlaceholderCommand` (`root.go:107`), `mustFinite`
      (`cnossos/road/emission.go:344`), the unused `cfg` param (`cnossos/industry/propagation.go:126`),
      `schall03`'s unexported-candidate `Beiblatt3RetarderRangierenLevel` and `OctaveBands`, and the
      never-called `roundToWholeDB` (`schall03/indicators.go:38`).

## Priority 8 — Frontend correctness and rework

The shipped crashes and the correctness defects behind them are fixed (`be19871`, `e55e366`,
`ad47eaa`): the `calcArea` autosave data loss, the map being destroyed and rebuilt on every model
edit, three render-phase `setState` sites, the global Ctrl+Z hijacking text inputs, object URLs
revoked while still rendered, swallowed layer errors, `loadReceivers` bypassing the command stack,
`CommandStack` never unsubscribing, four module-scope message calls freezing labels at import time,
and the bundle budget, which is now measured in gzipped bytes against a real build (map chunk
314.3 KB gz against a 400 KB budget) and enforced in `frontend-ci.yml`. `gh-pages.yml` no longer
deploys to production Pages with no test gate. Unit tests went 112 → 132.

Three items on the old list turned out to rest on false premises and are recorded in `ad47eaa`
rather than here: the "receiver" case was not missing, `ValidationPanel` was an unfinished feature
rather than dead code and has been wired up, and "58 unused message keys" was 38, of which ~24 were
un-wired i18n whose English text is hardcoded in the pages — deleting those would have cemented an
English-only UI, so they were wired up instead.

A six-lens review (architecture, UX, visual system, map editing, a11y + i18n, code quality) rated the
frontend ≈ 4/10 overall. The foundations hold — oklch tokens, shadcn primitives, the command-stack
model store, paraglide plumbing, strict TypeScript and ESLint — and are kept. The product layer on
top does not close the loop: in API mode the drawn model never reaches the backend, results never
reach the map, geometry cannot be edited after creation, three visual dialects coexist across eight
pages, and ~50 strings bypass i18n. The rework below is ordered so that contracts land before
components and components before pages; a phase is not started until the one above it is green.

One belief this review corrected: `run.tsx` is not untested. `run.test.tsx` drives the setup dialog
through Radix in 15 tests and is the template for page tests. `results.tsx` and `export.tsx` have
none.

Decisions taken up front: the API-mode model contract is a new `POST /api/v1/model` (backend work is
in scope); the route structure changes to the target IA below; the German UI uses the Sie/impersonal
register and Fachbegriffe (Immissionsort, Schallquelle, Schallschirm/Lärmschutzwand, Norm).

### Phase A — Contracts and robustness

Landed; the gates below hold and every later phase builds on them:

- [x] `POST /api/v1/model` replaces the project model through `projectfs.Store.SaveModel`, the path
      `aconiq import` uses too (`061524e`, hardened in `2379e37`). Refused models write nothing.
- [x] One `Backend` interface selected once; pages and hooks branch on `capabilities`, never on the
      mode, and every non-OK response goes through `api-error.ts` (`f07bb20`).
- [x] "Save to project" in the header; `dirty` means "differs from the project", so imports and
      restored drafts start dirty, only a successful save clears it, and the run dialog refuses to
      start on unsaved changes (`fb2cba7`). The calculation area is not in the payload — see the
      Phase C item.
- [x] Runs poll by activity (2 s active, 15 s idle, never in browser mode); the run log polls while
      running and is invalidated from the runs list on completion (`6e96e7d`, `afe6333`).
- [x] Browser-mode runs live in IndexedDB as one versioned document with a 20-run cap and quota
      eviction; the draft document is versioned too (`8b8fa1a`).
- [x] A lost WebGL context recovers on its own; the "Map unavailable" panel has Retry (`c282100`).
- [x] The E2E suite runs in WASM mode under `/Aconiq/` (`just fe-e2e`, `frontend-e2e` job) and
      carries the axe baseline (`fb80e87`, `11d9894`): WCAG A/AA clean on every route in `de` and
      `en`; `best-practice` findings are pinned per route in `KNOWN_VIOLATIONS` in both directions,
      so the list must be pruned as Phase B lands.

### Phase B — Design system foundation

Landed; the gates below hold and the pages are built on them:

- [x] Semantic tokens `success`/`warning`/`info` (+ `-foreground`) beside `destructive`, with
      `tokens.test.ts` measuring every text/surface pair at ≥ 4.5:1 in both themes — dark
      `--destructive` was 2.1:1 (`4125c6a`). IBM Plex is self-hosted (`b079903`); the type scale is
      six steps, 11–20 px, and `rounded-lg` is the largest radius, so `text-2xl`+ and `rounded-xl`+
      emit no CSS by design (`f0bc32a`, `docs/frontend-design-system.md`); `prefers-reduced-motion`
      collapses animation (`055838f`); the axe baseline also runs under `colorScheme: "dark"`
      (`3edb89c`).
- [x] The eleven missing shadcn primitives with a Toaster mounted once (`7c5219f`, `831bde8`), and
      the shared `StatusBadge`, `Callout`, `PageHeader`/`SectionHeading`, `KeyValueList`,
      `EmptyState`, `MasterDetail`/`ListItem`, `CopyButton`/`CopyField`, `MapPanel` and the
      locale-aware `ui/format.ts` (`3b8943a`, `4c23505`, `1c0effb`).
- [x] Every page and map panel sits on those components: no Tailwind palette class and no `dark:`
      patch is left outside `ui/components/`, the tab strips are `Tabs`, the checkbox and the
      boolean parameter are `Checkbox`/`Switch`, and `window.alert` is gone (`001c7e7`,
      `ac9e965`, `4cec7f3`, `5dbac60`, `062c178`, `7f9d17d`, `9f4c5d9`, `0c533f1`). The populated
      map workspace carries an `sr-only` `<h2>`, so `waitForPage` in `e2e/app.ts` holds for every
      route state. Still hardcoded: the "Map unavailable"/"Retry" strings in `map-view.tsx`,
      because `map-view.test.tsx` mocks the messages module to `""` — Phase E.
- [x] One `<main>`, a labelled `<nav>` with `aria-current`, a skip link, `<html lang>` from the
      locale, contiguous headings and one `useGlobalShortcut` hook with the text-entry guard
      (`9935196`, `887ab91`, `21c3111`). `KNOWN_VIOLATIONS` in `e2e/a11y.spec.ts` is empty on every
      route and stays two-way, so a best-practice regression fails the suite.

### Phase C — Information architecture and pages

The backend contracts this phase needs have landed, and the characterisation nets are in place
before the pages they protect are cut apart. One caveat on the citations below: the nine hashes in
the landed items are pre-rebase objects reachable from no branch — every phase reaches `main`
squashed, so this phase is `87da006` and nothing else. They are accurate as history and useless as
`git show` targets, which is the general rule for a per-branch hash in this file.

- [x] **The calculation area is a model feature.** `calc-area` is a fifth kind in the v1 GeoJSON
      schema — Polygon only, no `height_m`, at most one (`model.calc_area.duplicate` refuses a
      second) — so it travels through `POST /api/v1/model`, is reprojected by `NormalizeWithCRS`
      like everything else, and is honoured by `aconiq run` as well as by the API (`32b3175`,
      `35a8942`). `buildReceiversFromPoints` takes the extent from it when present; `run.log`
      records `grid_extent=calc_area|source_extent`, and the over-cap refusal names which extent
      produced it, because a user can now trip the 250 000-receiver cap by drawing. **Padding still
      applies to the drawn area** — `browser-backend.ts` already pads it and `browser-parity.test.ts`
      pins the two kernels together, so suppressing it here would be a silent divergence; set
      `grid_padding_m` to 0 for the area exactly. `schema_version` stays 1: it has write sites and
      no reader, and `ToFeatureCollection` does not emit it.
- [x] **`GET /api/v1/model` and a model hash on `ProjectStatusResponse`** (`ed8eebb`). Both, because
      they answer different questions: the GET returns the content a reloaded workspace needs, with
      `?crs=` reusing `NormalizeWithCRS` in reverse (the frontend has no proj4). **The hash is a
      receipt, never recomputed by the client** — `POST /api/v1/model` returns the hash of what it
      wrote, the draft stores it, and startup compares two strings. A canonical form hashed on both
      sides would need the spec implemented twice over coordinates that do not survive a
      WGS84→25832→WGS84 round trip. The normalized file is byte-deterministic for identical content,
      which is what makes a file-bytes hash a content identity; `TestSaveModelIsByteDeterministic`
      pins it. The model hash is part of the SSE dedupe key, or the stream would serve a stale hash
      forever with every test still green.
- [x] **`DELETE /api/v1/runs/{id}`** plus `aconiq delete-run` (`e6d3680`). Manifest first, then
      `os.OpenRoot` + `RemoveAll`, so a failed removal leaves orphan bytes rather than a manifest
      that lies. Refused with 409 while a run is still writing its directory. Export bundles are
      kept and reported in `retained_paths`: a bundle may already have been delivered. It answers
      200 with a body rather than 204, both so the UI can say the bundle was kept and because
      `http-backend.ts`'s request helper always parses JSON.
- [x] **One project-status builder** (`3f7e861`), which also closed the three `context` fields that
      were emitted but absent from schemas declaring `additionalProperties: false`
      (`StandardDescriptor` — required, it has no `omitempty` — plus `RunSummary` and
      `LastRunStatus`). Phase F's premise is generating a strict client from this document, so it
      cannot start by fixing backend bugs.
- [x] **Characterisation nets before the splits** (`2e0d0d6`, `5539395`, `e3c1507`): first tests
      for `results.tsx` and `export.tsx`, and coverage for `run.tsx`'s timeline and cascade. The
      cascade test asserts the _sequence_ of values a parameter field held, via a `MutationObserver`
      — a final-DOM assertion cannot catch an effect-based rewrite of the render-phase update,
      because `fireEvent` flushes effects before it returns. A locale key-parity test (`35c5b53`)
      guards the ~40 key changes the rest of this phase makes; nothing in `src/` reads `de.json`,
      so a forgotten translation was previously invisible.

- [x] **The six defects those nets pinned are fixed.** The nets worked as intended: each fix was
      the expectation flip the comment promised. Three of them were wider than the entry said.
      The colon belonged to 18 `label_*` messages, not 19, and those messages are rendered _bare_
      in four more places than the one this file named — `import.tsx`'s `KeyValueList` terms,
      the map layer toggles (the legend read "Gebäude:"), and five headings and `<Label>`s in
      `run.tsx` — so stripping the colon from the message fixed all of them and only three sites
      needed one added back. `sheet.tsx` carried the same hardcoded English "Close" as
      `dialog.tsx`. And removing the false "PDF generation is planned" notice emptied its whole
      section, which went with it. The timeline is now an `<ol>` whose markers carry
      `role="img"` and a localised status name, with `aria-current="step"` on the active one; both
      test helpers read roles and accessible names instead of `animate-spin` and
      `parentElement.parentElement`. A new locale-parity assertion forbids a trailing colon on any
      `label_*` message in either catalogue, so the class cannot come back.

- [x] **The routes say what is selected** (#28). `/results` and `/export` each carry a bare index
      route and a `:runId` child holding the _same_ element — React Router keys no rendered route,
      so one instance spans the index and every run, and the detail pane's open tab survives a
      selection change; a parent/`<Outlet/>` split or a second `lazy()` would remount it. Four
      constraints the remaining pages inherit. An unresolvable id has **three** outcomes, not two
      — absent from the run list, present but not completed, or selectable — and "unknown" is
      decided on `data !== undefined`, so a deep link that lands before `useRuns` resolves shows
      the spinner rather than a warning at a good URL. A row that changes the URL takes
      `ListItem`'s `to`, not `onSelect`: a button has no href, no middle-click and no links rotor
      entry, and **no axe rule catches it**. The rail matches route patterns (`/results/:runId?`),
      never a pathname prefix, so it neither drops the highlight under `/results/<id>` nor lights
      a section on the not-found page. And retired paths get no redirect — `/map` falls through
      to a not-found page inside the shell (an `errorElement` replaces the layout, so
      `waitForPage` would hang on a missing `h1` instead of reporting the error), because a
      redirect keeps a missed migration working forever.
- [x] **Target IA, the pages half** (#29). `/` is one project page — get-started, validation
      summary, backend health, project facts — replacing a welcome page whose only live content
      was a project summary under three cards of static marketing copy and a status page carrying
      that same summary again. Three copies of `ProjectSummary` became one. `/welcome` and
      `/status` are gone with no redirects; sixteen keys went with them.
      **The page heading renders outside every query branch**, and a test asserts a contiguous
      outline across all twelve combinations of the two requests' states: `waitForPage` waits for
      a header `h1` _and_ an `h2`/`h3` inside `main`, so a heading inside a success branch would
      make a stopped backend hang every e2e spec on the route for the full Playwright timeout
      rather than fail legibly.
      `/model` mounts the map from the start, with the empty-model hint as a labelled region over
      it rather than instead of it — the screen telling the user to start a workspace was
      previously the one screen with no way to draw one. `ModeChip` names the backend in the
      header (decorative: an `aria-label` on a role-less element is what axe's
      `aria-prohibited-attr` flags), and `ModeGate` disables the export dialog's Generate button
      with a reason instead of hiding it. Settings is down to General and Connection, ids kept
      because `?category=` is a URL.
      Three things this turned up. **`ModeGate` uses `aria-disabled`, not `disabled`** — a
      disabled button takes no focus and fires no pointer events, so a tooltip over one never
      opens; the tests hover and Tab for real, because every DOM-shaped assertion passes while the
      tooltip is invisible. The same shape is still live in `undo-redo-bar.tsx:33-47`, though it
      costs far less there — **left open, listed under Phase D**.
      **`validateModel` must not be called from the UI**; `useModelValidation` passes the
      receivers and answers "empty" above the validator, so a fresh install no longer reads "1
      error" in hardcoded English. And **terra-draw tore down against a removed map**: React
      destroys a deleted subtree's effects parent-first, so `MapView`'s `map.remove()` ran before
      `useDraw`'s `draw.stop()` and threw `getSource` of undefined, which React surfaced as a
      crashed page on whatever route the user had just navigated to. Leaving `/model` for
      `/import` reproduced it every time; it was invisible only because the map never mounted on
      an empty model.
- [x] **Drawing worked nowhere, and now works** (#29). `useDraw` ran one level above the
      `MapContext` provider, so the map was always null, the terra-draw init effect
      early-returned, and a draw-tool click only moved React state. A `DrawProvider` rendered
      _inside_ `MapView` owns the instance, which makes the position structural. The test stubs
      the terra-draw library rather than the hook — mocking the hook is what hid this — and
      `map.test.tsx`'s `MapView` stub now renders its children inside a real context, so anything
      laid over the map is exercised at all.
- [x] **The workspace hydrates from the project, and the drawn area reaches it** (#26). Both
      halves together, because either alone loses the area on reload:
      `use-project-hydration.ts` sits in `RootLayout` beside `useAutosave`, gated on
      `runsAgainstSavedModel`, and `ModelPayload.calcArea` is required so the compiler
      enumerated the emit sites. Four constraints follow.
      **Exactly one writer may put a hash on a draft** — `useProjectSync.save()`, in the branch
      that has established the store did not move during the request. An autosave that carried
      the last save's receipt over newer edits would be restored _clean_ on the next start, a
      workspace silently claiming to be the project; hydration writes no draft at all, or it
      would overwrite the divergent one that must still be offered.
      **The id lives in `properties.id`, not the GeoJSON `id` member** —
      `ToFeatureCollection` writes it there and `projectfs` stores it that way, so a reader
      that consults only the member mints fresh UUIDs and the next save rewrites every id in
      the project. An area the user drew has no stored id, so one is derived at emit time from
      the ids already in the payload: nothing reserves `calc-area` in an imported file, and the
      backend refuses the whole model with `feature.id.duplicate` when two features share one.
      **A load is not a hydration** — `loadModel` sets `dirty` because its content comes from
      outside the project; hydrated content comes from it and lands through `hydrateModel`
      clean, or the unload guard arms on every reload and the autosave writes a hash-less draft
      of the project's own model two seconds later.
      **Startup must not act on a question it has not answered.** Child effects run before
      parent effects, so the draft offer is decided in the hydration store rather than in
      `DraftBanner`; an errored project status is not an answer of "no model", so it leaves the
      hook armed for a later refetch; and no route renders until the decision settles, because
      every route is editable and an edit made over the not-yet-hydrated store makes the next
      save replace the project with it.

- [x] **Placeholders and apologies are gone, and the hand-offs are commands.** The raster
      colour-ramp/probe block, the "Phase 24+" description, the run-to-run diff notice and the
      permanently disabled Cancel button are removed, and seven message keys retired with them.
      Five constraints follow.
      **`api/cli.ts` is the only place an `aconiq` command is spelled**, and it mirrors
      `export.go`'s flag surface: `EXPORT_FORMATS` is a `const` tuple, so a bad `--format` is a
      type error, and flags are emitted in the registration order `export.go` uses. It carries
      `--run-id`, `--format` and `--pdf` and nothing else — add a flag when a call site wants
      one, not before.
      **A blank `--run-id` is a correctness bug, not a formatting one.** `aconiq export`
      defaults that flag to the _latest_ run, so a command built from an unselected id acts on
      a different run than the one on screen. The builder substitutes a placeholder, and that
      fallback lives there rather than at each call site.
      **`ArtifactRef` carries no run id** — it is `id`, `kind`, `path`, `created_at` — so
      anything building a `--run-id` command out of an artifact needs the run threaded to it.
      `RasterArtifactCard` takes one from `RasterTab`.
      **Every kind `export.go` appends an `ArtifactRef` for needs a row in
      `EXPORT_KIND_LABELS`, added with it.** The PDF half of the old entry had already landed
      in `759d119`; the live instances were `export.report_typst` and
      `export.assessment_16bimschv_json`, both now labelled. `kindMeta`'s raw-string fallback
      stays as the guard for the next one, not as a design. Two CLI hand-offs were false rather
      than merely prose: `/results` and `/export` sent the user to the CLI while `/run` starts
      runs in both modes and the export header already carries a New Export button. Both empty
      states now point at the UI that does the work.
      **`map/color-ramp.ts` was deliberately kept although it had zero importers and 0 % coverage**,
      because "Results on the map" below named `NOISE_LEVEL_RAMP`. That instruction has expired:
      `result-layers.tsx` imports it, so the module is now reached by the code rather than only by
      a promise in this file.
- [x] **Split run/results/export** (`9b591a0`..`0a57d78`). `pages/run.tsx` is a route module
      again and its parts live in `src/run/`, with `useRunSetupSelection` and `useRunFromRoute`
      carrying the cascade and the URL→run rule. Six constraints follow.
      **Not `pages/run/` as this file asked for**: `pages/map.tsx` already answers this the
      other way, and keeping the route module where it was made the move reviewable as a move.
      **The parameter seeding stays render-phase** — an effect paints the fields empty for one
      commit, which `run.test.tsx`'s `MutationObserver` pins as `["999", "25"]`.
      **`useRunFromRoute` takes eligibility as a parameter** because `/results` resolves against
      completed runs and `/export` against every run; the run page's own first-visible fallback
      stayed inline rather than share a name with a rule that refuses exactly that.
      **A standard's name is a name, and its evidence qualifier is derived from
      `evidence_tier`** — never from a second table keyed by id, which is the parallel signal
      `AGENTS.md` forbids and which `STANDARD_LABELS` had already demonstrated by being keyed on
      a _parameter_ name and never matching anything. `useStandardLabel` resolves the tier for
      the sites that hold only a `RunSummary`.
      **Parameter labels are derived, not translated.** The RLS-19 vehicle classes get the
      terminology the catalogue already carries; everything else keeps the backend's own name,
      humanised. Composing German across all 98 would have invented `Gleisrauheitsklasse` for a
      published norm — Phase E owns that. The raw name stays on screen, because it is what
      `--param` takes.
      **`PERSISTED_STATE_VERSION` stayed at 1, against what this file called mandatory.**
      `decodePersisted` throws on an unknown version and `loadState` swallows that into
      `initialState()`, so the bump buys nothing upward — `decodeState` already reads field by
      field — and downward it discards the user's runs. `runHighWaterMark` is additive, raised
      in `setRun`, and its `max(stored ids)` fallback is exact for every document written before
      it, because nothing could delete a run then.
      **A confirmed destructive action must say where focus goes.** Radix restores it to the
      trigger, which these actions delete; no axe rule covers landing on `<body>`, and the
      run-delete case only settles after the mutation does, so it is answered in an effect
      rather than in `onCloseAutoFocus`.
      Still open from the review: the frontend's list of registered standard ids
      (`standards-meta.ts`) is a copy, because the Go registry composes its ids inside each
      module's `Descriptor()` and no file names them all. A standards manifest generated from
      the registry and read by both targets would make the drift a test failure instead of a raw
      id on screen.
- [x] **The receiver table is a window, and it says how big the table is**
      (`f315baf`..`7f88df7`). `ReceiversTab` moved to `src/results/receiver-table.tsx` and
      mounts only the rows near the viewport. Five constraints stay live; each is argued where
      it is enforced.
      **The window is spacer `<tr>`s, never absolute positioning** — absolutely positioned rows
      force `display: flex` onto every `<tr>`, and the `scope="col"` and `aria-sort` on the
      headers then describe a grid that no longer exists. `/results` is `NONE` in
      `e2e/a11y.spec.ts`, and that list is checked in both directions.
      **`aria-rowcount` and `aria-rowindex` travel with the rows**, or the table announces its
      window as its size.
      **The scroll element needs a resolved `max-height` of its own.** `overflow-auto` bounds
      nothing; without it the div grows to the spacer rows, `virtual-core` reads that back as
      the viewport, and every row mounts. The ancestor in `pages/results.tsx` is what scrolled,
      and the virtualizer does not watch it.
      **`virtual-core` measures with `offsetWidth`/`offsetHeight`, not `getBoundingClientRect`**,
      which jsdom answers with 0 — where no rows mount at all. The viewport stub in
      `results.test.tsx` is load-bearing for 12 other assertions.
      **The sort collator keeps the default options.** `numeric: true` would put "R2" before
      "R10", which is a behaviour change owed its own commit and its own test.
      The "~100k" spread ceiling this file used to assert is not a constant: a spread takes one
      stack slot per argument, so where it gives out depends on what the caller already spent —
      125 000 survives in bare node and throws under vitest. `results/summarise.ts` counts in
      one loop and has no ceiling.
- [x] **Import page** (`9349769`..`9bc7322`). The wizard reads the whole v1 schema, adds to the
      workspace or replaces it, and its three flows live in `src/import/` beside the route module.
      Four constraints are still live.
      **Skipping, not re-minting, is what makes a re-import idempotent**, over one id namespace
      spanning features and receivers, because `validateProjectModel` checks them as one. A merge
      that changes nothing pushes no command, or an import that did nothing would clear the redo
      stack.
      **Add needs no confirmation and Replace does**: Add is a single undo, Replace resets the
      command stack. An empty workspace gets one button and neither question.
      **The done step revalidates the merged model** rather than filtering the preview's report by
      the landed ids — a skip can resolve the finding it is filtered by. The preview names a
      feature id without linking it, because nothing is in the store until Add or Replace is
      chosen.
      **`model.empty` is answered above the validator**, whose message is hardcoded English. A file
      holding only a calculation area runs no validator at all, and still counts as one object to
      import: `countModelObjects` is the only definition of that count.
- [x] **One receiver CSV, Go's spelling, pinned across the tree boundary.** `encoding/csv` is
      canonical — comma, LF, a trailing newline on every record including the last, minimal
      quoting — and `frontend/src/model/receiver-csv.ts` mirrors it for both the page download and
      the stored browser artifact. Three live constraints follow.
      **Go's `unicode.IsSpace` is not JavaScript's `\s`**: Go includes U+0085 and excludes U+FEFF,
      `\s` does the opposite, so the leading-whitespace quoting rule needs an explicit character
      class in TS and the fixture carries all three leading characters to prove it.
      **`strconv.FormatFloat(v,'f',-1,64)` and `String(v)` share their digits but not their
      presentation** — JS switches to an exponent past 1e21 and below 1e-6 and prints `-0` as `0`,
      so the TS formatter expands and special-cases; 199 894 doubles built from raw bit patterns
      agreed on every one.
      **A fixture whose point is its bytes must be exempt from EOL normalization** — the
      `csv-parity` directory carries `.gitattributes` with `* -text`, or git would rewrite the CR
      the self-test exists to check. That directory is the second golden directory read from the
      frontend tree; `docs/testing/golden-tests.md` no longer calls `testdata/parity/` the only one.
      `PERSISTED_STATE_VERSION` was deliberately not bumped: that guard refuses a document the code
      cannot read, and re-spelling one downloadable does not justify discarding a user's model and
      twenty runs. Runs already in IndexedDB keep their old bytes; both spellings parse.
- [x] **`ParameterDefinition` carries a unit**, as a short SI-style symbol from a `framework`
      constant, required through `cloneParameterSchema` or it vanishes in profile resolution, and
      published on `GET /api/v1/standards` and in the OpenAPI document. 75 of the 117 parameters
      carry one; 42 are genuinely dimensionless. `standards-meta.ts` must therefore read `unit`
      from the API rather than being born with a table of its own — the drift test this entry used
      to ask for is unnecessary if the mirror is never written.
      Two constraints: the suffix conventions cover most names but **not the RLS-19 traffic
      counts**, which carry `1/h` from their descriptions and not from `traffic_day_pkw`, so the
      suffix drift test says in its own comment that it cannot see them. And `c0_met` is **not**
      dimensionless as this file previously implied — `MeteorologicalCorrection` feeds C0 into
      `10^(-0.1·C_met)`, so it carries dB per ISO 9613-2 Eq. 22.
- [x] **`aconiq import --soundplan` emits a `calc-area` feature.** An imported project's auto-grid
      now matches the original's extent: the run log says `grid_extent=calc_area`. The z ordinate
      is dropped from the ring and kept as `soundplan_base_elevation_m`, following what the
      building appender already does. Closure is decided in **2D**, deliberately: the import
      report's `IsClosed` compares x, y _and_ z, so a footprint that closes in plan but differs in
      elevation would otherwise gain a zero-length closing segment.
- [x] **The model's `calc-area` wins the raster comparison; the import report is the fallback**
      (`5432253`, `8b31087`). `calc_area_source` records which area was chosen and `calc_area_role`
      what it did there — the GM-metadata path places receivers from the grid's own origin and
      consults the area only for the row direction, so `model` there does not mean "the model's area
      placed these".
      **An agreement tolerance is now possible and still unset.** It was blocked while the map
      save path moved geometry further than the grid resolution — any threshold below 5 m fired on
      a save that changed nothing, any above it hid a real edit. Priority 1.6 closed that; the save
      path now moves a vertex by nanometres, so a tolerance can be chosen on what a real edit looks
      like rather than around a defect. The comparison warns on the vertex
      count after normalising closure and records `calc_area_bounds_delta` with its unit, which is
      the project CRS's axis unit and not always metres.
      **`ACONIQ_SOUNDPLAN_FIXTURES` is what makes the licensed-fixture suite visible** — without it
      `TestCompareSoundPlanReceivers` and the SoundPLAN import tests skip and the run still reports
      green.

### Phase D — Map workspace

- [ ] **A `disabled` Button under a Radix tooltip is the live constraint, not an open item.**
      Every control on the map that refuses an action now uses `aria-disabled` plus a
      click handler that swallows activation — `ui/mode-gate.tsx` carries the argument,
      `map/draw-toolbar.tsx` and `map/undo-redo-bar.tsx` both use it. `disabled` puts
      `disabled:pointer-events-none` on the button and takes it out of the focus order, so a
      tooltip whose trigger is one never opens, by mouse or keyboard, in exactly the state it
      describes. Two corrections the undo/redo fix produced, worth keeping for the next control:
      the `aria-label` and the tooltip being byte-identical is what made that instance harmless,
      so severity has to be read off the tooltip's own text rather than off the shape; and jsdom
      computes no Tailwind stylesheet, so a `hover` + `findByRole("tooltip")` assertion passes
      over a real `disabled` button too — `aria-disabled` and Tab-reachability are what catch the
      regression (`map/draw-toolbar.test.tsx`, `map/undo-redo-bar.test.tsx`).
- [x] **The stale `soundplan_base_elevation_m` comments are corrected** (#51). Two sites claimed a
      save from the map drops the property, which `calcAreaToGeoJSON`'s passthrough ended:
      `calcAreaFromModel` and the `without base elevation` case in `compare_raster_test.go`.
      Two live constraints, and the second is why this took a second pass.
      **An absent property records no provenance.** The first correction asserted that an absent
      value meant a _drawn_ area — trading one over-claim for another, since an ordinary GeoJSON
      import carries the property only if its source did. Absence says that no base elevation was
      recorded and nothing more; 0 is the right reading either way.
      **This entry's own claim that the reread "came back empty" was false**, and review caught what
      a grep over non-test files did not: the test comment repeated the same stale sentence. The
      passthrough also carries a rule worth keeping — a carried `properties.id` is rewritten to the
      id `resolveCalcAreaID` settled on, because `featureID` reads `properties.id` before the
      GeoJSON `id` member and a stale one would recreate the `feature.id.duplicate` the resolution
      just stepped past.
- [x] **The selected feature is marked, and `FeaturePopup` is gone** (#50). Two constraints it
      leaves live: the highlight is a `feature-state` and never a model property, because the map
      is a projection of the model and a highlight that reached the store would be an input to a
      run; and it is written from `ModelLayers` alone, because `featuresToSourceGroups` splits the
      features into three sources by kind and only that component knows which one an id is drawn
      from — a state written against the wrong source paints nothing and reports no error. The
      docked editor is now the one click surface, and it shows a feature's properties as form
      fields rather than as an HTML string built by concatenation from values an import supplied.
- [x] **Geometry editing** (#56): the selected feature is fed into select mode and reshaped back
      into the model (`map/use-geometry-edit.ts`). Five constraints it leaves live. **A reshape is
      not a _draw_ finish** — `use-draw.ts`'s finish handler resets the mode to "static" and removes
      the feature, which in select mode would disarm the tool and delete the feature being reshaped
      — but the event is still the end of a gesture, and the only one that says so: terra-draw
      leaves a dropped feature selected, so `deselect` arrives once for a whole run of drags.
      Select-mode `finish` is therefore forwarded to the edit owner and is the per-drag commit and
      seal boundary. **Two commit triggers, chosen by the store's CRS**: in `DISPLAY_CRS` the
      inverse is the identity so every `change` commits, while over a metric model the latest
      geometry is held and projected once per drag — otherwise a drag costs one
      `POST /api/v1/transform` per mousemove. **The merge keeps the newest `execute` and the oldest
      `undo`** (`model/command-stack.ts`), because the geometry commands capture `previous` before
      they run. **Terra-draw's copy is its own**: an undo or any other outside write leaves it
      stale, so a session re-arms when the store's geometry object is no longer the one it wrote,
      and on `DrawApi.instanceEpoch`, because a basemap switch rebuilds the instance and empties its
      store. **One inverse transform, two callers** (`map/use-inverse-projection.ts`), so the map
      still has exactly one place where a coordinate travels into the model; it is newest-wins, and
      that is safe here only because terra-draw's geometry is cumulative. Left out: `Multi*`
      geometry is refused rather than silently reduced to its first ring, and a metric reshape whose
      projection fails is reported by the feature snapping back rather than by a notice.
- [ ] **Editor completeness**: the array-valued `schall03_operations` and `schall03_track_features`
      are still reported by count and not edited, so a rail model's Zugarten still have to be
      written into the model file by hand. That is a decision rather than a gap: a form for them is
      a second model editor, and half of one is how an Fz composition silently loses a vehicle.
      `schall03_track_features` carries coordinates in its properties besides, which puts it in
      `PROPERTY_GEOMETRIES` — browser mode cannot move those and refuses a model that carries one
      outright — so an editor for it would be filling in a model that half the app cannot run.
- [x] **The receiver's Gebietskategorie** (#52). The receiver panel writes
      `bimschv16_area_category`, the first of `categoryFromFeature`'s five tries, so a value set
      here outranks an older spelling the same receiver still carries;
      `model/bimschv16.test.ts` pins the four values, their labels and the key against
      `assessment.go`. Absent is not a default but `ExportEnvelope.Skipped`, and a select shows a
      stored spelling its vocabulary does not list rather than blanking it.
- [x] **The field tables, and the three rules they encode** (#50). The help under a field says what
      an _absent_ value means, and that answer is per property rather than per panel — "the run's
      default applies" is true for the RLS-19 road properties and false for every Parkplatz and
      Schall 03 one, where absent means either a refusal or a named reference row. The vocabularies
      and property names the editor writes are pinned against the Go extractors by
      `model/schall03.test.ts` and `wasm/parking-vocabulary.test.ts`; those pins are the only thing
      that catches a name drifting here, because a misspelled property is no type error anywhere —
      it is simply written, saved and never read. And a field's own bounds are enforced before the
      write, not by the `min`/`max` attributes, which mark an input invalid and let the value
      through: `step` is the stepper increment and never a constraint, so a whole-number field says
      so explicitly.
      `sourceType` is derived from the geometry, with the correction offered explicitly rather than
      applied on open — deriving alone would have left an imported contradiction visible and
      unrepairable, and a silent write would dirty a project for a panel that was only looked at.
- [x] **Results on the map, as receiver levels** (`c7bcbb3`). `ResultLayers` draws the newest
      completed run's receiver table from `NOISE_LEVEL_RAMP`. Four constraints follow.
      `newRunSummary` writes `project_crs` and `compute_crs` (provenance's own keys, which browser
      mode already wrote), because provenance is not an `ArtifactRef` and the API never serves it;
      the 13 digest goldens carry both, so a further summary key moves them again.
      The ramp is a decibel ramp and only paints a table whose `unit` says decibels: `beb-exposure`
      writes `"mixed"` and puts dwelling and person counts in `indicator_order`.
      The row→map link is offered only for an id the model store holds — `auto-grid` receiver ids
      are the run's own and `SelectRequest`/`FeatureEditor` find nothing under them — and is a real
      `<a>`, never a button that navigates, which no axe rule would catch.
      No style in `basemap.ts` declares `glyphs`, so a future label layer needs one added first.
- [ ] **Results on the map: the raster and the contours.** (2026-09-18) — partial: the three
      blockers are closed, the map layer is not. What landed, and the constraints each leaves live.
      **`results.RasterMetadata` carries a `georeference`** — the centre of cell (0,0), the pixel
      size and the row order — and populates the `CRS` field it had carried unwritten since it was
      added. Width, height and the georeference now travel as one `results.GridLayout` through all
      eleven standards modules, because the pair of loose ints threaded through a dozen signatures
      is how the third went missing. **Absence means "not a grid"**, never a grid at the origin:
      explicit receivers get no georeference, and a georeferenced format is then refused rather
      than written at the identity transform `rasterGeoTransform()` used to invent in silence.
      `exportfmt.GeoTransformFromGeoreference` is the one conversion to GDAL's corner convention,
      pinned against `InferGeoTransformFromReceivers` on the same grid so the switch could not move
      a raster already written; inference stays as the fallback for older sidecars.
      **Every `--format` file gets an `ArtifactRef`** — one per file, not per format, since GeoTIFF
      and COG write one per band — under `export.`-prefixed kinds, which is load-bearing because
      `projectfs.DeleteRun` selects on that prefix to keep a delivered bundle's bytes. The content
      endpoint stops answering `application/json` for everything that is not HTML or Markdown; it
      had been mislabelling `run.result.raster_binary` and `export.report_pdf` all along.
      **Browser mode stores real bytes**, through `model/raster-bin.ts` pinned to Go's writer by
      `testdata/raster-parity/`, in their own IndexedDB record rather than inside the state
      document — `persist` clones that document whole on every save. Three constraints worth
      carrying: `getArtifactURL` is synchronous and therefore refuses binary content;
      `instanceof ArrayBuffer` is the wrong check on a value read back from IndexedDB, because the
      structured clone can come from another realm; and **a byte record is deleted in the same
      IndexedDB transaction as the document that stopped naming it**. Neither order works across
      two transactions: delete first and a failed write leaves a retained run — `persistRun` puts
      the un-evicted list back — whose raster reads as missing, delete second and the eviction
      that exists to make room retries while the room is still occupied. `savePersistedStateForgetting`
      commits both or neither, deletes before the put.
      Still open, and the next batch: **the map layer itself** — `map/layers.ts` records that the
      `raster` and `contours` layer groups were removed because nothing added them. Then the
      map→table direction, which wants a receiver clicked on the map to scroll and mark its row;
      carrying the viewed run through to `/model` (`routes.tsx` has no params and `ResultLayers`
      always takes the newest completed run), so a row followed from an older run does not land on
      the newest run's levels; and a per-indicator unit on the receiver table, without which a
      mixed-unit run gets no map at all.
- [ ] **A deleted run takes its export artifact refs with it.** `dropRunArtifacts`
      (`io/projectfs/deleterun.go`) removes every ref belonging to the run and reports the
      `export.`-prefixed paths as `retained_paths` — the bytes survive, the manifest entries do
      not. That is deliberate and predates the format artifacts, but it now means a GeoTIFF the map
      could read becomes unreachable the moment its run is deleted, because an `ArtifactRef` id is
      the only handle `GET /api/v1/artifacts/{id}/content` takes. Decide whether a delivered
      bundle should keep addressable refs; do not change it as a side effect of the map work.
- [ ] **A raster byte write that hits quota is not retried.** `persistRun` recovers from quota by
      dropping the oldest run and writing again, but `startRun` writes the raster bytes _before_
      there is any document change to pair the eviction with, so it reports the failure instead.
      Covering it means inverting that order for the retry alone: evict (a document write, which
      frees the victim's bytes in the same transaction), then write the bytes, then the document
      naming them. Worth doing only alongside the sweep below, because the intermediate state —
      bytes stored under a document that does not yet name them — is exactly what the sweep
      reclaims.
- [ ] **Browser mode has no sweep for byte records nothing names.** Every deliberate path now
      deletes them — the run cap, eviction, `deleteRun`, and `startRun` when its own persist fails
      — but each is a caller that knows which ids it orphaned. A tab closed between
      `saveArtifactBytes` and the document write leaves a record no caller ever knew about, and
      only `clearPersistedState` removes it. `browser-storage` already keys these under
      `artifact-bytes:` and range-deletes the prefix, so listing them is a `getAllKeys` away; the
      work is the ordering, not the query. A sweep must run **once per session and under
      `withStoreLock`**, because `startRun` holds that lock across both writes and a sweep that
      does not would delete the bytes of a run another tab is mid-way through storing. It must
      also not run when the document was unreadable or the store unavailable: both leave the
      document in place deliberately, so what it names is unknown. Note that `reloadState()` calls
      `loadState()` on every write — a sweep placed there would delete the bytes of the run being
      written, and one placed in `ensureLoaded` can be reached from inside the lock it would need.
- [x] **CRS and basemap** (#53). The constraints it leaves live. The tile URL is a per-browser
      localStorage override (`map/tile-source.ts`, edited in Connection settings), so `basemap.ts`
      builds its styles on demand — one captured at import time would ignore the override — and a
      change takes effect on the _next_ map build. A tile failure falls back to `OFFLINE_STYLE` by
      **rebuilding** the map, not by `map.setStyle`: `ModelLayers` syncs on data and not on
      `styledata`, so swapping the style in place drops every model layer until the next edit. A
      rebuild must therefore carry the viewport, which `map-view.tsx` reads off the dying instance
      in its cleanup because the workspace page computes `center`/`zoom` once, and it must carry
      `layerVisibility`, which `ModelLayers` reapplies after adding the layers because a re-added
      layer comes back at its specification's visibility. And a tile failure must never reach
      `mapError` or the WebGL kill switch — that panel unmounts the canvas, and a failed tile is
      not a failed map, the same distinction a lost context already relies on. Only an `error`
      naming the basemap source counts; a failing model source is a model problem an offline
      basemap would hide.
      No proj4 was added and projection stays `aconiq.transform`'s, so the frontend cannot place a
      model where `aconiq run` would not — including the one direction that writes,
      `use-draw-projection.ts`'s inverse 4326 → `store.crs` transform on the draw-finish path. That
      path is the single deliberate exception to "a projection _of_ the model, never a source _for_
      it"; anything else on the map side that wants to write a coordinate is a second exception and
      needs arguing for. Drawing is gated on `backend.capabilities.canReprojectForDisplay`, not on
      the store's CRS — keep new gates on the capability, or a model the map can project will be
      refused for being metric.
- [x] **Keyboard path** (#54). `NewFeatureDialog` takes typed coordinates when it is opened with no
      drawn geometry, and `map/feature-list.tsx` lists features and receivers as Tab-reachable
      controls that set the page's `editingFeatureId`. Two constraints stay live. **The typed
      numbers are read as the store's own CRS and written into the store untouched, and no transform
      may be added to that path** — that is why it is no second exception to "a projection _of_ the
      model, never a source _for_ it"; `use-draw-projection.ts` needs its inverse only because
      terra-draw emits 4326 whatever the model holds. And the coordinate control must **not** take
      the `drawingDisabled` gate the drawing tools take: every reason that gate fires is a reason
      terra-draw's 4326 output could not be moved into the model's CRS, and on such a model typing is
      the only way left to add a feature.

### Phase E — i18n and German

- [ ] Validation messages become codes + params (`validate.ts:43-437` is English-only and rendered
      verbatim); add the keys still missing after Phase B (the "Map unavailable"/"Retry" strings in
      `map-view.tsx`, whose test mocks the messages module to `""` and has to stop first, and
      `ui/components/sidebar.tsx:276`'s hardcoded English "Toggle Sidebar" — a rendered control,
      found while localising the dialog and sheet close buttons); paraglide
      plural variants for `run{s}`; language switch without `location.reload()`; enable
      `react/jsx-no-literals` for `pages/`, `map/`, `ui/`.
- [ ] German terminology and register pass: Immissionsort, Schallquelle,
      Schallschirm/Lärmschutzwand, "Norm" consistently; Sie/impersonal register throughout (mixed
      du/Sie today); welcome copy aligned between `de.json` and `en.json`.

### Phase F — Tests, types and the kernel boundary

- [ ] `hooks.test.ts` against mocked `fetch` (`api/hooks.ts` leaves a third of its hooks
      unexercised), and a `browserBackend.startRun` test with a stubbed kernel compared to a backend
      golden. **`use-draw` and `model-layers` are done** — 95.6% and 89.8%, via
      `draw-provider.test.tsx` and `model-layers.test.tsx` — and `src/map` as a whole is now 94.0%,
      not the 37.3% over 1,765 statements this bullet used to quote. What survives of the original
      claim is narrower and still true: **the map-rebuild fix in `ad47eaa` has no test**, because
      `map-view.tsx`'s residual statements are MapLibre lifecycle that jsdom cannot reach. Faking a
      WebGL context would test the fake; it belongs in `frontend/e2e/`.
      The floors are **not** in `fe-ci` — they live in `frontend/vitest.config.ts` and are applied
      by an advisory `frontend-coverage` job, because a coverage regression must not be able to fail
      a required check. That job published a number less than half the truth from the day it was
      created until `45c98ef`; see `docs/testing/coverage.md` for what it was and why.
- [ ] **Four vendored shadcn components have no importers anywhere.**
      `ui/components/table.tsx`, `resizable.tsx`, `scroll-area.tsx` and `textarea.tsx` are 176
      statements that nothing in `src/` or `e2e/` imports. They are most of what keeps `src/ui` at
      77% while this repository's **own** shared components under `src/ui` sit at 91.5%, most of
      them at 100%. Testing them would be the purest coverage theatre available here, so the choice
      is to delete them or to keep them deliberately as design-system stock — a decision about what
      the design system holds on hand, not a coverage question.
      The remaining honest gaps after that are `src/wasm` (45.2%, the browser-side kernel loader —
      `kernel-node.ts` is what the parity suites exercise) and `src/layouts` (41.7%, 24 statements).
- [ ] Generate `client.ts` from `aconiq openapi` (openapi-typescript) and fail `fe-ci` on diff;
      delete the hand-written DTOs and the missing `generate-api-client.mjs` entry that
      `package.json` declares (`/api/v1/import/terrain` has no binding today). The three
      `context` schema gaps this bullet used to carry — `StandardDescriptor`, and the same defect
      found twice more in `RunSummary` and `LastRunStatus` — are closed (`3f7e861`), so a generated
      strict client no longer rejects live responses on day one.
- [ ] Move RLS-19 extraction, OSM mapping and the standards descriptor into the Go WASM kernel so
      `browser-backend.ts` shrinks to run bookkeeping + storage and `BROWSER_STANDARDS` comes from
      WASM; then move the kernel off the main thread — `backend/cmd/wasm/main.go` calls
      `road.ComputeReceiverOutputs` synchronously inside the Promise executor and there is no
      `Worker` anywhere in `src/`, so a grid run freezes the UI. While there: store one IndexedDB
      record per run instead of one document — `persist` in `browser-backend.ts` structured-clones
      every stored run (receiver tables, CSV, export HTML) on each write. The raster bytes are
      already out of that document, under an `artifact-bytes:` key each
      (`browser-storage.ts`), because they were the one part large enough to make the re-clone
      matter; that is one key space for one payload, not the split this item asks for, and the
      eviction, cap and clear paths each have to forget those records by hand today.

### Order and gates

**`just fe-ci` needs the public internet, and fails misleadingly without it.**
`frontend/project.inlang/settings.json` loads both inlang plugins from
`cdn.jsdelivr.net`, and `compile:i18n` treats a failed plugin import as a warning: it then writes an
**empty** message catalogue and exits 0. Everything downstream reads as catastrophe — ~2 200 eslint
findings and ~325 test failures, all of them `m.<key>` resolution noise — with nothing pointing at
the cause. Vendoring the two plugins, or failing the compile on a plugin import error, would make
an offline or network-restricted checkout say what is actually wrong. Until then, the two
`PluginImportError` warnings above a green compile are the tell.

B before C; D and E can run in parallel with C once B is green. The coverage floor is now live
(73.6% of statements, floors in `frontend/vitest.config.ts`, ledger in `docs/testing/coverage.md`),
so every refit from here is measured — advisory, so it reports rather than blocks. Each phase ends
with `just fe-ci` and `just fe-e2e` green, which includes the axe baseline on every route in `de`
and `en`.

## Priority 9 — Documentation truth

`AGENTS.md` and `README.md` are rewritten and every structural claim in them was checked against
the repository (`1257365`). `AGENTS.md` now defers all status to this file, carries a 29-row
package table and all ten CLI commands, and describes the standards modules by evidence tier rather
than as peers. The "all linters enabled" claim is gone from `AGENTS.md` and from
`docs/policies/formatting.md`, which also carried it — `README.md` never did.

- [ ] Promote the security scanners that only exist in a developer's `.trunk/`. That directory is
      gitignored and was never tracked, so it is one machine's tooling, not a second lint stack in
      the repository — "drop trunk" would change nothing here. What it does hold is `osv-scanner`,
      `trivy`, `trufflehog`, `checkov`, `gokart` and `actionlint`: exactly the coverage CI lacks,
      and the frontend's npm dependencies are scanned by nothing at all. Wire them into CI, as
      their own workflow, so the coverage belongs to the repository.

---

# Gate 3 — Feature roadmap

Everything below was already on the roadmap and remains open. It is deliberately behind Gates 1
and 2: a nicer Gutachten template does not help if the level in it is 23 dB low.

## Priority 10 — ISO 9613-2 geometry extensions

- [ ] Extract the shared barrier-intersection/ray-geometry logic from RLS-19 into a common package
      and wire automatic barrier detection for ISO 9613-2 diffraction inputs. _(Overlaps P7's
      acoustics-core extraction — do them together.)_ What is shared is the polyline/ray primitive.
      Schall 03's obstacle-grouping rule is **not** shareable: only Schall 03 has lateral
      diffraction, so only it needs to know which vertex a path may round. Lift the primitive, leave
      the rule with its caller.
- [ ] Add reflections via image sources for enclosed industrial-yard cases, once building geometry
      is readily available from the SoundPLAN import path.
- [ ] Add line and area source subdivision for extended industrial sources (conveyor belts,
      cooling towers, facades).
- [ ] Add spatial ground zones so per-region G values come from polygon geometry instead of a
      single global ground factor.
- [x] **The ISO 9613-1 α model replaced the nearest-row Table 2 lookup.** `AlphaForBand` evaluates
      the analytical model — classical absorption plus O₂ and N₂ relaxation — at every condition.
      **This entry understated the defect by describing the table's span rather than the resulting
      error.** The `dt/10, dh/50` weighting weighted temperature five times more heavily than
      humidity per unit, so 5 °C / 30 % RH selected the 10 °C / **70 %** row: 32.8 dB/km at 4 kHz
      where the formula gives 83.0, which is 25 dB over 500 m against the ±1 to ±3 dB clause 9
      claims for the whole method. Whole regions of input space were indistinguishable — 5 °C/30 %
      returned exactly what 10 °C/70 % did. Three constraints are live.
      **Table 2 is now a test oracle, not a rechenweg**, and that is what made the change safe to
      make without the ISO text: the table is a tabulation of the same formula, so it validates it.
      Agreement is ≤ 0.05 dB/km through 1 kHz and ≤ 1.4 % at 2–8 kHz, which is the table's own
      rounding. Do not loosen those tolerances to make a band pass — a disagreement is a
      transcription error in the formula.
      **The coefficient is evaluated at the reference pressure** (101,325 kPa); site pressure is not
      parameterisable, and that is the residual limitation that replaced the old one in the
      declaration.
      **`air_temperature_c` is bounded to [−60, 60] °C.** It previously accepted −273, where the
      formula divides by an absolute temperature at or below zero.

## Priority 11 — 16. BImSchV scope completion

- [ ] Define the exact 16. BImSchV scope Aconiq claims to support.
- [ ] Clarify which sections and annexes are covered and which are intentionally excluded.
- [ ] Decide whether workflow coverage should expand beyond the current explicit-receiver model to
      support the reporting and onboarding scenarios below.

## Priority 12 — DACH reporting and report verification

Goal: move from generic report/export capability to authority-facing German report packages that
are deterministic, reviewable, and CI-checked.

- [ ] Decide whether a DOCX path is required or whether Typst/PDF remains the only target.
- [ ] Define DACH report template requirements for TA Lärm Gutachten, 16. BImSchV Gutachten, and
      generic Schallimmissionsprognose.
- [ ] Implement Typst templates:
  - [ ] Cover page, table of contents, and the standard sections — Aufgabenstellung, Grundlagen,
        Beurteilungsgrundlagen, Berechnungsverfahren, Ergebnisse, Beurteilung.
  - [ ] Embedded result tables, maps, and contour plots.
  - [ ] Provenance section with standard version, data-pack version, evidence tier (P4) and input
        hashes.
- [ ] German-language assessment text generation: threshold comparison tables with pass/fail per
      receiver and area; Auflagen / Nebenbestimmungen suggestion blocks where appropriate.
- [ ] Add PDF golden/snapshot checks in CI, including metadata validation and selected page
      text/image probes. Note `aconiq export --pdf` shells out to an external `typst` binary
      (`report/reporting/report_typst.go:49-63`) — an undeclared runtime dependency for an
      "offline-first" product, and the reason no real PDF is produced in any test today.
- [ ] Add end-to-end report/export checks for Schall 03 and the common report/export paths.
- [ ] Embed report templates with `//go:embed` and `template.Must` at package level;
      `report/reporting/report.go:825-1077` holds ~250 lines of Markdown/HTML in string constants
      and re-parses them on every call.
- [ ] Define a template/versioning policy for backward-compatible report styles.
- [ ] Survey published Gutachten to pin down the minimum structure expected in practice.

## Priority 13 — SoundPLAN import and cross-validation

Parsers and the comparison harness are in place; the remaining work is model mapping and turning
the comparison into evidence (the assertion itself is Priority 3).

- [ ] Convert SoundPLAN rail geometry into Aconiq `TrackSegment` and `TrainOperation` structures.
      **This is now the blocker for Priority 3.** Since Priority 2, `aconiq run` reaches the
      normative chain whenever the model carries `schall03_operations` — but the SoundPLAN import
      writes only the preview `rail_*` properties, so `compare` opts into the preview engine and
      the 25 dB delta it reports says nothing about the normative code.
      Two things bound who can do this work, and they are the reason it keeps being deferred.
      **It cannot be validated without the licensed fixture.** `interoperability/` is gitignored and
      `repo-hygiene.yml` refuses to track it, so a checkout that has not been handed the project —
      or `ACONIQ_SOUNDPLAN_FIXTURES` pointing at one — makes `qa/fixtures.SoundPLANProjectDir` skip
      every test that would measure the mapping. The work is writable blind and testable against
      synthetic `RailTrack`/`TrainType` values, but the number it exists to move is unobservable.
      **And `FzComposition` is an editorial decision, not a conversion.** A 1990 `TS03` row carries
      one A-weighted base level and no Fz decomposition, so the 19 `Zugarten` of `beiblatt1.go` have
      to be reached through a declared lookup table someone is willing to defend against the norm.
      `railops.go`'s `classifyTrainClass`/`classifyTractionType` ordinal switch produces only the
      three-value preview vocabulary and is not a starting point for it.
      What the import already carries and does not yet use: `soundplan_train_names`,
      `soundplan_day_train_count`, `soundplan_night_train_count`, `soundplan_track_vmax_kph` and
      `soundplan_assessment_*_hours` are emitted per segment today. `StreckeMaxKPH` is available and
      reliable (`RailEmission.TrackV`); `Fahrbahn`, `Surface` and `BridgeType` arrive only as dB
      surcharges with no categorical 2014 counterpart, and all three reference enums are the zero
      value by design, so defaulting them is the safe choice rather than a gap.
- [ ] Map SoundPLAN track parameters and train types to Aconiq emission fields and Fz categories.
- [ ] Convert SoundPLAN buildings, barriers, terrain, receivers, and calculation areas into the
      internal model.
- [ ] Determine SoundPLAN project CRS and route it through the CRS pipeline. The transform accuracy
      that used to block this is settled (Priority 1.6), but read its live constraints first — in
      particular, a DHDN Gauss-Krüger project still sits ~1.8 m from PROJ in the **datum**, so a
      SoundPLAN comparison in 31466-31469 must not attribute that offset to the geometry.
- [ ] Fix the top-edge boundary rule in the heuristic raster alignment.
      `heuristicRasterRowCenters` (`app/cli/compare_raster.go:859`) places row 0 at exactly
      `y == maxY` whenever the row grid fills the CalcArea bounding box
      (`(rowCount-1)*resolution == maxY-minY`), and `calcAreaHorizontalSpan:928` uses a half-open
      scanline rule (`y < minY || y >= maxY`) that rejects every edge at that y. Row 0 therefore
      always falls back to the bounding-box span and emits a warning. Harmless for rectangles —
      the bbox span is the true span — but wrong for non-convex or non-rectangular CalcAreas, and
      the trigger condition is exact float equality, so real data hits it unpredictably. Either
      make the top boundary inclusive at the polygon's global `maxY` or nudge row centres
      epsilon-inward. Current behaviour is pinned by
      `TestBuildHeuristicRasterReceiversRowOnTopEdgeFallsBack`, which must be updated with the fix.
- [ ] Extend raster comparison from decoded values to fully validated spatial alignment —
      direction/anchor ambiguity remains for some GM variants.
- [ ] Generate a cross-validation report artifact with tables, deviation distribution, map overlay,
      and provenance.
- [ ] Add unit tests for all parsers.
- [ ] Research: how stable is the binary `.geo` format across SoundPLAN versions? Are there XML /
      ASCII export options easier to parse? Can SoundPLAN export via CadnaA-compatible exchange?
      Confirm and document the legal interoperability position for parsing the proprietary format.

## Priority 14 — QA hardening and conformance packaging

- [ ] **`ACONIQ_STRICT_ACCEPTANCE=1` fails a test that asserts the non-strict behaviour.**
      `go test ./internal/qa/...` under that flag fails `TestFullySkippedSuiteIsNotReportedAsPassed`
      with `expected status="skipped", got "failed"`. Strict mode is precisely what promotes a
      fully-skipped suite to `failed`, and the test asserts `skipped` unconditionally, so the two
      contradict each other by construction. Pre-existing on `main` and invisible to the ordinary
      gate, but it **blocks step 3 of the release checklist** in `docs/policies/releases.md`, which
      exists to prove the acceptance suites produced evidence rather than skipping. Fix the test to
      read the mode rather than the runner.

`just update-golden` is trustworthy again, and the entry that stood here named the wrong test.
The bullet blamed `TestCISafeSuiteExecutesTasks` and `TestRunCISafeSuiteProducesPassingReport`;
both are **readers**. The writer was a third test, `TestUpdateCISafeExpectedSnapshots`, which
called `t.Parallel()` _before_ its `golden.UpdateEnabled()` skip — so under `UPDATE_GOLDEN` Go
deferred it into the same parallel batch as the readers and rewrote all 38 CI-safe snapshots
while they were being decoded. It writes through the package's own `writeJSONFile`, never through
`golden.AssertBytesSnapshot`, so no amount of locking in the helper could have fixed it. Dropping
the `t.Parallel()` puts the rewrite ahead of the whole batch, which is how the sibling
`qa/acceptance/schall03` updater was already written. 5 failures in 6 before; 10 clean runs after.

Three things it leaves live:

- **There are four readers, not one** — the three that make a full `Run(ModeCISafe)` pass plus
  `TestParkingFixtureRelationsHoldByArithmetic`, which reads four goldens directly. `t.TempDir()`
  in the readers is only the report output directory; fixtures always come from the checked-in
  `testdata/ci_safe/` via `packageDir()`.
- **A second package had the same defect** and no entry here: `qa/acceptance`'s
  `TestAcceptanceFixtures` writes while `TestISO9613ToleranceCompliance` reads. Far rarer — it
  never surfaced in 60 whole-package update runs, and took `-count=300` over the two tests to
  produce six mid-write reads, because only three iso9613 goldens exist. There the writer _is_ the
  fixture assertion, so the reader skips under `UpdateEnabled()` instead, the way
  `TestCatalogProvidesDeterministicFixtures` already guarded itself.
- **`UPDATE_GOLDEN=true` used to regenerate half the tree.** `qa/acceptance/schall03` compared
  `os.Getenv("UPDATE_GOLDEN") != "1"` rather than calling `golden.UpdateEnabled()`, which accepts
  five spellings. Anything reading that variable goes through the helper.

- [ ] **The backend coverage guard's parser cross-check compares two different populations.**
      `scripts/coverage-check.sh` asserts that `go tool cover -func`'s total and the raw profile sum
      agree to 0.05 pp, on the stated premise that "if the two ever disagree, one of them is parsing
      the profile wrongly and neither number can be trusted". **That premise is wrong**, and the
      check fires as a result: it reported "the backend coverage measurement failed its sanity
      checks" on this branch while the coverage was fine.
      Diagnosed rather than guessed. `go tool cover -func` reports **functions**, so it attributes
      nothing from a file holding only package-level declarations —
      `internal/app/cli/run_modules_table.go` has zero top-level `func`s, being the `runModuleTable`
      var of function literals. The profile carries 233 files, `-func` 232. The two numbers are
      therefore both correct over different populations, and the gap is the dropped file's weight:
      0.0048 pp on `main` (78.9048 exact vs 78.9 printed), 0.0638 pp here (79.1638 vs 79.1). `main`
      passes by luck, not by construction, and any change to overall coverage can move an unrelated
      branch across the 0.05 threshold.
      The fix is to compare like with like — evaluate the exact ratio over only the files `-func`
      attributes — not to widen the tolerance, which would keep a check whose stated meaning is
      false. The headline percentage should stay the full-profile one. `go-coverage` is advisory, so
      nothing is blocked meanwhile; what is damaged is the credibility of a warning that says the
      measurement cannot be trusted when it can.
- [ ] Expand `internal/qa/` with loaders for standard test tasks, result comparison with tolerances
      and outlier reports, and a snapshot exporter for debugging.
- [ ] Expand fuzz/property tests: geometry robustness, numeric monotonicity where applicable.
- [ ] Add numeric drift tracking across commits.
- [ ] Add a repro-bundle export capturing run, inputs, standard, and profile in one package.
- [ ] Define the conformance package structure per module: supported scope and sub-scope;
      tolerance rules and comparison methodology; reference cases with provenance, source version
      and licence status; known deviations with rationale; machine-readable conformance JSON.
- [ ] Publish conformance packages for RLS-19 (leveraging TEST-20), Schall 03, and ISO 9613-2.
      The CNOSSOS-family scope statement is published
      (`docs/conformance/cnossos-umfangserklaerung.md`, P4); what is missing is the machine-readable
      conformance JSON and the CI gating alongside it.
- [ ] Automate conformance-report generation in CI with per-module pass/fail gating where practical.

## Priority 15 — Performance and scaling

- [ ] Optimize the tiled compute pipeline: spatial index tuning and candidate pruning. Note
      `engine/runner.go:161-169` currently builds a spatial index and **discards it**
      (`_, err := buildSourceIndex(cfg)`), and `computeOrLoadChunk` iterates all sources per
      receiver — the `prepare` progress stage is theatre.
- [ ] Throttle `writeRunState`: `runner.go:411` does a `MarshalIndent` + `MkdirAll` + `WriteFile`
      per completed chunk with the error discarded — ~7 800 synchronous writes on a 1 M-receiver
      raster.
- [ ] Compare numeric drift across worker and topology variants inside the benchmark flow.
- [ ] Optional, non-normative: `algo-fft` / `algo-dsp` post-processing pipelines; `algo-pde` for
      research-only wave and low-frequency experiments; WebAssembly delivery for interactive demos.

## Priority 16 — Examples and DACH onboarding

- [ ] Synthetic, licence-safe example projects: RLS-19 road corridor in a 16. BImSchV context;
      Schall 03 rail section; ISO 9613-2 industrial point source in a TA Lärm context; combined
      road-plus-rail.
- [ ] Each example ships input data, run config, expected outputs, and a step-by-step README.
- [ ] German-language getting-started guide.
- [ ] CI jobs that keep example projects green across releases.

## Priority 17 — Community

- [ ] Keep a public roadmap — this file, or a GitHub project board.
- [ ] Documentation site: getting started (EN + DE), project format specification, standards-module
      overview with status and conformance boundaries, QA/acceptance process and tolerances.
- [ ] German-language community presence — blog post, conference talk, or Fachzeitschrift article.

---

## Deferred and optional tracks

### Deferred frontend features

Distinct from Priority 8, which is correctness. These are genuinely optional.

- [ ] WebSocket progress streaming (SSE already works).
- [ ] TypeScript client-generation pipeline for frontend API types.
- [ ] Headless E2E smoke flow on the API side: import → validate → run → export.
- [ ] Box select and multi-select on the map.
- [ ] Contour overlays and labels on the result map.
- [ ] Contribution breakdown per receiver or selected result.
- [ ] Run-to-run diff layer; scenario change-set summary for model and parameter differences.
- [ ] Performance guardrails for large feature counts — clustering or tile fallback.
- [ ] Building-footprint import pipelines beyond GeoJSON.
- [ ] Per-source acoustics: UI coverage for editing/clearing/restoring overrides; surface overrides
      and inferred review flags in popups or run-setup summaries; decide whether further
      OSM-derived defaults are deterministic enough to enable; define follow-on source-editing
      scopes for other standards.

### Tiling and PMTiles

- [ ] Evaluate vector tiles for model and result delivery; evaluate an end-to-end PMTiles pipeline;
      define storage and size budgets.

### Desktop packaging

- [ ] Make the API runnable in-process with no external port requirement.
- [ ] Embed frontend assets into the Go binary.
- [ ] Define build targets for `web` versus `wails`; add smoke tests for desktop builds.
- [ ] Re-check Wails v3 maturity and define fallback options.

### Project format v2

- [ ] Map the data model to PostGIS with geometry storage and indexes.
- [ ] Store artifacts in object storage for rasters, tiles, and reports.
- [ ] Add minimal auth/users only if genuinely required.
- [ ] Add migration tooling from v1 projects to v2.

---

## Research backlog

### Standards and validation data

- [ ] **RLS-19: obtain the FGSV text and Korrekturblatt 2/2020.** Blocks verification of seven
      coefficient sets (see Priority 1.9) that currently have no normative cross-check.
- [ ] CNOSSOS Road/Rail/Industry/Aircraft: obtain the JRC reference report coefficient sets and
      worked examples, and decide whether real implementations are on the roadmap at all (P4).
- [ ] BUB/BUF/BEB: obtain current documents and annexes; define exact input requirements per module.
- [ ] Schall 03: clarify redistribution rights for the normative tables. The project's own note
      holds that Schall 03 coefficients are an amtliches Werk and can be embedded directly — if so,
      the out-of-repo data-pack mechanism (P2) may be unnecessary.
- [ ] TA Lärm: survey published Gutachten for structural conventions and assessment patterns.
- [ ] 16. BImSchV: clarify combined assessment rules for road plus rail.

### GIS, CRS, and format research

- [ ] GeoTIFF export: settle the long-term dependency strategy for writing, given a pure-Go reader.
- [ ] Contour generation: define the preferred algorithm and quality requirements.
- [ ] Decide whether NTv2 (BeTA2007) datum shifts are needed. `geo/crs.go` uses Helmert
      (`wgs84.DHDN2001GK`), which gives ~1 m on DHDN→ETRS89 — irrelevant for noise, relevant if
      cadastral interoperability is ever claimed.

### Determinism and tolerances

- [ ] Standardize numeric tolerances per standard and test suite — and stop presenting 1e-6 dB
      determinism checks as conformance tolerances.
- [ ] Define the stable summation strategy and document exactly where it must apply, or amend
      `docs/policies/determinism.md` §3 to match what is implemented (see P1.3).

### UX and workflow questions

- [ ] Define DTO-generation strategy and backward-compatibility policy.
- [ ] Define which exports are must-have versus deferred — GeoTIFF, CSV, PNG, report artifacts.
- [ ] Define map-layer performance thresholds and tile-fallback triggers.
- [ ] Define the accessibility baseline for map-heavy interactions.
