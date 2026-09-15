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
- [x] **Repository hygiene** (`91ffcc1`): `backend/wasm` (3.28 MB and orphaned — the real build
      target is `frontend/public/aconiq.wasm`), the committed `tsbuildinfo`, the empty
      `backend/cmd/noise/` and root `.codex`, plus gitignores for `.trunk/`, `playwright-report/`
      and `test-results/`.
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

### Open

- [ ] **The auto receiver grid applies metres to a geographic CRS.**
      `buildReceiversFromPoints` (`run_receivers.go:39-53`) computes
      `bbox.MinX - paddingM` and hands `resolutionM` to `GridReceiverSet` in the project's own
      units, and nothing on that path projects first — `calcAreaExtent` and `resolveGridReceivers`
      (`run_input.go:134-183`) read the model's coordinates as stored. So in a project whose CRS is
      geographic a 20 m padding is subtracted as 20 **degrees** and a 100 m resolution spaces
      receivers 100 degrees apart. Measured, not reasoned: a model spanning 9.985–10.015 E,
      53.545–53.565 N put its single receiver at `-10.015, 33.545`, and the levels computed there
      were reported as if they were the site's.
      This is not a corner case — `aconiq init` defaults the project CRS to **EPSG:4326** and
      `--receiver-mode` defaults to `auto-grid`, so it is the out-of-the-box path. Reproduced
      byte-identically on `main`, so it predates Phase C and is not a regression.
      Either project the extent into a metric CRS before padding and gridding, or refuse a
      geographic project CRS on the auto-grid path — silently computing at the wrong place is the
      one option that is not available. Whichever way it goes, the browser kernel's
      `buildReceiverGrid` pads the same bbox the same way, so `browser-parity.test.ts` pins the two
      together and both sides move at once.
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
- [ ] **Buildings do not shield.** A `building` feature can opt into acting as a reflector
      (`schall03_reflecting_wall`), but is never turned into a `BarrierSegment`, so a receiver
      behind a building is computed as if the building were absent. Reflection alone is the wrong
      half to ship on by default, which is why the reflector role is opt-in — but the shielding
      half is what a real project needs. Entangled with P10's shared barrier-geometry extraction.
- [ ] **Browser mode and the CLI order the receiver table differently.** Found by the parity
      comparison on its first run, and left standing rather than papered over: in the `custom`
      receiver mode `extractExplicitReceivers` keeps receivers in model order while
      `browser-backend.ts` sorts them by id, so for the same model the two targets emit the
      receiver table's rows in different orders — and `output_hash` is computed over that order.
      The levels agree receiver for receiver, so no assessed level is affected, but picking a
      winner changes the hash of every browser run already made. That is a decision, not a fix.
      `browser-parity.test.ts` pins the current behaviour of both sides meanwhile.
- [ ] **Terrain, explicit reflectors and per-direction sources are still CLI-only.** Browser mode
      builds road sources, barriers, buildings and Parkplätze, and `wasm/types.ts` now mirrors the
      terrain and reflector types because the kernel accepts them — but no model can express them
      in the browser, so the parity fixtures cannot cover them.
- [ ] **No terrain on the Schall 03 propagation path.** `elevation_m` is per segment and h_m falls
      back to the flat-ground special case (deviation 4 in the conformance declaration), even when
      the project carries a DTM the RLS-19 path already reads.

## Priority 3 — Establish real validation evidence

**The comparison now asserts, and it fails.** `compare_test.go` ran the whole
`init → import --from-soundplan → compare` pipeline against the reference project and never read a
single delta field. It does now (`ac33895`), and against the real fixture Aconiq reads
systematically high on **every one** of the 54 matched receivers:

| indicator | mean abs | p95 abs | max abs | exceeding |
| --------- | -------- | ------- | ------- | --------- |
| LrDay     | 24.052   | 37.859  | 39.745  | 54 / 54   |
| LrNight   | 22.630   | 36.098  | 37.965  | 54 / 54   |

Down from 25.110 / 23.329: removing the `10 lg(n + 1)` flow shift (Priority 1.2) accounts for the
whole of that, because the shift sat on the data-pack path the CLI runs. A 1 dB improvement on a
25 dB error is a rounding correction, not progress.

This is the single most important number in this file. Note the shape of it: ~25 dB high, on a
Schall 03 rail project, measured against `BuiltinDataPack()`'s invented spectra. Priority 2 has
since wired the CLI to the normative Anlage-2 chain, but **the comparison still does not reach
it**: the SoundPLAN import produces only the preview `rail_*` vocabulary, so `compare` now opts
into the preview engine explicitly. **The delta above is a preview-chain number and nothing should
be concluded from it. Priority 13's `TrackSegment` mapping is what makes it measurable.**

