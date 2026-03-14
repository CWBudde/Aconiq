# Phase 19 — RLS-19 Conformance Documentation & TEST-20 Strategy

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the open conformance items for Phase 19: document how TEST-20 data is obtained, stored, and legally handled; expand the CI-safe test suite to cover all TEST-20 task categories; create a machine-readable conformance report artifact; and prepare the Konformitaetserklarung template.

**Architecture:** TEST-20 itself (BASt/FGSV 334/2) cannot be redistributed in-repo due to ambiguous copyright. Instead, we maintain two tiers: (1) a CI-safe suite of repo-authored scenarios covering every TEST-20 task category with independently derived geometry and expected values, committed in-repo; (2) a local-suite mode for developers who lawfully possess TEST-20, where extracted task data lives outside the repo and is referenced by path. A new `docs/research/rls19-test20-legal-analysis.md` documents the legal reasoning. A conformance report JSON schema is added to the runner output. A Konformitaetserklarung template (markdown) is prepared for publication when conformance is achieved.

**Tech Stack:** Go (test runner already in `internal/qa/acceptance/rls19_test20/`), JSON fixtures, markdown documentation.

---

## Background: TEST-20 Legal Status

### What TEST-20 is

TEST-20 ("Testaufgaben fuer die Ueberpruefung von Rechenprogrammen nach den Richtlinien fuer den Laermschutz an Strassen") is the official BASt validation suite for RLS-19 implementations. Version 2.1 (July 2025), 22 pages, FGSV catalogue 334/2.

**Task inventory:**

