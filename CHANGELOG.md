# Changelog

All notable changes to Aconiq are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html) as scoped in
[`docs/policies/releases.md`](docs/policies/releases.md) — which, for this project, treats a change
in a normative standards module's computed levels as a breaking change even when no signature moved.

**Aconiq has never been released.** There are no tagged versions and no published binaries yet, so
everything below sits under `Unreleased` and describes the state a first tag would capture. Nothing
here has been back-dated into an invented release.

Because there is no prior release, the `Fixed` entries below correct defects that were only ever
reachable from a source build. They are listed anyway: several of them move computed dB values, and
anyone who ran an earlier working tree holds results those changes invalidate.

## [Unreleased]

### Added

- `aconiq` CLI with ten commands: `init`, `import`, `validate`, `run`, `compare`, `status`,
  `export`, `serve`, `openapi` and `bench`.
- Project format v1: a `.noise/` folder holding the manifest, per-run logs, `provenance.json`,
  result tables and rasters, export bundles and the engine cache.
- Model import from GeoJSON, GeoPackage, FlatGeobuf, CityGML, CSV attribute/traffic tables,
  OpenStreetMap via Overpass, SoundPLAN project bundles, and GeoTIFF terrain.
- Standards registry exposing thirteen standard IDs. Three carry real normative structure and
  coefficients (`rls19-road`, `schall03`, `iso9613`); the rest are preview or scaffold grade — see
  `Evidence tiers` below and `docs/conformance/`.
- German assessment layers: 16. BImSchV threshold assessment (emitted into export bundles) and
  TA Lärm Beurteilungspegel (library only, not yet wired to the CLI).
- Export formats: GeoTIFF, Cloud-Optimized GeoTIFF, GeoPackage, and contour GeoJSON/GeoPackage.
- Offline report generation: `report-context.json`, `report.md`, `report.html`, `report.typ` and an
  optional PDF.
- Local HTTP API (`aconiq serve`, default `127.0.0.1:8080`) with a hand-built OpenAPI v1 document,
  a standardized JSON error envelope, and an SSE event stream.
- WebAssembly compute kernel (`backend/cmd/wasm`), letting the browser UI run without the HTTP API.
- React/TypeScript frontend (Vite, shadcn/ui, MapLibre) driving the local API or the WASM kernel.
- `aconiq --version`, plus real build identity — version, commit and build date — stamped at link
  time and recorded in every `provenance.json` instead of the previous hardcoded `"dev"`.
- `--json` output on every CLI command.
- Evidence tiers. Every standards module declares `normative`, `preview`, `scaffold` or
  `test-fixture`; a module that declares none cannot be registered. The tier is printed by `run` and
  `status`, stamped into `provenance.json` and `run-summary.json`, carried into generated reports and
  export bundles, and returned by `GET /api/v1/standards`.
- `aconiq run --experimental`. Scaffold-tier standards now refuse to run without it, rather than
  quietly emitting authoritative-looking dB(A) values. The API rejects the same case with
  `experimental_opt_in_required`. The frontend shows the tier as a badge and asks for a
  per-standard acknowledgement.
- Schall 03: the normative Anlage-2 chain is reachable from the CLI, selected by a new
  `schall03_engine` run parameter (`auto` by default, `preview` as an explicit opt-in that warns in
  `run.log`). Normative inputs arrive through a `schall03_*` GeoJSON vocabulary covering Zugart,
  Streckenhöchstgeschwindigkeit, Fahrbahnart, surface measure, bridge type, station and
  Langsamfahr flags, water fraction and wall surfaces.
- `aconiq compare --param`, so a comparison can select the same engine and options as the run it
  compares against.
- A standard-data digest in `provenance.json` and in generated reports: a SHA-256 over every
  coefficient table a standards module carries, per table and overall, with the evidence tier as an
  input to the hash. Two runs whose digests agree used byte-identical coefficient data; re-tiering a
  module moves the digest, because it changes what the numbers mean. It is a dedicated field, not an
  `input_hashes` entry — that map is input-file path to SHA-256, and a non-file entry there renders
  as a bogus "Input files" row in every report.
- Determinism policy is now testable rather than aspirational: reductions whose term count scales
  with model size use Neumaier compensated summation, and fixed-length reductions are documented as
  exempt (`docs/policies/determinism.md`).

### Changed

- The CLI was renamed from `noise` to `aconiq`.
- Scaffold-tier module descriptions state what they do **not** implement. `cnossos-*`, `bub-*` and
  `buf-aircraft` contain no coefficient from Directive (EU) 2015/996 Annex II and must not be
  described as implementations of CNOSSOS-EU or of the German mapping directives. Their scope
  statements live in `docs/conformance/cnossos-umfangserklaerung.md` and
  `beb-umfangserklaerung.md` — explicitly not conformance declarations.
