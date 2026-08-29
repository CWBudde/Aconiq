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

### Open

- [ ] **The debts a green `just lint` still hides.** 293 findings were converted from
      config-hidden to fixed code in `4b0566c`; 557 remain hidden. Audit in `docs/lint-triage.md`.
      Today's uncapped counts:

      - `goconst` **167** excluded. 39 are the OpenAPI keyword set and are permanent; the other
        128 are owned by the Priority 7 parameter-descriptor refactor (99) and typed response
        payloads (29). Named and bounded, but not fixed.
      - gosec **G304 97** (47 non-test) still hidden by the exclusion presets. See Priority 6 —
        the verdict was taken at 47 and has not been re-litigated at 97.
      - `wsl_v5` **196** (was 120) and `noinlineerr` **94** (was 41) still disabled; both are
        style opinions this project has declined in writing and the reasons still hold.
        `noinlineerr` grew partly because the `wrapcheck` work introduces exactly the
        `if err := f(); err != nil` form it objects to.
      - `gocyclo` **3** still disabled as redundant with `cyclop`.

      Every remaining exclusion names a rule the project argued about in writing, or a path and a
      string value. `.golangci.yml`'s disable list currently holds **19** linters — PLAN.md said 20
      and before that 21, which is why neither `AGENTS.md` nor `docs/policies/formatting.md` now
      quotes a number.

- [ ] **Decide whether to keep `govulncheck` blocking.** It is wired in as its own CI job and is
      **green as of this commit**: `golang.org/x/text` went v0.35.0 → v0.41.0 (clears
      `GO-2026-5970`, reachable via `reporting.renderTypstSource`) and `backend/go.mod` gained
      `toolchain go1.26.6`, which clears the seven reachable standard-library advisories
      (`GO-2026-6218` net/url, `GO-2026-6091` html/template, `GO-2026-6090` crypto/tls,
      `GO-2026-6089` net/http, `GO-2026-6088` encoding/xml, `GO-2026-5972` encoding/asn1,
      `GO-2026-5026` net/http). Before the toolchain line there was none at all, so `setup-go`
      with `go-version-file` would have built CI against a 1.25.x standard library — older, and
      more exposed, than any developer's local toolchain. Since stdlib advisories land on their
      own schedule, this job will go red on its own; decide whether that blocks merges or opens an
      issue.
      Note what adding that `toolchain` line actually did, since this item half-anticipated it:
      commands run inside `backend/` do re-exec under go1.26.6, but `setup-go` still installs
      1.25.0 from the `go` directive, and govulncheck — built by `go install`, which resolves in
      its own module — stayed on 1.25 and could no longer parse the very standard library it was
      meant to scan. That broke the job outright until the fix recorded above. It is now a
      required status check, so "goes red on its own" means "blocks every merge until someone
      acts", which is the sharper form of the decision this item asks for.
- [ ] **`just check-formatted` mutates the working tree.** It runs `treefmt --fail-on-change`,
      which formats in place _and then_ reports. So the CI "check" step rewrites files, and
      running it locally silently reformats unrelated work. Make it operate on a copy, or use a
      genuinely read-only check.
- [ ] **`treefmt` cannot be installed with `go install`.** Every version fails: the module zip
      contains a test fixture whose path has an emoji, which is not a valid module file path, and
      `proxy.golang.org` 404s for all `treefmt/v2` `.info`/`.zip` requests. The workflow now uses
      the release tarball. The old `go install` step was therefore already broken on a clean
      runner, independently of the module blocker.
- [ ] **CI installed only the Go formatters.** `treefmt --allow-missing-formatter` skips silently,
      so markdown, YAML, JSON and TypeScript were never format-checked in CI even when the step
      passed. `shfmt` and `prettier` are now installed too. Watch for a backlog surfacing.
- [ ] **Reconcile the three-way `golangci-lint` skew.** CI and the local toolbox are now both
      pinned to 2.12.2; `.trunk/trunk.yaml` still pins 2.11.4. Related to the "resolve the two
      competing lint stacks" item in Priority 9.

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

### 1.1 Schall 03: the substitute-speed extent, the only judgement call still open