- [ ] **Fix receiver matching before tightening any tolerance.** All 54 matches fell back to the
      `ordinal` strategy (`distance_m = -1`): coordinate matching at 0.5 m tolerance matched
      _nothing_, so pairs are matched by list position rather than geometry, and two Aconiq
      receivers were paired to SoundPLAN records both labelled `Hauptstraße 4`. 23 of the 77 Aconiq
      receivers are unmatched and 0 SoundPLAN ones are. The comparison is not merely inaccurate —
      it is comparing arbitrary pairs. Suspect the CRS pipeline (Priority 13) first.
- [ ] Then tighten the thresholds. What is in the test today is a deliberately loose regression
      bound (mean*abs ≤ 30 dB, max_abs ≤ 45 dB) labelled in the source as \_not* a tolerance, sitting
      alongside exact self-consistency assertions that do hold the compare command to a real
      standard. Once matching and Priority 2 are fixed, this must come down by a large factor.
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

- [ ] **The frontend needs the tier to reach WASM mode honestly.** `browser-backend.ts` hardcodes a
      single `rls19-road` descriptor and now hardcodes its tier alongside it. That is correct today
      and is a second place the tier is declared, which is exactly the duplication the descriptor
      field exists to prevent. It should come from the WASM kernel.
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
- [ ] **Write the data-handling policy.** The CI guard that refuses any tracked
      `interoperability/` path is in place (`just check-no-third-party-data`, mirrored by
      `.github/workflows/repo-hygiene.yml`), but the policy the guard enforces is still unwritten:
      what may be stored there, who may hold it, and what happens if it leaks.
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

- [ ] **Finish the shared acoustics core.** `internal/acoustics` exists and owns the END indicator
      model — the day/evening/night types, the directive's Lden weighting and the bundle the six
      modules reporting that set publish. What it does not own yet is the summation: `energySumDB`
      exists in **9 copies with 3 different semantics** — `rls19/road/emission.go:147` skips
      `level <= -900`, `cnossos/road/emission.go:326` does not, `schall03/model.go:129` uses `-Inf`
      and returns NaN on +Inf. Two incompatible silence sentinels (`-999.0` vs `-Inf`) flow into the
      same `results.ReceiverTable`. One `EnergySum`, one sentinel, one `Level` type.
      Unlike the indicator lift, **this one moves numbers**, so it needs its own golden review; the
      13 digest goldens are the oracle. It also lands the compensated-summation item from P1.3.
- [ ] **Unify the four Schall 03 normative propagation kernels** —
      `compute.go:61`, `compute.go:246`, `reflection.go:223`, `reflection.go:281` are near-identical
      ~50-line implementations of Gl. 13–16, and the explanatory Gl. comments survive only in the
      first. A correction to a normative equation currently has to be applied four times.
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
      tooltip is invisible. That failure is still live in `undo-redo-bar.tsx:33-47`, whose two
      tooltips are silent in exactly the state they describe — **left open, listed under Phase D**.
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
      **`map/color-ramp.ts` has zero importers and 0 % coverage**, and was deliberately kept:
      "Results on the map" below names `NOISE_LEVEL_RAMP`. Do not delete it as dead code in the
      meantime.
