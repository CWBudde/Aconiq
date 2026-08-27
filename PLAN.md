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
- **Evidence tier** (new, see Priority 4): how much a module's output may be relied upon.

## Standards evidence tiers

The registry currently exposes 13 standards as peers. They are not peers. This table is the
target state; Priority 4 makes the code express it.

| Module                                | Target tier            | Reality today                                                                    |
| ------------------------------------- | ---------------------- | -------------------------------------------------------------------------------- |
| `rls19-road`                          | normative              | Real Eq. 4/6 structure and coefficients; length weighting fixed (`4142444`)      |
| `schall03`                            | normative              | Anlage-2 tables correct after `bbc2350` — **but the CLI does not call them, P2** |
| `iso9613`                             | normative              | Table 2/3 verbatim correct; three defects fixed (`775c7f5`)                      |
| `talaerm`, `bimschv16`                | normative (assessment) | Threshold tables and logic sound                                                 |
| `beb-exposure`                        | preview                | Aggregation logic reasonable; consumes preview levels                            |
| `cnossos-road/rail/industry/aircraft` | **scaffold**           | No directive coefficients. Invented base levels, no octave bands                 |
| `bub-road`                            | **scaffold**           | Re-parameterised clone of the CNOSSOS scaffold                                   |
| `bub-rail`, `bub-industry`            | **scaffold**           | Pure aliases over `cnossos/*`                                                    |
| `buf-aircraft`                        | **scaffold**           | Byte-identical copy of `cnossos/aircraft` bar one constant                       |
| `dummy-freefield`                     | test fixture           | Intentional                                                                      |

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

### Open

- [ ] **Make the tree buildable without private-repo credentials.** `backend/go.mod` requires
      `github.com/cwbudde/go-absolute-database v0.1.0`, which is unreachable for anyone but the
      author, so a fresh clone and every CI runner still fail even after the `go-overpass` fix
      above. It resolves on a developer machine only from a warm module cache.
      **Diagnosed 27 August 2026 — the module is not missing, it is private.** The repository
      exists at `github.com/CWBudde/go-absolute-database`, tag `v0.1.0` is pushed
      (`626c41e`), and `gh repo view` reports `"visibility": "PRIVATE"`. The other two
      first-party dependencies, `go-overpass` and `go-citygml`, are both public — this one is the
      sole exception. So the earlier framing (vendor the module, or gate `.abs` reading behind a
      build tag) solves a problem that does not exist; a build tag would not even work, since the
      unresolvable `require` poisons whole-graph resolution regardless of which files compile.
      **Action: make `CWBudde/go-absolute-database` public.** It is already MIT-licensed
      (`Copyright (c) 2026 Christian Budde`), ~3 500 LOC, and depends only on cobra and
      `golang.org/x/crypto`. Nothing in Aconiq changes; `NOTICE` has already been corrected.
      Verify afterwards with
      `GOFLAGS=-mod=mod GOMODCACHE=$(mktemp -d) go mod download github.com/cwbudde/go-absolute-database`,
      which must succeed against a cold cache.
- [ ] **Turn on branch protection for `main`.** This is the last step of the lint merge gate and
      the only one that cannot be done from the repo: it needs admin on `CWBudde/Aconiq`, and
      `main` currently has **no protection at all**. The CI side is done — `just lint` is a
      `go-ci.yml` step and exits non-zero on any finding. Required status checks should be
      `go-ci`, `go-race`, `govulncheck` and `frontend-ci`:

      ```bash
      gh api -X PUT repos/CWBudde/Aconiq/branches/main/protection \
        -H "Accept: application/vnd.github+json" \
        -F "required_status_checks[strict]=true" \
        -F "required_status_checks[contexts][]=go-ci" \
        -F "required_status_checks[contexts][]=go-race" \
        -F "required_status_checks[contexts][]=govulncheck" \
        -F "required_status_checks[contexts][]=frontend-ci" \
        -F "enforce_admins=false" \
        -F "required_pull_request_reviews=" \
        -F "restrictions="
      ```

      Note `frontend-ci` is currently green but `fe-bundle-check` is not part of it — the map
      bundle is over budget (Priority 8), so requiring `frontend-ci` does not yet gate bundle size.

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
`bbc2350`). Each was re-verified against the source text before the change, every affected golden
was regenerated, and the per-item magnitudes are recorded in those commit messages and in the
conformance declarations. What remains here is what those fixes did not settle.

### 1.1 Schall 03: two findings raised by the Gl. 14 work, both needing a judgement call