The Eisenbahn half is closed. `resolveEffectiveSpeed` applied `max(v, 50)` unconditionally, which
Nr. 4.3 does not prescribe — it requires only the 70 km/h im Bereich von Personenbahnhöfen und
Haltepunkten. The floor now applies to Straßenbahn segments only, where Nr. 5.3.2 puts it, and the
70 km/h station rule no longer reaches Straßenbahn segments either. Two consequences fell out: the
double clamp had made the Nr. 5.3.2 "dauerhaft v ≤ 30 km/h" exception unreachable (`buildVehicleInputs`
raised the speed to 50 before `ComputeStreckeEmission` could test it against 50), and the
Fahrbahnart zero value is now Schwellengleis on both the Eisenbahn and the Straßenbahn side.

- [ ] **Decide whether to model the ± 25 m extent of the Nr. 5.3.2 substitution.** Nr. 5.3.2 scopes
      the 50 km/h substitute speed to Weichen, Kreuzungen and Haltestellen an Strecken (each plus
      25 m on either side); Aconiq applies it to the whole segment below 50 km/h unless
      `permanently_slow` is set. Partial substitution needs the caller to split the segment, so the
      question is whether Aconiq should split automatically from Weichen/Haltestellen geometry it
      does not currently carry. Recorded as an open deviation in
      `docs/conformance/schall03-konformitaetserklaerung.md`; Anmerkung 1 to Nr. 5.3.2 argues for
      the current whole-segment reading, so this is not obviously a defect.

### 1.2 Fixture format change from the Fahrbahnart renumbering

`FahrbahnartType` and `SFahrbahnartType` are renumbered so Schwellengleis — the Nr. 4.4 / Nr. 5.4
reference type, carrying no c1 correction — is the zero value. Previously the zero value was Feste
Fahrbahn and straßenbündiger Bahnkörper, so an omitted `fahrbahn` / `s_fahrbahn` silently added
+7/+3 dB Schiene and +1 dB Reflexion, respectively up to +8 dB at 1000 Hz. All nine CI-safe
scenarios were renumbered; `a1_full_chain.scenario.json`, the one fixture that set `0` where its
siblings set `-1`, was set to `1` so it keeps exercising Feste Fahrbahn, and its expected snapshot
is unchanged, which confirms the reading.

- [ ] **The wire format is unversioned.** `TrackSegment` JSON is read straight from scenario files
      with no schema version, so a file written against the old numbering is silently misread. Any
      project format carrying `fahrbahn` needs a migration entry, or the field needs to become a
      string enum. Nothing outside `internal/standards/schall03` and the acceptance fixtures reads
      it today, which is why the renumber was safe now and will not be later.

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

Six fixtures were added for the cases the suite could not see. Each was verified to reach the path
it claims: `TestLateralDiffractionCanDominate` and `TestThreeDiffractionEdgesAreSelected` pin the
two barrier geometries, and the b1 geometry was re-checked to confirm its lateral A_bar is capped
at 20 dB in every band, so the assertion is not vacuous.

| fixture                     | closes                                                    |
| --------------------------- | --------------------------------------------------------- |
| `b3_lateral_diffraction`    | lateral path per-band cheaper than the top path (Gl. 18)  |
| `b4_three_edge_barriers`    | three diffraction edges survive the rubber band (Bild 6)  |
| `e4_bruecke_feste_fahrbahn` | bridge combined with Feste Fahrbahn (Nr. 4.6 suppression) |
| `e3_langsame_strecke`       | 40 km/h Eisenbahn line — non-`v₀`, no substitute speed    |
| `s3_langsamfahrstelle`      | Nr. 5.3.2 `permanently_slow` exception, end to end        |
| `iso9613/point_cmet`        | non-zero `c0_met` across the 10(h_s + h_r) threshold      |

The ISO 9613-2 fixture places two sources (5 m and 30 m) over four receivers from 60 m to 1000 m,
so C_met is exactly zero at the near receiver, active for one source only at 150 m, and active for
both further out. `LpAeq_LT` and `LpAeq_DW` now differ in the golden, which they did not in any
previous fixture.

- [ ] The suite still has no fixture where an intermediate barrier is _reflective_
      (`BarrierSegment.Reflective`), so Gl. 20's D_refl is exercised only by unit tests.

### 1.5 RLS-19 items that could not be verified

The RLS-19 text is not in the repo, so these were **not** checked and are not asserted as bugs.

