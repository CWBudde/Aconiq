# Determinism Policy

Status date: 2026-03-06

Goal: unchanged inputs, standard/profile, and runtime config must produce reproducible outputs.

## Floating-Point Rules

1. Operation order

- Aggregations must use a deterministic iteration order.
- Map iteration order must never influence numeric results.

2. Rounding and representation

- Keep internal calculations at float64 unless a standard explicitly requires another form.
- Apply rounding only at defined output boundaries.
- Record rounding rules in each standards module.

3. Summation strategy

- For numerically sensitive reductions, use a stable strategy (for example pairwise or compensated summation).
- Chosen strategy must be consistent across worker counts.

4. Tolerances

- Every comparison that uses tolerance must define and document the exact threshold.
- Tolerance constants belong in test/QA code, not hidden in production logic.

## Deterministic Parallel Reduction Strategy

1. Fixed partitioning

- Receiver/source chunking must be deterministic from immutable input ordering.

2. Stable reduction tree

- Partial results are merged in a fixed order independent of worker scheduling.
- No "first finished worker wins" accumulation.

3. Canonical output ordering

- Persisted tables and JSON structures must use canonical sort order for IDs/keys.

4. Determinism checks

- For engine phases, add tests that compare output hashes for `1 worker` vs `N workers`.
- Treat mismatches as correctness regressions unless justified and documented.