- [ ] **The 50 km/h speed floor does not belong on Eisenbahn segments.** Previously recorded as
      medium confidence; the PDF now settles it. The substitute speed is **Nr. 5.3.2**
      (Straßenbahnen, p. 21): "Ist die Streckenhöchstgeschwindigkeit geringer als 50 km/h, wird
      ersatzweise mit einer Geschwindigkeit von v = 50 km/h gerechnet." **Nr. 4.3** (Eisenbahnen,
      p. 15) prescribes no floor beyond the 70 km/h in Personenbahnhöfen. `resolveEffectiveSpeed`
      (`model.go`) applies `max(v, 50)` unconditionally, so slow Eisenbahn segments compute too
      loud — **+2.2 dB at 1000 Hz, +5.5 dB at 2000 Hz** for a 30 km/h approach. Second finding:
      even for Straßenbahnen, Nr. 5.3.2 scopes the substitution to Weichen, Kreuzungen and
      Haltestellen (each ± 25 m), not the whole segment. Both are recorded as open deviations in
      `docs/conformance/schall03-konformitaetserklaerung.md`.
- [ ] **`FahrbahnartFesteFahrbahn = iota` makes the wrong value the default.** `0` is the zero
      value of `FahrbahnartType` while Schwellengleis is `-1`, so a `TrackSegment` whose JSON omits
      `fahrbahn` silently receives Feste Fahrbahn corrections (+7/+3 dB Schiene, +1 dB Reflexion) —
      contradicting the comment in `sumC1ForTeilquelle` that Schwellengleis is the default.
      `a1_full_chain.scenario.json` is the one fixture that sets `"fahrbahn": 0` explicitly, and
      whether that was intended is unclear. Decide the intended default, then either renumber the
      constants or require the field.

### 1.2 Schall 03 still has the `+1` flow guard

- [ ] `schall03/emission.go` computes `10*log10(trainsPerHour + 1)`, the same spurious **+3.0 dB at
      1 train/h** that was fixed in `cnossos/road`, `cnossos/rail` and `bub/road` (`ac33895`). It
      was outside that change's fence. Use the same explicit zero-flow branch and `-999` sentinel.

### 1.3 Compensated summation — a policy decision, not a defect

- [ ] `docs/policies/determinism.md` §3 requires "a stable strategy (for example pairwise or
      compensated summation)" for sensitive reductions. Every reduction in `standards/` is a plain
      `sum +=`. The map-iteration half of this is now fixed everywhere it was reported. Decide:
      implement Kahan/Neumaier in the shared acoustics core (Priority 7.1), or amend the policy to
      match reality. Do not leave the policy asserting something the code does not do.

### 1.4 The fixture set has blind spots the fixes exposed

Five of the twelve defects moved **no golden at all**, not because they are harmless but because no
fixture exercises them: the lateral-diffraction path is never per-band dominant in any scenario,
every scene has at most two diffraction edges, and no fixture combines a bridge with Feste
Fahrbahn. Two more were invisible for structural reasons — the Tabelle 6 speed-factor error
vanishes at exactly v₀ = 100 km/h, and the acceptance runner decoded `c0_met` and then discarded
it, so ISO 9613-2's C_met was never exercised by any fixture at all.

- [ ] Add fixtures that cover the cases the current set cannot see: a dominant lateral-diffraction
      path, a three-or-more-edge barrier scene, a bridge with Feste Fahrbahn, non-`v₀` train
      speeds, and a non-zero `c0_met`. Without these, the same class of defect can land again and
      the suite will stay green.

### 1.5 RLS-19 items that could not be verified

The RLS-19 text is not in the repo, so these were **not** checked and are not asserted as bugs.

- [ ] Obtain the FGSV RLS-19 text and Korrekturblatt 2/2020, then verify: Tabelle 2 percentages;
      Tabelle 3 coefficients; Eq. 7a–7c gradient corrections (the Lkw `/10` vs Pkw `/100` divisors
      look asymmetric — up to 8–9 dB at 12 % grade); Eq. 9 `min(2h/w, 1.6)`; Eq. 15's constant 80;
      Tabelle 8 reflection losses 0.5/3.0/5.0 dB; Tabelle 6/7 parking values.

## Priority 2 — Make the CLI run the normative code

This is the largest single credibility gap in the repository, and it is not a code-quality issue:
it is a mismatch between what the conformance declaration says and what the binary does.