- [ ] Obtain the FGSV RLS-19 text and Korrekturblatt 2/2020, then verify: Tabelle 2 percentages;
      Tabelle 3 coefficients; Eq. 7a–7c gradient corrections (the Lkw `/10` vs Pkw `/100` divisors
      look asymmetric — up to 8–9 dB at 12 % grade); Eq. 9 `min(2h/w, 1.6)`; Eq. 15's constant 80;
      Tabelle 8 reflection losses 0.5/3.0/5.0 dB; Tabelle 6/7 parking values.

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

`AGENTS.md` claims standards are plug-in modules. `framework.StandardDescriptor` carries only
metadata — no compute contract of any kind. Dispatch is a **562-line `switch`** at
`run_pipeline.go:143`, carrying `//nolint:gocognit,gocyclo,cyclop,dupl,funlen,maintidx`.
Adding a standard means editing ~8 CLI files (`run_extract.go` 28 hardcoded standard IDs,
`run_options.go` 25, `run_persist.go` 20, `run_pipeline.go` 10, `compare.go` 9, …).
That is why `internal/app/cli` is 16 450 LOC — a third of the backend.

- [ ] **Extract a shared acoustics core** (`internal/acoustics`). `energySumDB` exists in **9
      copies with 3 different semantics**: `rls19/road/emission.go:147` skips `level <= -900`,
      `cnossos/road/emission.go:326` does not, `schall03/model.go:129` uses `-Inf` and returns NaN
      on +Inf. Two incompatible silence sentinels (`-999.0` vs `-Inf`) flow into the same
      `results.ReceiverTable`. One `EnergySum`, one sentinel, one `Level` type.
      This also lands the compensated-summation item from P1.3.
- [ ] **Unify the four Schall 03 normative propagation kernels** —
      `compute.go:61`, `compute.go:246`, `reflection.go:223`, `reflection.go:281` are near-identical
      ~50-line implementations of Gl. 13–16, and the explanatory Gl. comments survive only in the
      first. A correction to a normative equation currently has to be applied four times.
- [ ] **Define `framework.Module`** — `Descriptor()` / `BindInputs()` / `Compute(ctx, …)` — register
      implementations instead of bare descriptors, and delete the switch.
