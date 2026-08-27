# PLAN.md — Aconiq Roadmap

Status: 27 August 2026

This file tracks **what is still ahead**. Completed work is recorded in git history, in
`docs/conformance/`, and in the per-module baseline notes under `docs/`. Nothing here is a
status report.

Ordering principle: correctness and evidence come before features. A calculation that is wrong
by 23 dB is not improved by a nicer report template.

The "Phase QR" (quality remediation) block added in `9ec398d` has been absorbed here. Its
sub-phases map onto the new numbering as: QR-1 → Priority 0, QR-2 → Priorities 2 and 4,
QR-3 → Priority 1.8, QR-4 → Priority 3, QR-5 → Priority 7, QR-6 → Priorities 5 and 9. Commit
messages referencing `QR-N` remain resolvable through this table.

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

| Module                                | Target tier            | Reality today                                                              |
| ------------------------------------- | ---------------------- | -------------------------------------------------------------------------- |
| `rls19-road`                          | normative              | Real Eq. 4/6 structure and coefficients; **propagation defect, P1.1**      |
| `schall03`                            | normative              | Real Anlage-2 tables and Gl. 1–36 — **but the CLI does not call them, P2** |
| `iso9613`                             | normative              | Table 2/3 verbatim correct; **three defects, P1.2–P1.4**                   |
| `talaerm`, `bimschv16`                | normative (assessment) | Threshold tables and logic sound                                           |
| `beb-exposure`                        | preview                | Aggregation logic reasonable; consumes preview levels                      |
| `cnossos-road/rail/industry/aircraft` | **scaffold**           | No directive coefficients. Invented base levels, no octave bands           |
| `bub-road`                            | **scaffold**           | Re-parameterised clone of the CNOSSOS scaffold                             |
| `bub-rail`, `bub-industry`            | **scaffold**           | Pure aliases over `cnossos/*`                                              |
| `buf-aircraft`                        | **scaffold**           | Byte-identical copy of `cnossos/aircraft` bar one constant                 |
| `dummy-freefield`                     | test fixture           | Intentional                                                                |

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

- [ ] **Pay down the debts a green `just lint` hides.** `wrapcheck` is excluded for all of
      `internal/`, i.e. the entire backend, so the error-wrapping policy is unenforced; removing
      that exclusion costs 190 findings and should be sequenced after the `domain/errors` work in
      Priority 7. Separately, 430 of the original 542 findings were removed by switching linters
      off rather than by fixing code. Per-linter reasons in `docs/lint-triage.md`.
- [ ] **Finish the frontend package-manager consolidation.** `frontend/package-lock.json` is
      deleted — nothing referenced it (both workflows use `oven-sh/setup-bun` and
      `bun install --frozen-lockfile`; a repo-wide grep for `npm ci`/`npm install`/`package-lock`
      returns nothing outside this file). It had drifted from `bun.lock` on three resolved
      versions. **The root `package.json` and root `package-lock.json` must not simply be
      deleted**: the root `package.json` is not a workspace root — it has no `name`, `private`,
      `version` or `workspaces`, only `{"devDependencies": {"@axe-core/playwright": "…"}}` — and
      that lockfile is the only pin for `@axe-core/playwright`, which `e2e/smoke.spec.ts` imports
      and which is absent from `frontend/bun.lock`. The clean fix is a small restructure: move
      `@axe-core/playwright` into `frontend/package.json` devDependencies, relocate
      `playwright.config.ts` and `e2e/` under `frontend/`, switch `just fe-e2e` from
      `npx playwright test` to `bunx playwright test`, then delete both root files. Pairs with the
      "adopt the E2E suite or delete it" item in Priority 8.
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
- [ ] **Switch `just fe-e2e` and `playwright.config.ts` from `npx` to `bunx`.** `just fe-e2e` runs
      `npx playwright test` while everything else uses Bun, and `@playwright/test` is not declared
      anywhere — so it relies on an on-the-fly npm download. `playwright.config.ts:30` starts the
      dev server with `cd frontend && npx vite` for the same reason.

## Priority 1 — Fix known numeric defects

Found by review against normative sources. The Schall 03 items were checked against
`docs/bimsch16_anl2_neu-1.pdf` (the actual Anlage 2 text, present locally, gitignored).
Each fixes to a small number of lines; the _consequence_ is that every affected golden file must
be regenerated and every conformance table re-derived.

### 1.1 RLS-19: line-source contributions normalised by total road length — CRITICAL

`backend/internal/standards/rls19/road/propagation.go:458`