`backend/internal/app/cli/run_pipeline.go:435` calls `schall03.ComputeReceiverOutputs`, which
uses `BuiltinDataPack()` (`datapack.go:56-101`): invented spectra
`{73,76,80,84,87,85,81,76}`, a scalar `GroundAttenuationDB: 1.2`, an
`AirAbsorptionBandFactor` of `{0.3…2.4}`. The code is candid — `datapack.go:11-13` says
"repo-safe placeholders until a legally safe normative pack is provided out-of-repo", and
`indicators.go:102` stamps every run `compliance_boundary: "baseline-preview-no-normative-tables"`.

Meanwhile `ComputeNormativeReceiverLevels` / `…WithScene` — the real Anlage-2 chain — is reached
from exactly one caller outside the package: `qa/acceptance/schall03/runner.go:293,297`.

- [ ] Wire `run_pipeline.go` to `ComputeNormativeReceiverLevelsWithScene` when the model carries
      the required inputs, falling back to the data pack only with an explicit, logged opt-in.
- [ ] Resolve the contradiction inside `schall03/indicators.go`: line 8 names the model
      `phase20-normative-eisenbahn-strecke-v1` while line 102 stamps
      `baseline-preview-no-normative-tables`. The run manifest currently asserts both.
- [ ] Reconcile `docs/conformance/schall03-konformitaetserklaerung.md`, which checkmarks 20
      normative features the CLI path does not reach.
- [ ] Delete the three mutually inconsistent Schienenbonus statements in that document
      ("K_S = −5 dB nur für den Streckenanteil", "K_S = +5 dB (Schienenbonus retained für
      Straßenbahnen)", vs the code's `kSStrecke = 0.0`). **The code is legally correct** — the
      bonus was abolished in 2015 for Eisenbahnen and 2019 for Straßenbahnen. "+5 dB" is wrong in
      both sign and substance and should not survive review.
- [ ] Decide and document the data-pack story: is the out-of-repo normative pack a real
      distribution mechanism, or should the normative tables (an amtliches Werk, per the project's
      own legal note) simply be embedded?

## Priority 3 — Establish real validation evidence

**The comparison now asserts, and it fails.** `compare_test.go` ran the whole
`init → import --from-soundplan → compare` pipeline against the reference project and never read a
single delta field. It does now (`ac33895`), and against the real fixture Aconiq reads
systematically high on **every one** of the 54 matched receivers:

| indicator | mean abs | p95 abs | max abs | exceeding |
| --------- | -------- | ------- | ------- | --------- |
| LrDay     | 25.110   | 38.903  | 40.789  | 54 / 54   |
| LrNight   | 23.329   | 36.789  | 38.657  | 54 / 54   |

This is the single most important number in this file. Note the shape of it: ~25 dB high, on a
Schall 03 rail project, which is exactly what Priority 2 predicts — the CLI does not call the
normative Anlage-2 chain at all, it calls `BuiltinDataPack()`'s invented spectra. **Priority 2 is
now the prime suspect for this delta and should be done before anything is concluded from it.**

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
      runs. Today `interoperability/` is gitignored and the SoundPLAN tests skip in CI. Move the
      fixture path behind `ACONIQ_SOUNDPLAN_FIXTURES` and remove the customer project name from
      tracked source (`absresults_test.go`, `soundplanimport_test.go`, `import_soundplan_test.go`).
- [ ] Extend the all-skip guard to the Schall 03 acceptance runner. `acceptance.ResolveSuiteSkip`
      and `ACONIQ_STRICT_ACCEPTANCE=1` are wired into `rls19_test20` (`ac33895`); the Schall 03
      report type has no `SkippedCount` field and was left alone.
- [ ] Replace the `(to be filled from conformance report)` placeholders in
      `docs/conformance/rls19-konformitaetserklaerung.md` — all 20 TEST-20 task rows and every
      Max-delta column are blank, under a document titled _Konformitätserklärung_. The document now
      states plainly that they stay empty until the official task set is run, which is honest but
      not evidence.
- [ ] Set tolerances that mean something: ~0.5 dB against a reference tool, not 1e-6 dB against
      yourself. Keep the 1e-6 dB comparison, but rename it what it is — a determinism check.
- [ ] Fuzz the remaining parser. Five targets landed with `a8f3bc2` — GeoTIFF, WKB/GeoPackage,
      GeoJSON, CityGML and the SoundPLAN `.GM` grid — and `FuzzParseGeoTIFF` found a real
      negative-IFD-offset bug on its first run. CSV is still unfuzzed. `FuzzReadFGB` was
      deliberately not added: `gogama/flatgeobuf` reads vtables without validation, so it would
      find library panics this repo cannot fix. Decide whether to run `flatbuffers.Verify` per
      feature buffer or add a scoped `recover()` in `ReadWithCRS`, then add the target.