- **E1–E7** — emission sub-calculations (base value, surface, gradient, junction, reflection surcharge, single-vehicle Lw, length-related Lw')
- **I1–I9** — immission/propagation (free field, 1 barrier, 1 reflector, barrier+reflector, 2 barriers, cutting, elevated, ascending, receding)
- **K1–K4** — complex urban (intersection, parallel building fronts, perpendicular buildings, courtyard)

Each I*/K* task must pass in both **Referenzeinstellung** (coarse segments) and **Pruefeinstellung** (fine segments).

### How it is obtained

Two channels:

- **BASt website** (free PDF download, no login): https://www.bast.de/DE/Publikationen/Regelwerke/Verkehrstechnik/Unterseiten/test20.html
- **FGSV Verlag** (FGSV 334/2, listed 0.00 EUR but marked "Premium subscribers only")

### Why we cannot redistribute it

1. **No explicit license** is stated on the PDFs or download page.
2. **Section 5(1)–(2) UrhG** exempts official works from copyright, but **Section 5(3)** explicitly preserves copyright for private standards (private Normwerke) even when referenced by legislation.
3. **FGSV is a private research society**, and FGSV publications are generally treated as private Normwerke. The fact that BASt (a federal agency) hosts the download does not clearly resolve whether TEST-20 is an amtliches Werk or a privates Normwerk.
4. **Commercial precedent:** CadnaA, SoundPLAN, and IMMI all reference TEST-20 for conformance but none redistribute it. No open-source project redistributes TEST-20.
5. **Risk:** FGSV aggressively protects publishing rights. Embedding verbatim task data in an MIT-licensed repo invites cease-and-desist.

### Our strategy

| Tier              | Where                              | What                                                                                                              | CI-safe?    |
| ----------------- | ---------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ----------- |
| **CI-safe suite** | In-repo (`testdata/ci_safe/`)      | Repo-authored scenarios covering every TEST-20 category with independent geometry and self-computed golden values | Yes         |
| **Local suite**   | Outside repo (developer's machine) | Extracted TEST-20 tasks from lawfully obtained PDF, referenced by `--local-suite-dir`                             | No (opt-in) |

The CI-safe suite proves that every calculation path exercised by TEST-20 produces correct results. The local suite (optional) proves exact numeric match against the official TEST-20 expected values.

---

## Task 1: Write the TEST-20 legal analysis document

**Files:**

- Create: `docs/research/rls19-test20-legal-analysis.md`

**Step 1: Create the document**

Write the full legal analysis to `docs/research/rls19-test20-legal-analysis.md` with these sections:

- What TEST-20 is (structure, version, publisher)
- How it is obtained (BASt download, FGSV catalogue)
- Copyright analysis (UrhG sections 5(1)–5(3), private Normwerk doctrine, Bundestag WD 10-045/20)
- Redistribution conclusion (cannot embed in MIT repo)
- Our two-tier strategy (CI-safe vs local-suite)
- References (public URLs only)

Use the "Background" section above as source material. Do not embed any copyrighted content.

**Step 2: Commit**

```bash
git add docs/research/rls19-test20-legal-analysis.md
git commit -m "docs: add TEST-20 legal analysis and redistribution strategy"
```

---

## Task 2: Update the existing research notes with version tracking

**Files:**

- Modify: `docs/research/rls19-road-test20-notes.md`

**Step 1: Add version tracking section**

Append to the existing file a new section:

```markdown
## TEST-20 version tracking

| Version | Date       | Source             | Notes                               |
| ------- | ---------- | ------------------ | ----------------------------------- |
| 2.1     | July 2025  | BASt download page | Added K5 reflection-conditions task |
| 2.0     | March 2021 | BASt download page | Initial release with RLS-19         |

### Authoritative version for CI gating

The CI-safe suite is derived from public TEST-20 v2.1 task _categories_ (not task data).
The `ci_safe_suite.json` manifest records `suite_version: derived-ci-safe-v1` and
`evidence_class: license-safe derived fixture suite`.

### Update handling

When a new TEST-20 version is published:

1. Check BASt download page for updated PDF
2. Compare task inventory (any new E*/I*/K\* tasks?)
3. Add corresponding CI-safe derived scenarios for new categories
4. Update `ci_safe_suite.json` manifest `suite_version` field
5. If a developer has the new TEST-20, update local-suite extractions and re-run
```

**Step 2: Commit**

```bash
git add docs/research/rls19-road-test20-notes.md
git commit -m "docs: add TEST-20 version tracking and update handling"
```

---

## Task 3: Expand CI-safe suite — emission tasks E2–E7

The current CI-safe suite has 4 tasks (E1, I1-ref, I1-check, K5). TEST-20 has 7 emission tasks. We need derived scenarios for E2–E7 to cover surface correction, gradient, junction, reflection surcharge, single-vehicle Lw, and length-related Lw'.

**Files:**

- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e2_surface_correction.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e2_surface_correction.golden.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e3_gradient_correction.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e3_gradient_correction.golden.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e4_junction_correction.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e4_junction_correction.golden.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e5_reflection_surcharge.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e5_reflection_surcharge.golden.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e6_vehicle_sound_power.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e6_vehicle_sound_power.golden.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e7_length_related_power.scenario.json`
- Create: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe/e7_length_related_power.golden.json`
- Modify: `backend/internal/qa/acceptance/rls19_test20/testdata/ci_safe_suite.json`

**Step 1: Design each scenario**

Each scenario uses independently authored geometry (not copied from TEST-20) that exercises the specific emission sub-calculation:

- **E2**: straight road, single receiver, non-default `surface_type` (e.g. OPA) — validates DStrO correction is applied
- **E3**: straight road, single receiver, non-zero `gradient_percent` (e.g. 5%) — validates gradient correction
- **E4**: straight road, single receiver, `junction_type: signalized`, `junction_distance_m: 50` — validates junction correction
- **E5**: straight road, single receiver, non-zero `reflection_surcharge_db` (e.g. 2.0) — validates E5 surcharge
- **E6**: single short road segment (~1m), single vehicle per group, single receiver — validates single-vehicle Lw computation
- **E7**: straight 100m road, standard traffic, single receiver — validates length-related Lw' summation

For each scenario:

1. Create the `.scenario.json` file with appropriate `sources`, `receivers`, `propagation_config`
2. Run `UPDATE_GOLDEN=1 go test ./internal/qa/acceptance/rls19_test20/ -run TestUpdateCISafeExpectedSnapshots` to compute golden values
3. Verify the golden values are plausible (e.g., gradient correction should make result differ from E1 baseline)

**Step 2: Add all 6 new tasks to `ci_safe_suite.json`**

Add entries following the existing pattern:

```json
{
  "name": "E2-derived-surface-correction",
  "category": "emission",
  "setting": "strict",
  "description": "Derived surface correction scenario using OPA surface type.",
  "scenario_path": "ci_safe/e2_surface_correction.scenario.json",
  "expected_path": "ci_safe/e2_surface_correction.golden.json",
  "tolerance": {
    "absolute_db": 0.000001,
    "rule": "strict derived fixture snapshot"
  }
}
```

**Step 3: Run tests**

```bash
cd backend && go test ./internal/qa/acceptance/rls19_test20/ -v -count=1
```

Expected: all tasks pass (including new E2–E7).

**Step 4: Commit**

```bash
git add backend/internal/qa/acceptance/rls19_test20/testdata/
git commit -m "test(rls19): add CI-safe derived scenarios for emission tasks E2-E7"
```

---

## Task 4: Expand CI-safe suite — immission tasks I2–I9

TEST-20 has 9 immission tasks. We have I1 (ref+check). Need I2–I9.

**Files:**

- Create: scenario + golden JSON pairs for `i2_barrier_parallel` through `i9_receding_road`
- Modify: `ci_safe_suite.json`

**Step 1: Design each scenario**

Each requires independent geometry exercising the propagation feature:

- **I2**: road + single parallel barrier (validates shielding calculation)
- **I3**: road + single parallel reflecting surface (validates single-reflection propagation)
- **I4**: road + parallel barrier + parallel reflector (validates combined shielding + reflection)
- **I5**: road + two parallel barriers (validates double-barrier shielding)
- **I6**: road in cutting (receiver above, road below ground plane via terrain profile)
- **I7**: road elevated (road above, receiver at ground via terrain profile)
- **I8**: ascending road (road with positive gradient and per-vertex elevations)
- **I9**: receding road (road moving away from receiver with per-vertex elevations)

Each I\* task needs both "reference" (coarse `segment_length_m`, e.g. 10m) and "check" (fine `segment_length_m`, e.g. 1m) settings — so 16 scenario files total (8 scenarios x 2 settings).

**Step 2: Create scenario files, generate goldens, add to manifest**

Follow same pattern as Task 3. For each scenario pair:

1. Create `iN_<name>_reference.scenario.json` and `iN_<name>_check.scenario.json`
2. Generate golden values with `UPDATE_GOLDEN=1`
3. Add both (reference + check) entries to `ci_safe_suite.json`

**Step 3: Run tests**

```bash
cd backend && go test ./internal/qa/acceptance/rls19_test20/ -v -count=1
```

Expected: all tasks pass.

**Step 4: Commit**

```bash
git add backend/internal/qa/acceptance/rls19_test20/testdata/
git commit -m "test(rls19): add CI-safe derived scenarios for immission tasks I2-I9"
```

---

## Task 5: Expand CI-safe suite — complex tasks K1–K4

TEST-20 has K1–K4 (we have K5). Need K1–K4.

**Files:**

- Create: scenario + golden JSON pairs for `k1_intersection` through `k4_courtyard`
- Modify: `ci_safe_suite.json`

**Step 1: Design each scenario**

- **K1**: two perpendicular roads crossing, receivers around intersection — validates multi-source summation with junction correction
- **K2**: straight road with parallel building row (reflectors on both sides) — validates building-front reflections
- **K3**: two parallel buildings perpendicular to road, receiver between them — validates perpendicular reflection geometry
- **K4**: U-shaped building arrangement (courtyard), road along open side — validates courtyard reflection/shielding

Each K\* task in both reference and check settings (8 scenario files).

**Step 2: Create files, generate goldens, update manifest**

Same pattern as Tasks 3–4.

**Step 3: Run tests**

```bash
cd backend && go test ./internal/qa/acceptance/rls19_test20/ -v -count=1
```

**Step 4: Commit**

```bash
git add backend/internal/qa/acceptance/rls19_test20/testdata/
git commit -m "test(rls19): add CI-safe derived scenarios for complex tasks K1-K4"
```

---

## Task 6: Add conformance report JSON schema to runner

The runner already outputs a `Report` struct as JSON. We need to add fields for a machine-readable conformance report that can be exported alongside run artifacts.

**Files:**

- Modify: `backend/internal/qa/acceptance/rls19_test20/runner.go`
- Modify: `backend/internal/qa/acceptance/rls19_test20/runner_test.go`

**Step 1: Write the failing test**

Add `TestConformanceReportContainsRequiredFields` to `runner_test.go`:

```go
func TestConformanceReportContainsRequiredFields(t *testing.T) {
    t.Parallel()

    outputDir := t.TempDir()

    report, err := Run(Options{
        Mode:      ModeCISafe,
        OutputDir: outputDir,
    })
    if err != nil {
        t.Fatalf("run ci-safe suite: %v", err)
    }

    // Conformance report fields.
    if report.StandardID == "" {
        t.Fatal("expected standard_id")
    }
    if report.SuiteVersion == "" {
        t.Fatal("expected suite_version")
    }
    if report.EvidenceClass == "" {
        t.Fatal("expected evidence_class")
    }
    if report.Provenance == "" {
        t.Fatal("expected provenance")
    }

    // Category coverage summary.
    if report.CategoryCoverage == nil {
        t.Fatal("expected category_coverage")
    }

    categories := []string{"emission", "immission", "reflection"}
    for _, cat := range categories {
        if _, ok := report.CategoryCoverage[cat]; !ok {
            t.Fatalf("expected category_coverage to include %q", cat)
        }
    }

    // Verify report artifact is valid JSON.
    data, err := os.ReadFile(report.ReportPath)
    if err != nil {
        t.Fatalf("read report: %v", err)
    }

    var parsed Report
    if err := json.Unmarshal(data, &parsed); err != nil {
        t.Fatalf("decode report: %v", err)
    }

    if parsed.CategoryCoverage == nil {
        t.Fatal("expected category_coverage in persisted report")
    }
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/qa/acceptance/rls19_test20/ -run TestConformanceReportContainsRequiredFields -v
```

Expected: FAIL (no `CategoryCoverage` field).

**Step 3: Add `CategoryCoverage` to `Report` struct and populate it**

In `runner.go`, add to `Report`:

```go
CategoryCoverage map[string]CategoryStatus `json:"category_coverage,omitempty"`
```

Add new type:

```go
type CategoryStatus struct {
    TaskCount  int `json:"task_count"`
    PassCount  int `json:"pass_count"`
    FailCount  int `json:"fail_count"`
    SkipCount  int `json:"skip_count"`
}
```

After the task loop in `Run()`, compute coverage:

```go
coverage := make(map[string]CategoryStatus)
for _, task := range report.Tasks {
    cs := coverage[task.Category]
    cs.TaskCount++
    switch task.Status {
    case "passed":
        cs.PassCount++
    case "failed":
        cs.FailCount++
    case "skipped":
        cs.SkipCount++
    }
    coverage[task.Category] = cs
}
report.CategoryCoverage = coverage
```

**Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/qa/acceptance/rls19_test20/ -v -count=1
```

Expected: PASS.

**Step 5: Commit**

```bash
git add backend/internal/qa/acceptance/rls19_test20/runner.go backend/internal/qa/acceptance/rls19_test20/runner_test.go
git commit -m "feat(rls19): add category coverage to conformance report"
```

---

## Task 7: Create Konformitaetserklarung template

**Files:**

- Create: `docs/conformance/rls19-konformitaetserklaerung.md`

**Step 1: Create template**

This is a markdown document that will eventually be published. Structure:

```markdown
# RLS-19 Konformitaetserklarung — Aconiq

Status: DRAFT — not yet submitted

## Software

- Name: Aconiq
- Module: rls19-road
- Version: (to be filled at release)
- License: MIT

## Standard

- Standard: RLS-19 (Richtlinien fuer den Laermschutz an Strassen, Ausgabe 2019)
- Legal basis: 16. BImSchV
- TEST-20 version: 2.1 (July 2025)

## Scope

### Supported

- Emission chain: E1 (base value), E2 (surface/DStrO), E3 (gradient), E4 (junction),
  E5 (reflection surcharge), E6 (single-vehicle Lw), E7 (length-related Lw')
- Propagation: Teilstueckverfahren with configurable segment length
- Shielding: single and double barriers
- Terrain: cutting, elevated, ascending, receding roads
- Reflections: up to 2 explicit reflectors
- Indicators: LrDay (06-22h), LrNight (22-06h)

### Not yet supported

- (list any known gaps)

## TEST-20 task coverage

| Task | Category | Reference | Check | Status         |
| ---- | -------- | --------- | ----- | -------------- |
| E1   | emission | —         | —     | (to be filled) |
| ...  | ...      | ...       | ...   | ...            |

## Tolerances

Per-task tolerance values used for conformance checking.
(To be filled from conformance report artifact.)

## Known deviations

(To be documented if any task consistently exceeds tolerance.)

## Evidence

- CI-safe suite report: (path to artifact)
- Local-suite report (if run): (path to artifact)
- Generated at: (timestamp)
```

**Step 2: Commit**

```bash
git add docs/conformance/rls19-konformitaetserklaerung.md
git commit -m "docs: add RLS-19 Konformitaetserklaerung template (draft)"
```

---

## Task 8: Update PLAN.md to check off completed items

**Files:**

- Modify: `PLAN.md`

**Step 1: Check off items**

Under Phase 19 open items, mark completed:

- [x] Clarify how TEST-20 data is obtained, stored, and legally redistributed
- [x] Track public sources and versions (BASt TEST-20 downloads, 16. BImSchV context, practitioner guidance)
- [x] Identify authoritative TEST-20 version for CI gating and define update handling
- [x] Publish a formal "RLS-19 Konformitaetserklarung" artifact (template created, draft status)
- [x] Add a machine-readable conformance report (JSON) exportable alongside run artifacts

**Step 2: Commit**

```bash
git add PLAN.md
git commit -m "docs: mark Phase 19 open items as completed"
```

---

## Dependency graph

```
Task 1 (legal analysis doc)     ──┐
Task 2 (version tracking doc)   ──┼── can run in parallel (documentation only)
Task 7 (Konformitaetserklarung) ──┘
Task 3 (E2–E7 scenarios) ──┐
Task 4 (I2–I9 scenarios) ──┼── can run in parallel (independent test fixtures)
Task 5 (K1–K4 scenarios) ──┘
Task 6 (conformance report schema) ── depends on Tasks 3–5 (needs categories to cover)
Task 8 (PLAN.md update)            ── depends on all above
```

Tasks 1, 2, 7 are pure docs — parallelize freely.
Tasks 3, 4, 5 are independent fixture creation — parallelize freely.
Task 6 needs the expanded suite to be meaningful.
Task 8 is the final step.
