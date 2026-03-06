# Definition of Done (Phase Baseline)

Status date: 2026-03-06

A phase is done only when all planned items for that phase are complete and the baseline conditions below are met.

## Baseline DoD for Every Phase

1. Scope completeness
- All checklist items for the phase are checked in `PLAN.md`.
- Deferred items are explicitly moved to a later phase with rationale.

2. Build and test health
- Relevant automated tests pass locally and in CI (if CI exists for that phase).
- No known failing tests are introduced by the phase.

3. Determinism and reproducibility
- Outputs are reproducible for unchanged inputs/configuration.
- Any numeric tolerance choices are documented.

4. Documentation and traceability
- User/developer docs affected by the phase are updated.
- New commands, configs, and data formats are documented.

5. Quality and operability
- Errors are actionable for users (clear messages and exit behavior).
- Logging and artifacts are sufficient to debug failures.

6. Compliance and data hygiene
- New dependencies have acceptable licenses.
- Added datasets/examples are redistribution-safe.

## Phase-Specific Completion Rule

Each phase can add stricter criteria, but cannot weaken the baseline DoD above.
