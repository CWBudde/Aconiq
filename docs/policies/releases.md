# Release Policy

Status date: 2026-08-28

Goal: a result a third party holds must be traceable to an identified build, and the version number
attached to that build must tell them whether their archived numbers still stand.

## What a version covers

A single version number covers the whole repository. `backend/` and `frontend/` are two build
targets of one product and are tagged together.

The version is a promise about these surfaces:

1. The CLI surface — command names, flags, argument shapes, exit codes, and the structure of
   `--json` output.
2. Project format v1 — the layout of `.noise/`, the manifest schema, and the migration path between
   manifest versions (`docs/project-format-v1.md`, `docs/project-migrations.md`).
3. The input schemas — the GeoJSON v1 vocabulary (`docs/geojson-schema-v1.md`), including the
   per-standard property vocabularies layered on it, and the accepted import formats.
4. The result containers — raster binary/metadata and receiver table CSV/JSON
   (`docs/result-containers-v1.md`), and the provenance and run-summary keys.
5. The local HTTP API v1 and its OpenAPI document.
6. **The computed levels produced by `normative`-tier standards modules.** See below.

The version is explicitly **not** a promise about:

- Any Go package under `backend/internal/`. The module path exists so the code compiles, not so
  third parties import it; there is no exported-API stability guarantee.
- The `window.aconiq` surface of the WebAssembly kernel, which tracks the frontend it serves.
- The numbers produced by `preview`- or `scaffold`-tier modules. See below.
- Anything under `docs/`, except that a conformance declaration is expected to match the release it
  ships with.

## Semantic versioning, applied to this project

Standard SemVer, with one addition that is specific to a computation tool.

### MAJOR

- A breaking change to any surface in the list above.
- **A change to a `normative`-tier module's computed levels.** This is the addition. An archived run
  is evidence: someone has a `provenance.json`, a receiver table and possibly a permit application
  resting on them. A corrected coefficient invalidates those numbers just as thoroughly as a removed
  CLI flag invalidates a script, and it does so silently — no signature moved, nothing fails to
  compile, the same command produces different dB. It is therefore a breaking change, and it is
  announced as one.
  - This holds regardless of direction or size. A fix that moves a level by 0.1 dB is still a
    change in the value the tool asserts.
  - It holds for changes that are unambiguously corrections. "The old number was wrong" is the
    reason for the bump, not an exemption from it.
  - It does not hold for a change that provably moves no computed value — a refactor, a faster
    reduction with identical output, a new indicator alongside the existing ones. The golden-snapshot
    gate below is what turns "provably" into something checkable.

### MINOR

- New commands, flags, import or export formats, standards modules, or API endpoints, added
  compatibly.
- A new standards module, a new version or profile within an existing module, or a new indicator —
  provided the existing ones do not move.
- Any change to a `preview`-tier module's computed levels. `preview` says the aggregation logic is
  reasonable but its inputs are not normative; the numbers are expected to move as the inputs are
  replaced.

### PATCH

- Fixes and improvements that move no computed level on any `normative`- or `preview`-tier module:
  crash fixes, parser hardening, error messages, performance, reporting and export defects,
  documentation.
- Any change to a `scaffold`- or `test-fixture`-tier module's numbers. Those tiers carry **no
  numeric stability promise at all** — their base levels are invented, they implement no directive
  coefficients, and `aconiq run` refuses them without `--experimental` precisely so that nobody can
  archive their output as an assessment. A scaffold's dB(A) values may change in any release,
  including a patch, without further notice than a changelog line.

### Before 1.0.0

While the version is `0.y.z`, everything above shifts down one place: a change that would be MAJOR
becomes a MINOR bump (`0.4.0` → `0.5.0`), and a change that would be MINOR or PATCH becomes a PATCH
bump. SemVer §4 permits anything to change at `0.y.z`; this rule narrows that so the numeric
promise is still legible before 1.0.

The classification does not change — a normative numeric change is still recorded and announced as
a breaking change in `CHANGELOG.md`, it just moves the minor digit rather than the major one.

1.0.0 is reserved for the point at which at least one normative module has published validation
evidence against an independent reference and the local API's threat model is closed. Until then,
`0.y.z` is the honest signal.

## Tags

- One annotated, signed-if-possible git tag per release, on `main`, named `vMAJOR.MINOR.PATCH`
  (`v0.1.0`). Pre-releases use the SemVer form: `v0.1.0-rc.1`.
- Nothing else is tagged. There are no per-module or per-standard tags.
- The tag is the release trigger: `.github/workflows/release.yml` fires on `v*` and is the only
  thing that publishes binaries.