- [ ] **Move the run pipeline out of `app/cli`** into `internal/engine` (or `internal/app/run`) as
      `Run(ctx, store, req) (RunResult, error)`. Today `api/httpv1` reaches it by fork/exec'ing its
      own binary (`handler.go:408-478`, parsing exit code 2 back into a typed error) — fork/exec
      used as dependency inversion. Delete `newCLIProcessRunExecutor` once this lands.
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
- [ ] Collapse the mechanical duplication — ~4 300 non-test LOC, about 9 % of the backend:
  - [ ] `buf/aircraft` → alias package over `cnossos/aircraft`. `compute.go` and `emission.go`
        are **byte-identical**; `propagation.go` differs by one constant. `bub/rail` and
        `bub/industry` already demonstrate the correct 211-LOC alias pattern. **−1 050 LOC.**
  - [ ] Lift `ExportResultBundle` (12 copies), `PeriodLevels`/`ReceiverIndicators`/`ComputeLden`
        (8 copies of the END directive's defining formula), `ComputeReceiverOutputs` (12),
        `ProvenanceMetadata` (10) and `geometricDivergence` (8) into `framework`.
  - [ ] Replace the 11 `persist*RunOutputs` and 10 `hash*Outputs` clones with two generics.
  - [ ] Merge `extractCnossosAircraftSources` / `extractBUFAircraftSources`
        (`run_extract.go:1673,1913`, 239 LOC each, identical but for the type prefix). The
        `//nolint:dupl` claim that "the source/output types differ" is false:
        `run_options.go:1178-1184` converts between them with a direct type conversion because
        the structs are identical.
  - [ ] Consolidate 7 copies of `writeJSONFile`/`writeJSON`.
  - [ ] Remove the 12 illegitimate `//nolint:dupl` directives (of 20 total; the 8 in
        `schall03/beiblatt1.go` are genuine — those are coefficient tables).
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
- [ ] Split the god files: `run_extract.go` (3 087 lines), `run_options.go` (1 604),
      `run_persist.go` (1 147), `api/httpv1/handler.go` (1 137), `report/reporting/report.go`
      (1 082), `app/cli/export.go` (1 005). The first two already violate the project's own
      configured `revive file-length-limit: 1500` — undetected because that package does not compile.
- [ ] Delete dead code: `newPlaceholderCommand` (`root.go:107`), `mustFinite`
      (`cnossos/road/emission.go:344`), the unused `cfg` param (`cnossos/industry/propagation.go:126`),
      `schall03`'s unexported-candidate `Beiblatt3RetarderRangierenLevel` and `OctaveBands`, and the
      never-called `roundToWholeDB` (`schall03/indicators.go:38`).

## Priority 8 — Frontend correctness

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

### Decisions this work left open

- [ ] **Is the map's load-timeout fallback wanted at all?** `map-view.tsx` called two functions that
      were defined nowhere, so there was no behaviour to restore — only a choice. `mapRef.current`
      is assigned solely inside `m.on("load", …)`, and `load` fires only after the style _and_
      first paint complete, so the timer cannot distinguish a blocked GPU from a slow network. As
      wired it sets a per-mount error at 15 s, and the session-wide WebGL kill switch fires only
      when `new maplibregl.Map()` throws — the one signal that is genuinely permanent.
      `webglcontextlost` deliberately does **not** trip it, since a `webglcontextrestored` handler
      expects recovery. Should the "Map unavailable" panel offer a retry? Nothing clears the flag.
- [ ] **Adopt the E2E suite or delete it — all 6 specs fail.** The tooling half is fixed:
      `frontend/e2e/` and `playwright.config.ts` are Bun-native, type-checked and linted, and
      `just fe-e2e` starts Vite and drives Chromium correctly. The assertions have bit-rotted,
      because no workflow has ever run them. Two causes: `vite.config.ts` sets `base: "/Aconiq/"`
      and `routes.tsx` passes `basename: import.meta.env.BASE_URL`, but the config's `baseURL` is
      `http://localhost:5173` and every spec navigates to a bare `/map`; and the index route
      redirects to `/welcome`, not `/map`. Both axe checks fail as a consequence rather than on
      their own merits, so the accessibility baseline is still entirely unmeasured.
- [ ] Write the missing `frontend/scripts/generate-api-client.mjs` that `package.json` already
      declares, or delete the script entry. The client is hand-written and drifting —
      `/api/v1/import/terrain` has no binding.

### Still open

- [ ] **Move the WASM kernel off the main thread.** `backend/cmd/wasm/main.go` calls
      `road.ComputeReceiverOutputs` synchronously inside the Promise executor — the Promise is
      cosmetic and there is no `Worker` anywhere in `src/`. A grid run freezes the UI.
- [ ] Test the untested half: `src/map/` is thin on tests and `run.tsx` + `results.tsx` +
      `export.tsx` are ~2 600 lines with zero. Run axe against real pages, not the hand-written
      fixture in `src/ui/a11y.test.tsx`. Add `role="dialog"`, focus trap and Escape handling to
      `FeatureEditor`/`FeaturePopup`. The map-rebuild fix in `ad47eaa` also has no test — it needs
      a real WebGL context, and `map.test.tsx` stubs `MapView` out entirely.
- [ ] `settings.tsx` has a hardcoded English paragraph with no message key. Small, but it is the
      same class of defect as the ~24 keys that were wired up.

## Priority 9 — Documentation truth

`AGENTS.md` and `README.md` are rewritten and every structural claim in them was checked against
the repository (`1257365`). `AGENTS.md` now defers all status to this file, carries a 29-row
package table and all ten CLI commands, and describes the standards modules by evidence tier rather
than as peers. The "all linters enabled" claim is gone from `AGENTS.md` and from
`docs/policies/formatting.md`, which also carried it — `README.md` never did.

- [ ] Resolve the two competing lint stacks. `.trunk/trunk.yaml` is invoked by nothing yet holds
      `osv-scanner`, `trivy`, `trufflehog`, `checkov`, `gokart`, `actionlint` — exactly the security
      coverage CI lacks — and pins `go@1.21.0` against a `go 1.25.0` module, and `golangci-lint`
      2.11.4 against CI's 2.12.2. Either promote those scanners into `go-ci.yml` or drop trunk.

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