```go
lengthWeight := 10 * math.Log10(seg.LengthM/totalLength)   // wrong
lengthWeight := 10 * math.Log10(seg.LengthM)               // right (l₀ = 1 m)
```

`emission.LmEDay` is a **length-related** level in dB(A)/m — `emission.go:70` subtracts 30 dB
precisely to convert veh/h ÷ km/h into per-metre. Dividing by `totalLength` makes
Σ 10^(w/10) = 1, so the entire road radiates as if it were 1 m long.

Error = −10·lg(L_road / 1 m): **−20 dB at 100 m, −23 dB at 200 m, −30 dB at 1 km.**

Two independent confirmations it is a bug and not a convention: splitting one 200 m source into
two 100 m sources changes the answer by ~3 dB (the result depends on how the user chops the
geometry), and the parking path in the same package (`parking.go:200`) uses a total sound power
with no such normalisation and produces sane absolute levels.

- [ ] Fix `propagation.go:458` and the reflected path at `propagation.go:538-539`.
- [ ] Regenerate all 24 RLS-19 `ci_safe` fixtures and re-derive
      `docs/conformance/rls19-konformitaetserklaerung.md`.
- [ ] Add an absolute-level regression test, not a relational one — every existing RLS-19
      propagation test ("near > far", "day > night", "doubling adds 3 dB") passes under this bug.

### 1.2 ISO 9613-2: broadband L_WA replicated into all 8 bands — +7.0 dB on the default path

`backend/internal/standards/iso9613/octaveband.go:27-34` sets all 8 bands to `L_WA`;
`propagation.go:116` then energy-sums them **and adds A-weighting again**:
Σ 10^(0.1·A_j) = 4.997 → **+6.99 dB**.