What the tag drives:

- `just build` computes `git describe --tags --dirty --always` and stamps it into
  `internal/buildinfo` via `-ldflags -X`, together with the short commit and a UTC build date. The
  release build stamps the same three variables from the tag.
- `buildinfo` feeds `aconiq --version` and the `tool_version` field of every `provenance.json`. So
  the first tag is what turns provenance from `v0.0.0-…-gabc123` guesswork into a statement a
  reader can resolve to a published artifact.
- Because `git describe --tags` is useless on a shallow clone, any workflow that builds a stamped
  binary checks out with `fetch-depth: 0` and `fetch-tags: true`. Both `go-ci.yml` and
  `release.yml` do.

## Changelog discipline

`CHANGELOG.md` at the repo root, [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

- Every change that is visible to a user of the CLI, the API or the project format adds an entry
  under `## [Unreleased]` **in the same commit that makes the change**. The changelog is not
  reconstructed from `git log` at release time; this repository's commit bodies are detailed enough
  that reconstruction feels possible, and it is exactly that temptation the rule exists to remove.
- Entries are grouped `Added` / `Changed` / `Deprecated` / `Removed` / `Fixed` / `Security`, plus a
  `Known limitations` section this project keeps because the evidence tiers make it necessary.
- Any entry that moves a computed level is prefixed **numeric** and names the affected module and
  the size of the shift. That prefix is the input to the MAJOR/MINOR decision above, so it is not
  optional.
- Releasing means renaming `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`, opening a fresh empty
  `Unreleased`, and updating the link definitions at the bottom. A released section is never edited
  afterwards; a mistake in one is corrected by an entry in the next release.
- `CHANGELOG.md` is excluded from treefmt in `treefmt.toml`, so its formatting is the author's
  responsibility.

## Release checklist

Run from a clean tree on `main`, in order.

1. `just ci` passes — the full Go gate plus the frontend gate.
2. **Golden-snapshot gate.** Run `just update-golden` and confirm `git status` reports no change.
   Any golden that moves is a computed value that moved, and one of two things is true:
   - The move is intended. It must already have a **numeric** changelog entry, and the version bump
     must be the one that entry's tier calls for. If it does not, the release stops here.
   - The move is not intended. It is a regression that reached `main` and the release stops
     outright.

   This gate is the entire mechanism by which the "normative numbers are a breaking change" rule is
   enforced. Nothing else in CI can distinguish a refactor from a silent recalibration.

3. Confirm the acceptance suites produced evidence rather than skipping. Run with
   `ACONIQ_STRICT_ACCEPTANCE=1` so a suite that skipped everything fails instead of reporting
   `passed`.
4. Check that every `docs/conformance/` document still describes what the tree does, and that
   `PLAN.md`'s evidence-tier table matches the tiers the registry actually declares. A conformance
   declaration that outran the code is the worst defect this project can ship.
5. Move `Unreleased` to the new version heading in `CHANGELOG.md`; commit.
6. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z"` and push the tag. The workflow builds, checksums and
   publishes.
7. Verify the published artifact: download one binary, run `aconiq --version`, and confirm it
   reports the tag rather than `dev` — the stamp is the thing being released, so it is the thing
   worth checking by hand.

## Published artifacts

`.goreleaser.yaml` defines them; that file is the source of truth and this list is a summary.

- `aconiq` for linux, darwin and windows on `amd64` and `arm64`. All six are built with
  `CGO_ENABLED=0`: the only dependency that would otherwise need a C toolchain is
  `modernc.org/sqlite`, which is a pure-Go translation, so every target cross-compiles from one
  Linux runner and the binaries are statically linked and dependency-free at the destination. That
  is the property an offline-first tool needs.
- The `js/wasm` compute kernel, as its own archive. It is the same engine the browser UI runs. The
  matching `wasm_exec.js` is not shipped: it belongs to the Go toolchain, and the archive names the
  toolchain version it was built with so a consumer can take the right one.
- `checksums.txt` — SHA-256 over every archive.
- `LICENSE`, `NOTICE`, `README.md`, `CHANGELOG.md`, `SECURITY.md` and `docs/conformance/` travel
  inside each archive. The conformance and scope declarations are what tell the recipient what the
  binary's numbers are worth, so they ship with the binary rather than only on the web.

## Supported versions

Only the latest release receives fixes. There are no maintenance branches and no backports; a fix
ships in the next release from `main`. `SECURITY.md` states the same thing and is the authoritative
version of it for security reports.