- [x] **Split run/results/export.** `pages/run.tsx` went 1,335 lines → 162: it is the route module
      now, and the parts live in `src/run/`. **Not `pages/run/` as this file asked for** — the repo
      already answers this question the other way, and consistently: `pages/map.tsx` is a thin route
      module composing a dozen files from `src/map/`, and no nested page directory exists anywhere.
      Keeping `pages/run.tsx` as the default export also left `routes.tsx`, `routes.test.tsx` and
      `run.test.tsx`'s import untouched, which made the move commit reviewable as a move: zero test
      edits, 827 tests unchanged. `parseTimeline` sits in its own module because a file exporting
      both a component and a function loses fast refresh.
      `useRunSetupSelection` carries the cascade. **The parameter seeding stays render-phase**;
      an effect paints the fields empty for one commit on the way, which `run.test.tsx`'s
      `MutationObserver` pins as `["999", "25"]` rather than `["999", "", "25"]`. The experimental
      acknowledgement stayed outside the hook — it is an evidence-tier decision, not part of
      choosing a profile. `useRunFromRoute` replaced the duplicated URL→run resolution in
      `results.tsx` and `export.tsx`, with eligibility as a parameter because the two genuinely
      differ; the run page's own selection stayed inline, since it falls back to the first visible
      run and sharing a name with a rule that refuses exactly that would make the name lie.
      `STANDARD_LABELS` was keyed by `upstream_mapping_standard` — a **parameter** name — so every
      lookup had always missed; both message keys are gone. The eight raw `standard_id` sites now
      read a name, and **the label carries its own evidence limit**: `EvidenceTierBadge` sits
      beside it in only two of five places, and `RunSummary` has no tier to give the rest one, so a
      scaffold reads "BUB Straße (Gerüst)". Parameters are grouped by name prefix and labelled with
      the API's `unit`; the RLS-19 vehicle classes get the terminology the catalogue already
      carries, and **everything else keeps the backend's own name, humanised, in English**.
      Composing German across all 98 parameters would have invented `Gleisrauheitsklasse` for
      published norms — Phase E owns that. The raw name stays on screen in a `code`, because it is
      what `--param` takes.
      The run gate goes through `useModelValidation`, on **errors only** — a model with one RLS-19
      review warning must stay runnable — and **an empty store is not an empty project**: when
      `runsAgainstSavedModel` and hydration errored, the dialog says the model could not be read
      and offers the retry. The dialog body became a child, so its queries and this validation run
      while it is open rather than on every render of `/run`.
      `ui/confirm-dialog.tsx` has call sites at last: discarding a draft, deleting a feature or a
      receiver, and import replacing the model. The two map deletes are undoable and the
      confirmation says so, which is the cheapest way to make that true when the undo bar is at the
      other end of the workspace. `src/map/feature-editor.test.tsx` exists now.
      `Backend.deleteRun` lands in both modes with `exportsOutliveRunDelete`, because **which
      sentence is true has to be said before the user agrees** and `retained_paths` arrives after.
      Delete is offered only on a finished run. `ConfirmDialog` forwards Radix's
      `onCloseAutoFocus`: confirming unmounts the button focus would return to, and no axe rule
      covers landing on `<body>`.
      **`PERSISTED_STATE_VERSION` stayed at 1, against what this file called mandatory.** The bump
      is the wrong tool: `decodePersisted` throws on an unknown version and `loadState` swallows
      that into `initialState()`, warning that "the next completed run replaces them" — so a bump
      buys nothing upward, since `decodeState` already reads field by field, and downward it makes
      an older build discard twenty runs. `runHighWaterMark` is additive and raised in `setRun`;
      its fallback, `max(stored ids)`, is exact rather than approximate for every document written
      before it, because nothing could delete a run then.
- [ ] **Results page**: virtualised receiver table (`@tanstack/react-virtual`); the sortable
      header buttons already carry `aria-sort` and `scope="col"` since Phase B; hoist
      `SortIcon` (nested at `results.tsx:139-147`, so it remounts the header on every keystroke)
      along with `level` and `columnLabel`; `RunColumn` is already at module scope and only needs
      moving. The CSV half of this bullet is done — one builder in `model/receiver-csv.ts`, Go's
      spelling, pinned by `testdata/csv-parity/` — and the instruction it used to carry was wrong:
      freezing the stored artifact's bytes was impossible, because the same escaping defect was in
      both writers. Also replace
      `summaryCards`' `Math.min(...vals)`, which throws past ~100k arguments — exactly what
      virtualising the table admits,
      `results.test.tsx` written alongside.
- [ ] **Import page**: split the 400-line component into `FileImport`, `OsmImport`, `PreviewStep`;
      ask replace-vs-merge before `loadFeatures`; link preview errors to features. UI import drops
      `kind: "receiver"` features (`normalize.ts:12` lists only source/building/barrier) and
      `loadFeatures` clears placed receivers, so "import, then Save to project" replaces the project
      model without the receivers `aconiq import` had put there — import receivers into
      `receivers`.
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
- [ ] **The SoundPLAN calculation area now exists twice in a project, and the two can diverge.**
      `compare-raster` reads it from `soundplan-import-report.json` — verbatim, 3D, possibly open —
      because `calcAreaHorizontalSpan` intersects each raster row against the real outline and the
      model feature is lossy for that. So a user editing the model's `calc-area` does not move what
      `compare-raster` uses. Reconcile, or state which one wins, before P13 turns the comparison
      into evidence.

### Phase D — Map workspace