- **Flag value change:** the Schall 03 standard version is renamed from `phase18-baseline-preview`
  to `2014-anlage2`. A script passing the old value to `aconiq run --version` must be updated. The
  bundled model versions of the preview and scaffold modules are likewise renamed from
  `phaseNN-preview-vN` to `baseline-preview-<subject>-vN`; those appear in `provenance.json` and
  `run-summary.json` rather than on the command line. Internal sprint numbers had been leaking into
  artifacts meant to be read by third parties, and no scaffold's model version now names a standard
  it does not implement.
- Run-summary provenance is unified on a single `model_version` key across every standard-backed
  persist path; RLS-19's `data_pack_version` was renamed to match.
- Schall 03 model versions are renamed away from internal sprint numbers:
  `phase20-normative-eisenbahn-strecke-v1` becomes `anlage2-2014-strecke-v1` on the normative path
  and `baseline-preview-datapack-v1` on the preview path. The data-pack version is recorded only on
  the path that opens one.
- **Input format break:** the Schall 03 `fahrbahn` and `s_fahrbahn` enums are renumbered so that
  Schwellengleis — the reference type carrying no `c1` correction — is the zero value. A model that
  omitted the property previously collected Feste Fahrbahn (up to +7 dB) or the straßenbündig row
  (up to +8 dB) by accident.
- `bub-rail` and `bub-industry` are wired into the run pipeline. They were registered but
  unreachable, and fell into the dispatch switch's default branch.

### Fixed

Entries marked **numeric** change computed levels. Results produced before them are not comparable
with results produced after them.

- **numeric** ISO 9613-2: A-weighting was applied twice on the default import path (broadband
  `L_WA` replicated into all eight octave bands, then energy-summed with a second weighting), worth
  about +7 dB. Clause 1 NOTE 1 is now implemented as a single 500 Hz band. Golden levels drop 5.8
  to 6.6 dB.
- **numeric** ISO 9613-2: the 63 Hz `A_m` column of Table 3 had the wrong sign (`+3q` instead of
  `-3q`), and `C_met` was computed once from the farthest source instead of per path. With
  `C_0 = 5 dB` and sources at 100 m and 1000 m the near source was over-corrected by 2.5 dB.
- **numeric** ISO 9613-2: a non-screening obstacle no longer enters Eq. 12, where it could turn a
  negative ground effect into a positive screening attenuation.
- **numeric** RLS-19: line-source segments are weighted against `l0 = 1 m` rather than the total
  road length.
- **numeric** Schall 03: ten normative defects, each verified against the 16. BImSchV Anlage 2
  text. `d0` in Gl. 14 was read as a path length instead of the 1 m reference length, which zeroed
  the only ground term at normal assessment distances; the Tabelle 6 Rollgeräusche speed-factor row
  was shifted one octave band (invisible at exactly 100 km/h, which is why the tests missed it);
  `c1` was double-counted on bridges against the Nr. 4.6 exclusion; `D_refl` was applied to every
  barrier instead of reflective walls within 5 m; reflected-path directivity used the mirror-image
  direction instead of the reflection point; lateral diffraction climbed over the barrier top
  instead of around a vertical edge; `e` was measured as a chord instead of a polyline; an
  energetic sum was computed in arithmetic dB; the Nr. 5.3.2 Straßenbahn substitute speed was
  applied to Eisenbahn segments; and the flow term computed `10 lg(n + 1)`, worth a spurious
  +3.0 dB at one train per hour and a full bare spectrum for a period with no trains at all.
- **numeric** `cnossos-road`, `cnossos-rail` and `bub-road` had the same `10 lg(Q + 1)` flow defect
  and now take an explicit zero-flow branch.
- **numeric** Schall 03 height summation iterated a map, so results could differ by roughly one ULP
  between runs — small, but a direct violation of the determinism policy. Pinned by a 500-run
  bit-identity test.
- `cnossos-road` run summaries carried `cnossos-industry` constants — both the model version and
  the reporting precision were wrong.
- The engine kept writing chunk files into the cache directory after a run had reported itself
  cancelled.
- Input hashing streams each file through SHA-256 instead of reading it whole, so a terrain raster
  larger than available memory hashes instead of exhausting it.
- The Schall 03 acceptance suite reported `passed` when every task had skipped, so a conformance
  claim could stop being checked without anyone noticing. An empty suite now always carries an
  explicit reason, and `ACONIQ_STRICT_ACCEPTANCE=1` escalates it to a failure.
