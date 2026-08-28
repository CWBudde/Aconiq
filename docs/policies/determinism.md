# Determinism Policy

Status date: 2026-08-28

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

- A reduction is **numerically sensitive** when either of these holds:
  - its term count scales with model size (integration over line-source subsegments, over
    polyline vertices, over polygon ring edges, over segments × operations), or
  - its terms alternate in sign, so terms can cancel (the shoelace area sum).
- Sensitive reductions must accumulate through `internal/numeric.CompensatedSum` (Neumaier
  compensated summation). Fixed-length reductions — the eight octave bands of a spectrum, the
  vehicle classes of a train, the entries of a correction table — are exempt: their error is
  bounded by a term count the model cannot grow.
- The chosen strategy must be consistent across worker counts. `CompensatedSum` depends only on
  the order terms are added in, so it is as deterministic as the loop feeding it; the fixed
  partition order required below is what makes that order stable.

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