## Priority 4 — Honest standards labelling

Decided: add an evidence tier and relabel; do not rename packages or delete modules.

- [ ] Add `EvidenceTier` (`normative` | `preview` | `scaffold`) to `framework.StandardDescriptor`
      (`standards/framework/framework.go:57`) and populate it per the tier table above.
- [ ] Surface it where a user cannot miss it: a one-line banner from `aconiq run`, a field in
      `provenance.json` and `run-summary.json`, a row in every report and export bundle, and a
      column in `aconiq status`.
- [ ] Rewrite the `Descriptor()` strings that currently assert conformance —
      e.g. `cnossos/road/model.go:192` "CNOSSOS-EU road preview module with expanded typed source
      schema and deterministic indicators." There is not one CNOSSOS coefficient in the tree:
      no octave bands, no `A_R/B_R/A_P/B_P`, no roughness or contact-filter spectra, no
      `Agr,H/Agr,F`, no `Adiff` with Fresnel/C_f, no favourable-condition probability `p`, no NPD
      curves, no ECAC Doc 29 flight profiles.
- [ ] Build on the normativity signal that already exists rather than a parallel one. Every module
      returns `ProvenanceMetadata` carrying a free-text `compliance_boundary`
      (`iso9613-engineering-octaveband`, `baseline-preview-expanded-road-contract`, …), which
      `buildRunProvenanceMetadata` (`run_options.go:373`) merges into the manifest and
      `run_phase8_test.go` asserts is persisted. The new tier should be the machine-readable form
      of that string and the free text derived from it — not maintained alongside it.
- [ ] Gate the scaffold tier behind an explicit `--experimental` acknowledgement so it cannot
      silently emit authoritative-looking dB(A) values.
- [ ] Rename or remove `cnossos/aircraft` and `buf/aircraft`. **CNOSSOS-EU defines no aircraft
      method at all** — Directive 2015/996 Annex II covers road, rail and industry; aircraft noise
      is ECAC Doc 29. The package name asserts a method the named standard does not contain, which
      no relabelling of the description string fixes.
- [ ] Move the honest disclosures out of `docs/phaseNN-*.md` (named after internal sprint numbers)
      into `docs/conformance/`, where someone evaluating the tool will look. Add a short CNOSSOS
      declaration saying what the module is and is not.
- [ ] Make `run_pipeline.go`'s `default:` branch unreachable. It currently returns "standard %q is
      registered but not wired in run pipeline yet"; `bub-rail` and `bub-industry` are registered
      (`standards/standards.go:24-25`) and unreachable. The registry must not offer what the
      executor cannot run.

---

# Gate 2 — Trustworthy product

## Priority 5 — Provenance integrity and release engineering

Build identity, the provenance schema and the dead links are done (`0a3b11a`): `internal/buildinfo`
is stamped via `-ldflags -X` with a `debug.ReadBuildInfo()` fallback, `aconiq --version` exists, the
cnossos-road run summary no longer carries cnossos-**industry** constants, all eleven `persist*`
functions agree on `model_version`, and `NOTICE` is regenerated from a `just license-report` that
now actually works (it was failing on the toolchain GOROOT and only ever seeing one GOOS).

What is left is release process, which is a set of decisions rather than defects.

- [ ] Rename the `phaseNN-preview-vN` model versions. Internal sprint numbers leak into artifacts
      that are meant to be read by third parties.
- [ ] Start tagging releases. `git tag` is empty; there is no `CHANGELOG.md`, no goreleaser, no
      release workflow. `SECURITY.md` asks reporters for "the version(s) affected" and promises
      fixes "in the latest release" — neither exists. Note the build stamp now uses
      `git describe --tags`, so the first tag immediately improves every artifact's provenance.
- [ ] Record standard-internal data versions — the Schall 03 data-pack version and hash, the
      per-module coefficient-table version — so identical provenance implies identical numbers.
      Do **not** put them in `input_hashes`: `projectfs.Store.hashInputs` defines that map as
      input-file path → SHA-256 and `reporting.inputFilesFromHashes` renders every entry as an
      "Input files" row, so non-file entries there produce misleading reports. Add a dedicated
      standard-data digest field, and only put a coefficient artifact in `input_hashes` when it
      genuinely is a hashed input file. Entangled with the Priority 2 data-pack decision.