The doc comment cites ISO 9613-2 Note 1 ("use the 500 Hz attenuation terms when only the
A-weighted sound power is known") — which requires a **single** band evaluated at 500 Hz with
**no** re-weighting.

This is the default path, not a corner case: `model.go:184` makes
`iso9613_sound_power_level_db` the only import parameter and `OctaveBandLevels` is nil by
default. Both committed goldens use it.

- [ ] Implement Note 1 as a single 500 Hz band, or require octave-band input.
- [ ] Regenerate `qa/acceptance/testdata/iso9613/*.golden.json`.

### 1.3 ISO 9613-2: sign error in A_m at 63 Hz

`backend/internal/standards/iso9613/ground.go:62` returns `3 * q`; Table 3 gives `−3q`.
The `default:` branch on the next line correctly returns `−3*q*(1-gm)` — the two branches
disagree in sign within one function. Error 6q dB in the 63 Hz band; A-weighted impact usually
< 0.1 dB, but the band output is wrong and it matters for low-frequency-dominated sources.

- [ ] Fix the sign.

### 1.4 ISO 9613-2: C_met applied once, post-summation, from the farthest source

`iso9613/compute.go:52-66` takes `max(d_p)` over all sources and `sources[0].SourceHeightM`,
computes one C*met, and subtracts it from the already energy-summed `L_AT(DW)`. Eq. 6 applies
C_met per source–receiver path \_before* summation. Near sources are over-corrected by up to
C₀ dB. Currently masked because `c0_met` defaults to 0.

- [ ] Move C_met inside the per-source loop.
- [ ] `iso9613/barrier.go:6-23` — `pathDifference` is ≥ 0 for any input, so a receiver with clear
      line of sight over a low barrier still gets full A_bar. Require z < 0 when the line of sight
      clears the screen, or reject inconsistent `BarrierGeometry`.

### 1.5 Schall 03: Gl. 14 misreads the reference length d₀ = 1 m as a path length

`backend/internal/standards/schall03/propagation.go:150-153`

```go
val := 4.8 - (2.0*hm/d)*(17.0+300.0*dp/d)   // dp = land path length, wrong
val := 4.8 - (2.0*hm/d)*(17.0+300.0/d)      // d₀ = 1 m per Gl. 11
```

Gl. 14 (PDF p. 24) reads `A_gr,B = [4,8 − (2·h_m/d)·(17 + 300·d₀/d)] ≥ 0 dB`, and its variable
list contains only `h_m`, `d`, `S` — **d_p does not appear in Gl. 14** (it appears only in
Gl. 16, the water term). The code substitutes a land path length of tens to hundreds of metres
for a 1 m constant.

With h_m = 2 m: **+4.0 dB at 100 m, +4.5 dB at 250 m, +1.2 dB at 1000 m** too loud. Ground
damping is the only ground term in Schall 03, so the code effectively zeroes it at normal
assessment distances. Affects direct paths (`compute.go:100,279`), reflected paths
(`reflection.go:257,315`) and yard sources (`rangierbahnhof.go`).

- [ ] Fix Gl. 14 and drop the `dp` parameter.
- [ ] Document that `h_m` is hard-coded to `(hg+hr)/2` (`compute.go:82`) whereas Gl. 15 defines
      `h_m = S/d`. Defensible as a flat-ground special case, but it is not currently listed among
      the conformance document's known limitations.

### 1.6 Schall 03: Tabelle 6 Rollgeräusche speed-factor row shifted by one band

`backend/internal/standards/schall03/tables.go:31`

```go
B: BeiblattSpectrum{-5, -5, 0, 10, 25, 25, 25, 25},      // wrong — a −5 was dropped
B: BeiblattSpectrum{-5, -5, -5, 0, 10, 25, 25, 25},      // Tabelle 6, Zeile 2 (PDF p. 15)
```

The speed term is `b_f·lg(v/v₀)` (`emission_v2.go:193`), so the per-band error is `Δb·lg(v/100)`:
**+3.1 dB at 1000 Hz for 160 km/h, −3.3 dB for 60 km/h.** 1000 Hz dominates A-weighted rail
rolling noise, so A-weighted error is roughly ±2–3 dB. It vanishes at exactly v₀ = 100 km/h,
which is why the unit tests miss it.

- [ ] Fix the row. Tabellen 7, 9 and 17 were checked against the PDF and are exact — no change.
- [ ] Add a speed-sweep test at v ≠ 100 km/h so a shifted row cannot pass again.

### 1.7 Schall 03: remaining defects

- [ ] **Tabelle 7 c1 double-counted on bridges** (`emission_v2.go:202` and `:204`). The Festlegungen to
      Tabelle 9 (PDF p. 17) state Tabelle 7 Zeile 1–4 corrections are _nicht anzusetzen_ when a
      bridge correction applies. For "Brücke mit fester Fahrbahn" the code adds K*Br = +4 \_and*
      c1 = +7 dB → **+8 dB at 500 Hz**.
- [ ] **D_refl applied to every barrier with no d_s ≤ 5 m test** (`barrier.go:134-136`,
      `barrier_scene.go:295`). Gl. 20 defines D_refl only for reflective walls at d_s ≤ 5 m with an
      absorbing base. There is no distance check and no reflective/absorbing flag in
      `BarrierSegment`, so **every** barrier loses up to 3 dB of insertion loss.
- [ ] **Reflected-path directivity computed for the wrong direction** (`reflection.go:512-520`,
      `:590-598`). The code uses source → its own mirror image, which is perpendicular to the wall
      by construction and independent of the receiver; Gl. 28 requires the direction toward the
      reflection point. For a wall parallel to the track this always yields sin²δ = 1 → +1.73 dB
      where the correct value reaches −6.58 dB. **Worst case 8.3 dB.** Use
      `firstGeom.ReflectionPoint − pt`.
- [ ] **`energeticTotalSpectrum` sums dB arithmetically** (`barrier_scene.go:436-446`) in a
      function documented as "the A-weighted energetic sum". No 10^(x/10), no A-weighting. This is
      the only arithmetic-dB sum in `standards/`. It ranks candidate lateral-diffraction paths, so
      it can select the wrong dominant path.
- [ ] **Lateral diffraction climbs over the barrier top** (`barrier_scene.go:393-413`).
      `lateralPathAbar` computes the plan-view detour around the barrier _endpoint_ but sets
      `dhS = barrierTopH − sourceH`, adding a full vertical climb to a path that goes _around_ a
      vertical edge. `z` is over-estimated, so lateral attenuation is over-estimated, and since
      `ComputePathBarrierAttenuation:507-511` takes the per-band minimum, the lateral path is then
      almost never selected. Both halves are wrong and partly cancel.
- [ ] **`e` computed as a chord, not a polyline** (`ComputeBarrierGeometryFromEdges:266-269`).
      Bild 6 defines `e = e₁+e₂+e₃…` along the diffracted path; the code keeps the outermost two
      hull edges and uses the straight distance. `z` is under-estimated.
- [ ] **Spurious 50 km/h speed floor on Eisenbahn segments** (`resolveEffectiveSpeed`, `model.go:372-380`, floor at `:375`). The 70 km/h
      floor in Personenbahnhöfen is normative (PDF p. 15); the general 50 km/h floor is the
      Straßenbahn rule from Nr. 5.3.2, already applied separately at `emission_v2.go:82-88`.
      For a 30 km/h approach: **+2.2 dB at 1000 Hz, +5.5 dB at 2000 Hz.**
      _Medium confidence — verify against Nr. 4.3 before changing._

### 1.8 Determinism defects

- [ ] **Map iteration order determines summation order.** `compute.go:78`, `compute.go:260`,
      `reflection.go:238`, `reflection.go:296` all do
      `for h, spectrum := range emission.PerHeight` (a `map[int]BeiblattSpectrum`) and accumulate
      floats. Magnitude ~1 ULP, but it is a direct violation of `docs/policies/determinism.md`
      ("Map iteration order must never influence numeric results"). Iterate a fixed `[]int{1,2,3}`.
- [ ] **No compensated summation anywhere**, despite `determinism.md` §3 requiring "a stable
      strategy (for example pairwise or compensated summation)" for sensitive reductions. Every
      reduction in `standards/` is a plain `sum +=`. Decide: implement Kahan/Neumaier in the shared
      acoustics core (Priority 7.1), or amend the policy to match reality.
- [ ] `10*log10(flow + 1)` guards (`cnossos/road/emission.go:86`, `cnossos/rail/emission.go:40`,
      `bub/road/emission.go:43`) add a spurious **+3.0 dB at 1 veh/h**. Use an explicit
      zero-flow branch.
- [ ] `cnossos/road/emission.go:330` does not filter the `−999` sentinel that
      `emissionForVehicleClass:56` returns, whereas `cnossos/industry` and `rls19` do. Harmless
      today (10⁻⁹⁹·⁹) but inconsistent — fold into the shared `EnergySum` (Priority 7.1).

### 1.9 RLS-19 items that could not be verified

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

Today: **0 % of the numerical output is validated against an independent reference.** All 69
`*.golden.*` files are snapshots of the code that produces them, generated by
`qa/acceptance/rls19_test20/runner_test.go:126-171` and
`qa/acceptance/schall03/runner.go:361-395`, compared at **1e-6 dB** — a bit-identity check
wearing conformance vocabulary. That is why a 23 dB error survived.

The single highest-value change on this entire roadmap is already 90 % built.

- [ ] **Assert on the SoundPLAN deltas.** `backend/internal/app/cli/compare_test.go:13-80` runs
      `init → import --from-soundplan → compare` against a real reference project (77 receivers,
      4 raster runs) and computes `MeanAbsDeltaDB`, `MaxAbsDeltaDB`, `P95AbsDeltaDB`,
      `ToleranceExceeding` (`compare.go:27-33`). The test asserts row counts and non-empty IDs and
      **never reads a single delta field**. No test anywhere in the repo does. Add the assertions.
- [ ] Get reference data into CI — submodule or Git LFS, licence permitting — so the comparison
      runs. Today `interoperability/` is gitignored, 53 SoundPLAN tests skip silently in CI and 9
      hard-fail. Move the fixture path behind `ACONIQ_SOUNDPLAN_FIXTURES` and remove the customer
      project name from tracked source (`absresults_test.go:9`, `soundplanimport_test.go:15`,
      `import_soundplan_test.go:197`).
- [ ] Make a full-suite skip a failure. A suite where every test skips is worse than a red one.
      `rls19_test20/runner_test.go:44-63` already shows the right pattern (skip _with a reason_,
      surfaced in the report).
- [ ] Replace the `(to be filled from conformance report)` placeholders in
      `docs/conformance/rls19-konformitaetserklaerung.md:110-145` — all 20 TEST-20 task rows and
      every Max-delta column are blank, under a document titled _Konformitätserklärung_. Either
      populate with measured deltas or state plainly that validation has not been performed.
- [ ] Set tolerances that mean something: ~0.5 dB against a reference tool, not 1e-6 dB against
      yourself. Keep the 1e-6 dB comparison, but rename it what it is — a determinism check.
- [ ] Add clause-cited formula tests to the scaffold modules, or delete the tautological ones.
      `cnossos/road/road_test.go:177-183` asserts `terms.GeometricDB == geometricDivergence(100)`
      — the function compared against a direct call to its own helper. It cannot fail. Same shape
      at `cnossos/rail:139,143`, `cnossos/industry:129`, `bub/road:178`, `beb/exposure:228`.
- [ ] Fuzz the parsers that actually take untrusted input — GeoJSON, CSV, GeoTIFF, WKB, SoundPLAN.
      The only fuzz target today is `geo/geometry_fuzz_test.go:8` on point-segment distance.
- [ ] Add tests for `internal/domain/errors` (the taxonomy that decides CLI exit codes, 0 %) and
      `internal/app/config` (0 %).
- [ ] De-flake `TestCancellationLeavesConsistentState` (`engine/runner_test.go:76`). It races a
      40 ms `time.Sleep` against 600 receivers at a 2 ms `ComputeDelay` and asserts the run was
      cancelled; on a loaded runner the work can finish first. Drive cancellation off a
      deterministic signal from inside the compute callback.

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

For a tool whose output is intended to support planning-approval files, an artifact that cannot
be traced to the binary that produced it has no evidentiary value.

- [ ] **`toolVersion = "dev"` is hardcoded** at `backend/internal/io/projectfs/store.go:23`.
      Every `provenance.json` ever written claims `"tool_version": "dev"`. Inject version, commit
      and build date via `-ldflags -X` in `just build` and CI, with a `debug.ReadBuildInfo()`
      fallback.
- [ ] Add `aconiq --version`. There is no version command and no `Version:` field on the root
      cobra command (`app/cli/root.go:90-100`).
- [ ] **Fix the wrong model version in cnossos-road provenance.**
      `run_persist.go:226-227` writes `cnossosindustry.BuiltinModelVersion` ("phase12-preview-v2")
      and `cnossosindustry.ReportingPrecisionDB` into every **cnossos-road** run summary; the
      correct constant is `cnossosroad.*` ("phase10-preview-v2").
- [ ] Unify the four different provenance schemas this produced: `model_version` present in 6
      `persist*` functions, absent in `persistBUFAircraftRunOutputs:558`,
      `persistCnossosIndustryRunOutputs:667` and `persistISO9613RunOutputs:720`, and renamed
      `data_pack_version` in `persistRLS19RoadRunOutputs:339`.
- [ ] Rename the `phaseNN-preview-vN` model versions. Internal sprint numbers leak into artifacts
      that are meant to be read by third parties.
- [ ] Start tagging releases. `git tag` is empty; there is no `CHANGELOG.md`, no goreleaser, no
      release workflow. `SECURITY.md` asks reporters for "the version(s) affected" and promises
      fixes "in the latest release" — neither exists.
- [ ] Fix the dead links: `CONTRIBUTING.md:52` and `SECURITY.md:7` point at
      `github.com/aconiq/backend`; the remote is `github.com:cwbudde/Aconiq`. Both 404 —
      including the security-advisory link.
- [x] `CONTRIBUTING.md` now says "Go 1.25+", matching `backend/go.mod` (`b96e319`).
- [ ] Regenerate `NOTICE` from `just license-report`. It still omits `mousetrap`, `go-isatty` and
      `go-strftime`. Two false claims were corrected by hand as P0 landed — the stale `go-overpass`
      entry, and `go-absolute-database` being listed as "MIT (local replace)" under a heading
      "Internal dependencies (MeKo-Tech)" when `backend/go.mod` has no `replace` directive and the
      module is not in that namespace — but the file has still never been generated from the
      actual module graph, which is the only thing that stops it drifting again.
- [ ] Record standard-internal data versions — the Schall 03 data-pack version and hash, the
      per-module coefficient-table version — so identical provenance implies identical numbers.
      Do **not** put them in `input_hashes`: `projectfs.Store.hashInputs` defines that map as
      input-file path → SHA-256 and `reporting.inputFilesFromHashes` renders every entry as an
      "Input files" row, so non-file entries there produce misleading reports. Add a dedicated
      standard-data digest field, and only put a coefficient artifact in `input_hashes` when it
      genuinely is a hashed input file.
- [ ] Define versioning and changelog process (SemVer + `CHANGELOG.md`), publish CLI binaries via
      GitHub Releases, enable Issues with templates, and add release-tag golden-test gates.

## Priority 6 — Security hardening

The threat model is untrusted third-party files plus a local API a browser talks to. Both are
currently open.

- [ ] **GeoTIFF: unrecoverable OOM from a ~250-byte upload.**
      `backend/internal/geo/terrain/geotiff.go:249` never validates `bps`; `getUint` returns 0 for
      a missing `BitsPerSample`, so `bytesPerSample == 0` and the guard
      `if len(raw) < n*bytesPerSample` (`:454`) compares against 0 and **fails open**.
      `make([]float64, n)` at `:458` then runs with attacker-controlled `n` — `2^40` = 8 TiB →
      `runtime.throw`, uncatchable. Reachable unauthenticated and cross-origin via
      `POST /api/v1/import/terrain` (`handler.go:785`) and fatally in `cmd/wasm/main.go:96`.
      Validate `bps ∈ {8,16,32,64}` and `width*height <= maxPixels` with overflow-safe arithmetic
      before any allocation. Same class at `:376`, `:597`, `:141-154`.
- [ ] **Local API is unauthenticated and CSRF-open.** No auth, no CSRF token, no `Host` validation.
      `handleRunCreate` (`handler.go:322`) decodes JSON without checking `Content-Type`, so a
      cross-origin `fetch` with `text/plain` is CORS-safelisted, skips preflight, and executes
      `exec.CommandContext` with attacker-chosen `--model`/`--param`/`--input`. DNS rebinding
      bypasses the origin check entirely. The comment at `cors.go:17` is incorrect for both shapes.
      Add a Host allowlist, reject non-`application/json` bodies, and mint a session token at
      `serve` startup.
- [ ] **No request size limits.** `ParseMultipartForm`'s argument is `maxMemory`, not a cap
      (`handler.go:736-743,781`); the remainder spills to unbounded temp files, then `io.ReadAll`.
      There are **zero** occurrences of `http.MaxBytesReader` or `io.LimitReader` in the backend.
- [ ] **Arbitrary file read.** `handler.go:519` builds a path by raw string concatenation with no
      `Clean` and no containment check; a shared project whose manifest sets
      `log_path: "../../../../etc/passwd"` reads it over HTTP. Use `filepath.IsLocal()` +
      `os.OpenInRoot`. `:577` cleans but still does not constrain.
- [ ] **SSRF by design.** `overpass_endpoint` (`handler.go:715` → `osmimport.go:41-54`) is taken
      from the request body with no scheme/host validation. Allowlist it or drop it from the API.
- [ ] **GeoPackage SQL injection into a read-write handle.** `gpkgimport.go:155,178` build
      `"SELECT * FROM "+tableName`; the `//nolint:gosec` justification ("comes from
      `gpkg_geometry_columns` metadata") is backwards — that metadata is in the attacker's file.
      `sql.Open("sqlite", path)` (`:25,85`) opens READWRITE|CREATE and `ATTACH` is compiled in.
      Whitelist `^[A-Za-z_][A-Za-z0-9_]*$`, quote it, and open `?mode=ro&immutable=1`.
- [ ] **WKB decoder.** `wkb.go:167-170,198-201` allocate from a 4-byte count before bounds
      checking (68 GB from a 17-byte blob) and `:204` recurses with no depth cap (fatal stack
      exhaustion at ~18 MB). `readPoints` at `:218-224` already does this correctly — copy it.
- [ ] Bound the remaining O(n²) and unvalidated-length paths: `gridmap.go:328-338`,
      `modelgeojson/validate.go:355-380`, `fgbimport.go:167,191,225`, `citygmlimport.go:62`
      (nesting depth only — XXE and billion-laughs are already safe under Go's `xml.Decoder`),
      `store.go:365-377`, `gridmap.go:87` / `railops.go:155`.
- [ ] Stop neutering gosec: `.golangci.yml` exclusion presets suppress G304 (47 hits) and G301
      (44 hits, all `MkdirAll(0o755)`). Decide each explicitly rather than suppressing silently —
      G304 is arguably the product (the tool opens paths the user names), G301 is a real choice.
      **Correction (27 August 2026):** the earlier claim that "17 G115 integer-overflow hits
      concentrate in `geotiff.go`" is wrong. `geo/terrain/geotiff.go` carries 13 explicit,
      individually justified `//nolint:gosec` directives, all confirmed still live by
      `nolintlint`; that is correct handling, not suppression. A gosec-only run over the whole
      tree surfaces no unhandled G115. See `docs/lint-triage.md`.
      The 8 kept gosec findings do include two worth judgement: **G702** at
      `api/httpv1/handler.go:456` (the HTTP handler shells out to `aconiq` with request-controlled
      argv — no shell involved, so not classic injection, but a real exposure surface that the
      Priority 7 "move the run pipeline out of `app/cli`" item would remove outright) and **G120**
      at `:743` (`ParseMultipartForm` bounds memory only, the body is unbounded — same defect as
      the "no request size limits" item above). **G202** at `report/export/gpkg.go:207` is a
      false positive: `sanitizeColumnName` is a strict `[a-z0-9_]` allow-list.
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
      This also lands the compensated-summation item from P1.8.
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

Not "deferred polish" — the shipped Run page throws on its default path. All four crashes would
have been caught by the typecheck gate fixed in Priority 0.

All four shipped crashes listed here are now fixed — see the Priority 0 Landed entry. Two of
them left a decision behind:

- [ ] **Decide whether the map's load-timeout fallback is wanted at all.** `map-view.tsx` called
      two functions that were defined nowhere, so there was no behaviour to restore — only a
      choice. `mapRef.current` is assigned solely inside `m.on("load", …)`, and `load` fires only
      after the style _and_ first paint complete, so the timer cannot distinguish a blocked GPU
      from a slow network. As wired it now sets a per-mount error at 15 s (was 4 s), and the
      session-wide WebGL kill switch fires only when `new maplibregl.Map()` throws — the one
      signal that is genuinely permanent. `webglcontextlost` deliberately does **not** trip it,
      since there is a `webglcontextrestored` handler that expects recovery. Open question: is a
      timeout-driven fallback worth keeping, and should the "Map unavailable" panel offer a retry?
      Nothing currently clears the session flag.
- [ ] **Add a "receiver" case wherever `option_source|building|barrier` are handled.**
      `option_receiver` was simply missing from `messages/{en,de}.json` and has been added, which
      suggests the receiver branch of that dialog was never exercised. Worth a test.
- [ ] **`calcArea` is marked dirty but never persisted** — silent data loss.
      `use-autosave.ts:70` saves only `{features, receivers}` while `model-store.ts:238` sets
      `dirty: true` on `setCalcArea`, and `calcArea` is missing from the effect deps
      (`use-autosave.ts:81`), so a pending timer writes a stale draft and calls `markClean()`.
- [ ] **The map is destroyed and rebuilt on every model edit.** `map-view.tsx:103` deps include
      `center`/`zoom`, and `pages/map.tsx:126-129` re-memoizes `workspaceView` on every store
      change. Pan/zoom is lost, layers re-added, the fallback timer re-armed. (The stale
      `eslint-disable` and the "only re-create on basemap change" comment that contradicted the
      deps array were removed in `e55e366`; the deps themselves are unchanged.)
- [ ] Fix render-phase `setState` at `pages/export.tsx:417`, `pages/results.tsx:769` **and
      `pages/run.tsx:1138`** (a third instance of the same "auto-select first run" shape, found
      while fixing the type errors — the index access is now sound but the write is still in the
      render body; the fix is a `useEffect` or deriving `selectedRun ?? firstRun` instead of
      storing it); the
      global Ctrl+Z that hijacks text inputs (`map/undo-redo-bar.tsx:20`); object URLs revoked
      while still rendered (`api/browser-backend.ts:281-284`); silently swallowed layer errors
      (`map/model-layers.tsx:55-57,74-76`); `loadReceivers` bypassing the command stack
      (`model-store.ts:225`); `CommandStack` never unsubscribing (`model-store.ts:44`).
- [ ] **Move the WASM kernel off the main thread.** `backend/cmd/wasm/main.go:47-72` calls
      `road.ComputeReceiverOutputs` synchronously inside the Promise executor — the Promise is
      cosmetic and there is no `Worker` anywhere in `src/`. A grid run freezes the UI.
- [ ] **Stop the frontend/backend `SurfaceType` list from drifting again.** The typecheck fix
      exposed a live numeric defect: `src/wasm/types.ts` declared 10 of the 18 Go
      `rls19/road.SurfaceType` constants, missing `SMA-5-8`, `SMA-8-11`, `OPA-11`, `OPA-8`,
      `Pflaster-eben`, `Pflaster-sonstig`, `SMA-LA-8` and `Gussasphalt-nicht-geriffelt`. These are
      not aliases — each has its own DStrO row in `rls19/road/tables.go`, and the spread is large
      (`Pflaster-sonstig` +5…+7 dB vs `Pflaster-eben` +1…+3 dB; `OPA-8` −5.5 dB). Narrowing an
      incoming surface against the stale union would have silently downgraded all eight to the
      `"SMA"` fallback and changed the computed emission. The union has been completed, but it is
      now the **third** hand-maintained copy of the same list (Go constants,
      `model/source-acoustics.ts:RLS19_SURFACE_TYPES`, `wasm/types.ts`). Generate them from the Go
      constants, or at minimum add a test that fails when the three diverge.
- [ ] Write the missing `frontend/scripts/generate-api-client.mjs` that `package.json:12` already
      declares, or delete the script entry. The client is hand-written and drifting —
      `/api/v1/import/terrain` (`handler.go:230`) has no binding.
- [ ] Fix the bundle budget: `dist/assets/map-*.js` is 1 214 KB against a 750 KB limit
      (`scripts/check-bundle-size.mjs:14`), so `bun run bundle-check` — and therefore `just ci` via
      `fe-ci` — fails on a real build. Move budgets to gzip.
- [ ] Delete dead frontend code: the `APIClient` class in `src/api/client.ts` (never
      instantiated — only re-exported from `api/index.ts`; **the rest of the module is live**, six
      files import its types, so do not delete the file), `src/map/legend.tsx` (never rendered),
      `ValidationPanel` (unreachable — `pages/map.tsx:119` never calls `setShowValidation(true)`),
      58 unused message keys. Note the header bug fixed in `e55e366` was inside that unused class,
      which is how it survived.
- [ ] Test the untested half: `src/map/` has 1 test across 17 files; `run.tsx` + `results.tsx` +
      `export.tsx` are 2 597 lines with zero. Run axe against real pages, not the hand-written
      fixture in `src/ui/a11y.test.tsx`. Add `role="dialog"`, focus trap and Escape handling to
      `FeatureEditor`/`FeaturePopup`.
- [ ] Fix `feature-editor.tsx:26-36`, which calls `m.option_source()` at module scope, freezing
      labels at import time. `draw-toolbar.tsx:23` shows the correct pattern.
- [ ] Adopt the E2E suite or delete it: `e2e/smoke.spec.ts` + `playwright.config.ts` + `just fe-e2e`
      exist, no workflow runs them, and root `package.json` declares `@axe-core/playwright` but not
      `@playwright/test`.
- [ ] Add `backend/**` to `frontend-ci.yml`'s path filter (a WASM API change never triggers
      frontend CI) and gate `gh-pages.yml` on both CI workflows — it currently deploys to
      production Pages on every push to `main` with no test gate.

## Priority 9 — Documentation truth

- [ ] **Rewrite `AGENTS.md`.** It is loaded into every AI agent session via `CLAUDE.md` and is
      wrong on every structural claim: "Currently at Phase 6", "the React/TypeScript frontend is
      deferred", "`engine/` — not yet implemented", "`standards/` — not yet implemented",
      "`scripts/` CI/build automation" (contains only `.gitkeep`), a package table missing 14
      packages, and 7 CLI commands where `root.go:90-100` registers 10.
- [ ] Fix `README.md`: it references `goal.md`, which does not exist; lists ISO 9613-2,
      GeoPackage, FlatGeobuf, CSV and GeoTIFF as "deferred" though all five ship; omits
      `aconiq compare`, `aconiq bench`, and two API endpoints.
- [ ] Make `PLAN.md` the single status source and have `AGENTS.md` link to it rather than restate it.
- [ ] Resolve the two competing lint stacks. `.trunk/trunk.yaml` is invoked by nothing yet holds
      `osv-scanner`, `trivy`, `trufflehog`, `checkov`, `gokart`, `actionlint` — exactly the security
      coverage CI lacks — and pins `go@1.21.0` against a `go 1.25.0` module. Either promote those
      scanners into `go-ci.yml` or drop trunk.
- [ ] Stop claiming "all linters enabled". `AGENTS.md` and `README.md` both say it; `.golangci.yml`
      disables roughly 20. "Defaults plus tuned disables" is accurate and just as short.
- [ ] `.golangci.yml` disables `wrapcheck` for all of `internal/` — the whole backend — so the
      error-wrapping policy is unenforced. Removing the exclusion costs 190 findings, so sequence
      it after the `domain/errors` work in Priority 7; a `FIXME(lint-triage)` in the config carries
      the count. (The `sqlclosecheck` half of this item is done: the "no SQL in this project"
      claim was false, and the linter is re-enabled at 0 findings — `0a13e80`.)

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
      `docs/policies/determinism.md` §3 to match what is implemented (see P1.8).

### UX and workflow questions

- [ ] Define DTO-generation strategy and backward-compatibility policy.
- [ ] Define which exports are must-have versus deferred — GeoTIFF, CSV, PNG, report artifacts.
- [ ] Define map-layer performance thresholds and tile-fallback triggers.
- [ ] Define the accessibility baseline for map-heavy interactions.