- [ ] **The undo/redo tooltips are dead.** `undo-redo-bar.tsx:33-47` puts a Radix tooltip on a
      `disabled` Button, and a disabled button takes no focus and fires no pointer events — so
      neither tooltip opens, by mouse or by keyboard, in exactly the state it describes ("nothing
      to undo"). Nothing notices because the label is duplicated in `aria-label` and no axe rule
      covers it. `ui/mode-gate.tsx` has the fix and the test shape: `aria-disabled` plus a
      swallowed click, asserted with a real `hover`/`tab` and `findByRole("tooltip")` — an
      attribute check passes while the tooltip is invisible.
- [ ] **Selection and chrome**: `setFeatureState` on click and `feature-state` paint expressions
      (nothing on the map shows which feature is being edited); Esc cancels drawing, Del deletes the
      edited feature; delete `FeaturePopup` (second click surface and an HTML-injection vector);
      move the coordinate display off the undo bar's corner; dock the editor as a `MapPanel` beside
      the map instead of over the layer control.
- [ ] **Geometry editing**: on click in select mode `draw.addFeatures([feature])`, listen for
      change/deselect, commit as one `updateFeature` command; add coalescing (`mergeWith`) to
      `CommandStack` first so drags do not become one step per mousemove.
- [ ] **Editor completeness**: derive `sourceType` from geometry (the select at
      `feature-editor.tsx:220-242` can contradict it); add Parkplatz (`rls19_parking_*`) and rail
      (`schall03_*`) field groups — `validate.ts` flags missing parking fields the UI cannot set;
      table-drive the 16 near-identical `PropertyNumberField` blocks; show the feature's own issues
      inline; `role="dialog"`, focus trap and Escape handling.
- [ ] **Results on the map**: `ResultLayers` (raster image source + contours), legend from
      `NOISE_LEVEL_RAMP`, `glyphs` in the style (`layers.ts:169` requests a font no style provides);
      row↔map highlight from the receiver table. Until it lands, hide the result toggles in
      `layer-control.tsx:74-76`.
- [ ] **CRS and basemap**: guard `fitBounds` (`model-layers.tsx:91` throws on EPSG:25832 input);
      proj4 with 25832/25833 on import and a UTM readout in the coordinate display; tile-error →
      `OFFLINE_STYLE` with a notice; basemap picker; tile URL in Connection settings
      (`basemap.ts:28` hardcodes `tile.openstreetmap.org`).
- [ ] **Keyboard path**: coordinate-entry form in `NewFeatureDialog` and a keyboard-navigable
      feature list, so the map is not mouse-only.

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

- [ ] `hooks.test.ts` against mocked `fetch`; a `browserBackend.startRun` test with a stubbed
      kernel compared to a backend golden; tests for `use-draw` and `model-layers` (the map-rebuild
      fix in `ad47eaa` still has no test — it needs a real WebGL context, and `map.test.tsx` stubs
      `MapView` out entirely). Coverage measurement itself is done, and it prices this bullet:
      `src/map` sits at **37.3%** over 1,765 statements, the largest single gap in the frontend.
      The floors are **not** in `fe-ci` as this bullet used to say — they live in
      `frontend/vitest.config.ts` and are applied by an advisory `frontend-coverage` job, because a
      coverage regression must not be able to fail a required check. See
      `docs/testing/coverage.md`.
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
      every stored run (receiver tables, CSV, export HTML) on each write.

### Order and gates

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
      acoustics-core extraction — do them together.)_
- [ ] Add reflections via image sources for enclosed industrial-yard cases, once building geometry
      is readily available from the SoundPLAN import path.
- [ ] Add line and area source subdivision for extended industrial sources (conveyor belts,
      cooling towers, facades).
- [ ] Add spatial ground zones so per-region G values come from polygon geometry instead of a
      single global ground factor.
- [ ] Implement the ISO 9613-1 analytical α model to replace nearest-row Table 2 lookup
      (`iso9613/atmospheric.go:27-46`), which snaps to one of 6 points using an undocumented
      `dt/10, dh/50` weighting. At 4 kHz the table spans 22.9–88.8 dB/km. The deviation _is_
      honestly disclosed at `docs/conformance/iso9613-konformitaetserklaerung.md:84` — implementing
      the ~20-line formula is cheaper than maintaining the caveat.

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
- [ ] Map SoundPLAN track parameters and train types to Aconiq emission fields and Fz categories.
- [ ] Convert SoundPLAN buildings, barriers, terrain, receivers, and calculation areas into the
      internal model.
- [ ] Determine SoundPLAN project CRS and route it through the CRS pipeline.
- [ ] Fix the top-edge boundary rule in the heuristic raster alignment.
      `heuristicRasterRowCenters` (`app/cli/compare_raster.go:581`) places row 0 at exactly
      `y == maxY` whenever the row grid fills the CalcArea bounding box
      (`(rowCount-1)*resolution == maxY-minY`), and `calcAreaHorizontalSpan:636` uses a half-open
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

- [ ] **`just update-golden` is flaky, and has been all along.** `internal/qa/acceptance/rls19_test20`
      runs `TestCISafeSuiteExecutesTasks` and `TestRunCISafeSuiteProducesPassingReport` in parallel
      against the same `testdata/ci_safe/*.golden.json` files, so under `UPDATE_GOLDEN=1` one test
      decodes a golden the other is mid-write and fails with `unexpected end of JSON input`. The
      file named differs every run. Reproduced 4 times in 6 on `main`; `-p 1` does not help, because
      the race is inside one package. The merge gate never sees it — `go test ./...` without the
      flag is stable — but the one command a maintainer runs before regenerating snapshots is not
      trustworthy, which is the wrong way round. Serialise the two tests or give them separate
      fixture directories.
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
