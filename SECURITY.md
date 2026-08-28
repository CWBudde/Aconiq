# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in Aconiq, please report it responsibly.

**Do not open a public issue.** Instead, send details to the maintainers via a [GitHub Security Advisory](https://github.com/cwbudde/Aconiq/security/advisories/new) or by contacting the repository owner directly.

Please include:

- A description of the vulnerability and its potential impact.
- Steps to reproduce or a proof of concept.
- The version affected — see below for how to identify it.

We will acknowledge receipt within 5 business days and aim to provide a fix or mitigation plan within 30 days.

## Identifying your version

Run:

```bash
aconiq --version
```

It prints the release version, the source commit and the build date, for example
`aconiq v0.1.0 (commit 0cfb763a1b2c, built 2026-08-28T10:00:00Z)`. Quote that line verbatim in your
report.

Two notes on what you may see:

- A binary built from a working tree rather than from a release reports a `git describe` string such
  as `v0.1.0-7-g0cfb763`, `v0.1.0-7-g0cfb763-dirty`, or — with no version information available at
  all — `dev`. In those cases the commit field is what identifies the build, so please include it.
- Every run also records the same identity in `.noise/runs/<run-id>/provenance.json` under
  `tool_version`. If you are reporting against a stored result rather than a live binary, that file
  is the authoritative answer.

## Scope

Aconiq is a CLI-first, offline-first application. The primary attack surface is:

- Local file parsing (GeoJSON, GeoPackage, CityGML, GeoTIFF, CSV, FlatGeobuf, SoundPLAN project
  bundles). These are untrusted third-party files by assumption.
- The local HTTP API when `aconiq serve` is running (localhost-only by default), including the
  browser UI that talks to it.
- Dependencies (Go modules).

Known and already-tracked weaknesses in these areas are recorded as open work in `PLAN.md`. A report
that restates one of them is welcome but will be handled as a duplicate; a report that shows one of
them is worse than recorded, or reachable in a way we have not written down, is not.

## Supported versions

Security fixes are applied to the latest release only. There are no maintenance branches and no
backports: a fix lands on `main` and ships in the next release from `main`.

"The latest release" means the most recent `vX.Y.Z` tag published at
[Releases](https://github.com/cwbudde/Aconiq/releases). Pre-releases (`vX.Y.Z-rc.N`) are not
supported; upgrade to the corresponding final release.

**Until the first tag exists, there is no released version.** In that window the supported version is
the current `main`, fixes are delivered as commits rather than as releases, and a report should
identify the affected build by the commit field of `aconiq --version`. The versioning and release
process that ends this window is documented in [`docs/policies/releases.md`](docs/policies/releases.md).