- The frontend Run page: correctness defects behind the shipped page, 55 type errors and 49 lint
  findings that the previously non-checking gates had hidden.
- Dead links in `CONTRIBUTING.md` and `SECURITY.md` pointed at a repository that does not exist —
  including the security advisory URL, the one link a reporter needs.
- `just license-report` produced nothing usable: it misclassified the whole standard library
  whenever a `toolchain` directive moved `GOROOT` into the module cache, and only ever reported one
  `GOOS`, which is how three dependencies went missing from `NOTICE`.

### Security

- Every allocation the untrusted-file parsers make is bounded before it happens — GeoTIFF,
  GeoPackage/WKB, FlatGeobuf, GeoJSON, CityGML and SoundPLAN — with a regression test per bound and
  five fuzz targets. The GeoTIFF out-of-memory condition was reachable from an **86-byte** input,
  and a deflate bomb found alongside it was closed at the same time.
- The local HTTP API is no longer reachable from a hostile web page. A `Host` allowlist closes DNS
  rebinding, which an origin check cannot see; every endpoint that reads a body requires a real
  media type, closing the CORS simple-request path that let a cross-origin `text/plain` `fetch`
  reach the run executor and its request-controlled argv; and every state-changing method must
  carry an `X-Aconiq-Client` header, which a simple request cannot set, forcing a preflight. An
  opt-in bearer token (`aconiq serve --api-token`, env `ACONIQ_API_TOKEN`) sits on top for machines
  where other local processes are not trusted.
- Every API request body is bounded, per endpoint. The terrain upload's `ParseMultipartForm` was
  being handed the 50 MB *body* cap as its *maxMemory* argument, so a 50 MB upload was buffered
  whole in RAM; it now gets a real memory share, with the body capped separately.
- Arbitrary file read over HTTP is closed. Two handlers built paths from manifest values without
  containment, so a shared project whose manifest set `log_path: "../../../../etc/passwd"` served
  that file. Both now go through `filepath.IsLocal` plus `os.OpenInRoot`, which also defeats
  symlinks planted inside the project.
- `overpass_endpoint` is allowlisted. Taken from a request body with no validation, it made the API
  a general-purpose HTTP client for anyone who could reach it — internal addresses and cloud
  metadata services included.
- Three unbounded allocations in FlatGeobuf import, all reachable from a small file and all
  originating in `gogama/flatgeobuf` sizing `make` from unvalidated header and length fields: a
  **65-byte** input requested 1 TiB through `DataRem`, a **116-byte** input reached 4.2 GB RSS
  through `readFeature`'s 32-bit length prefix — roughly 36-million-fold amplification — and a
  property length prefix inside a valid file asked for 4 GiB through `PropReader.ReadBinary`. The
  parser now bounds every length against the bytes actually left in the file, and the header's
  feature count is an integrity check after the fact rather than an allocation size. All three were
  found by the new `FuzzReadFGB`.
- FlatGeobuf import survives a corrupt flatbuffer vtable. The pinned flatbuffers release ships no
  verifier, so a per-element guard converts a library fault into a typed error naming the feature
  index; it re-panics untouched if the fault came from Aconiq's own code.
- CSV import is bounded on columns, records and total properties. A CSV row costs a few bytes on
  disk and becomes one map entry per column, so the cost follows columns times rows and no single
  one of those limits bounds it.
- CI refuses any tracked path under `interoperability/`. Roughly 4 GB of licensed third-party
  material was protected by a single `.gitignore` line, which does not cover a force-added file, a
  file already tracked when the rule landed, or a branch that predates it.
- Reachable vulnerabilities in dependencies were cleared, and `govulncheck` now runs as a CI gate.
- Dependency license scanning (`just license-check`) fails the build on restricted, forbidden or
  unknown licenses.

### Known limitations

- The local HTTP API is unauthenticated by default. The browser-borne vectors are closed (see
  `Security` above), but any process on the same machine can reach it unless you start it with
  `--api-token`. `--listen 0.0.0.0:8080` now serves loopback only, because a wildcard bind names no
  host to add to the allowlist — bind the address you mean instead.
- `cnossos-*`, `bub-*` and `buf-aircraft` are scaffolds: no directive coefficients, invented base
  levels, no octave bands. They require `--experimental` and their output carries no accuracy
  claim.
- `beb-exposure` is preview grade: the aggregation is sound, but it consumes preview-grade levels.
- The SoundPLAN cross-check currently disagrees by roughly 25 dB mean on the Schall 03 preview
  chain. It is asserted rather than hidden, and it is an open defect.

[Unreleased]: https://github.com/cwbudde/Aconiq/commits/main