- [ ] Define versioning and changelog process (SemVer + `CHANGELOG.md`), publish CLI binaries via
      GitHub Releases, enable Issues with templates, and add release-tag golden-test gates.

## Priority 6 — Security hardening

The threat model is untrusted third-party files plus a local API a browser talks to. **The file
half is closed** (`a8f3bc2`): every allocation the GeoTIFF, GeoPackage/WKB, FlatGeobuf, GeoJSON,
CityGML and SoundPLAN decoders make is now bounded before it happens, with a regression test per
bound and five fuzz targets. The GeoTIFF OOM turned out to be reachable from an **86-byte** input,
and a deflate bomb that was not in this list was found and closed alongside it.

**The API half is untouched and is now the whole of this priority.**

- [ ] **Local API is unauthenticated and CSRF-open.** No auth, no CSRF token, no `Host` validation.
      `handleRunCreate` decodes JSON without checking `Content-Type`, so a cross-origin `fetch`
      with `text/plain` is CORS-safelisted, skips preflight, and executes `exec.CommandContext`
      with attacker-chosen `--model`/`--param`/`--input`. DNS rebinding bypasses the origin check
      entirely. The comment at `cors.go:17` is incorrect for both shapes. Add a Host allowlist,
      reject non-`application/json` bodies, and mint a session token at `serve` startup.
- [ ] **No request size limits.** `ParseMultipartForm`'s argument is `maxMemory`, not a cap; the
      remainder spills to unbounded temp files, then `io.ReadAll`. There are **zero** occurrences
      of `http.MaxBytesReader` or `io.LimitReader` in `api/httpv1`. This is also the standing G120
      finding.
- [ ] **Arbitrary file read.** `handler.go:519` builds a path by raw string concatenation with no
      `Clean` and no containment check; a shared project whose manifest sets
      `log_path: "../../../../etc/passwd"` reads it over HTTP. Use `filepath.IsLocal()` +
      `os.OpenInRoot`. `:577` cleans but still does not constrain.
- [ ] **SSRF by design.** `overpass_endpoint` (`handler.go` → `osmimport.go`) is taken from the
      request body with no scheme/host validation. Allowlist it or drop it from the API.
- [ ] Re-examine the gosec **G304** suppression on current evidence. It was judged acceptable at 47
      findings and now stands at 97 (50 test, 47 non-test) — the same drift that made G301 worth
      fixing. Opening a user-named path is arguably what a file-format toolchain does, but nobody
      has re-litigated the verdict at the new count. The other two kept findings still worth
      judgement are **G702** at `api/httpv1/handler.go:456` (the HTTP handler shells out to
      `aconiq` with request-controlled argv — no shell involved, so not classic injection, but an
      exposure surface that Priority 7's "move the run pipeline out of `app/cli`" removes outright)
      and **G120**, folded into the size-limit item above. **G202** at
      `report/export/gpkg.go:207` is a false positive: `sanitizeColumnName` is a strict allow-list.
- [ ] Add `flatbuffers.Verify` per feature buffer, or a scoped `recover()` in
      `fgbimport.ReadWithCRS`. `gogama/flatgeobuf` reads vtables without validation, so a corrupt
      vtable can panic inside the library before any of the bounds added in `a8f3bc2` are reached.
      This is what blocks a `FuzzReadFGB` target (Priority 3).
- [ ] Add a CI guard that hard-fails on any tracked `interoperability/` path, and write the
      data-handling policy that currently does not exist. 4.1 GB of third-party project data is
      protected by a single `.gitignore` line. _(Verified: nothing proprietary has ever been
      committed on any branch — this is about keeping it that way.)_

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

- [ ] **`backend/README.md` is now the worst stale-status file in the repo.** It carries a
      phase-by-phase "Phase 3 Baseline … Phase 23 Initial Slice" changelog and a "Planned package
      structure" list for packages that all exist. It was not in this priority's original scope and
      is untouched. Either fold it into the root `README.md` or cut it to a build note.
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
- [ ] Fix the "7 reference rows" comment (`atmospheric.go:12`) and its echo in the conformance
      doc — there are 6, matching the standard.
- [ ] `schall03/propagation.go:169-178` — `directivityDI`'s doc comment defines δ against the
      perpendicular to the track axis; Gl. 8 defines it against the Gleisachse. The inlined callers
      use the correct convention, so this is currently dead code with an inverted contract worth
      8.3 dB if anyone calls it. Fix or delete.

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
      For the CNOSSOS family, publish a scope statement instead until real coefficients exist (P4).
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
