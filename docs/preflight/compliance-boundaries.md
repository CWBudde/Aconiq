# Compliance Boundaries

Status date: 2026-03-06

## Scope Categories

1. Code (our implementation)
- License target: permissive open-source license (to be finalized before first public release).
- Dependencies must be tracked with version and license metadata.
- No dependency with unknown license status is allowed in main branch.

2. Standards texts and annexes
- Many normative documents are paywalled or redistribution-restricted.
- The code may implement algorithms from lawfully obtained standards, but cannot embed full normative text unless redistribution rights are explicit.
- Internal references to formulas must avoid verbatim reproduction of restricted content.

3. Test and acceptance data
- Public, license-safe test data is preferred and should be versioned in-repo.
- Restricted test suites (for example vendor or authority-provided packs) must be stored outside the public repo and referenced by local path/config.
- CI must not depend on private datasets for required checks in public branches.

4. Generated artifacts and examples
- Example projects in this repo must be synthetic or clearly redistribution-safe.
- No third-party basemap or dataset is vendored unless terms permit it.

## Immediate Actions

- Finalize project LICENSE choice before publishing binaries.
- Add dependency license scanning in Phase 2 CI.
- Keep legal notes per standards module when implementation begins.
