# Compute Engine Skeleton v1 (Phase 7)

Status date: 2026-03-06

Implemented in `backend/internal/engine`.

## Pipeline

Execution stages are explicit and emitted as structured progress events:

1. `load`
2. `prepare`
3. `chunk`
4. `compute`
5. `reduce`
6. `persist`

## Receiver Chunking

- Deterministic chunking by receiver input order.
- Configurable chunk size.

## Worker Pool

- Configurable worker count.
- Chunk jobs processed concurrently with context cancellation support.

## Deterministic Reduction

- Chunk results are reduced in sorted chunk-index order.
- Output hash is generated from canonical JSON encoding of reduced results.

## Cancellation and Cleanup

- `context.Canceled` propagates as the run error.
- Run state is persisted as `canceled`.
- Temporary chunk files (`*.tmp`) are cleaned up.

## Disk-Backed Cache v1

- Per-run directory: `<cacheDir>/<runID>/`
- Per-chunk cached results: `chunks/chunk-<index>.json`
- Run state and output files are persisted in the run directory.

## Tests

- Determinism test (1 worker vs N workers same output hash)
- Cancellation consistency test (state and cleanup)
- Cache reuse test
- Progress stage coverage test
