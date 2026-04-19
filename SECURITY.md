# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in Aconiq, please report it responsibly.

**Do not open a public issue.** Instead, send details to the maintainers via a [GitHub Security Advisory](https://github.com/aconiq/backend/security/advisories/new) or by contacting the repository owner directly.

Please include:

- A description of the vulnerability and its potential impact.
- Steps to reproduce or a proof of concept.
- The version(s) affected, if known.

We will acknowledge receipt within 5 business days and aim to provide a fix or mitigation plan within 30 days.

## Scope

Aconiq is a CLI-first, offline-first application. The primary attack surface is:

- Local file parsing (GeoJSON, GeoPackage, CityGML, GeoTIFF, CSV, FlatGeobuf).
- The local HTTP API when `aconiq serve` is running (localhost-only by default).
- Dependencies (Go modules).

## Supported versions

Security fixes are applied to the latest release on `main`. There is no long-term support for older versions at this time.
