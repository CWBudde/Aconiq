# Aconiq

Monorepo for an environmental noise modeling system (CLI-first, offline-first) with:

- Go backend (`backend/`)
- React/TypeScript frontend (`frontend/`), deferred until API/GUI phases
- Planning and governance docs (`docs/`, `PLAN.md`, `goal.md`)

## Repository Layout

- `backend/`: Go application, domain logic, standards modules, QA harness
- `frontend/`: browser UI workspace (deferred implementation)
- `docs/preflight/`: Phase 0 decisions (compliance, platforms, DoD, risks, constraints)
- `scripts/`: legacy CI scripts (superseded by `justfile`)
- `examples/`: license-safe sample projects and fixtures

## Development

Prerequisites: [Go](https://go.dev/), [just](https://github.com/casey/just), [golangci-lint](https://golangci-lint.run/), [treefmt](https://github.com/numtide/treefmt)

```bash
just          # list all available recipes
just ci       # run full check suite (format, test, lint, tidy)
just fmt      # format all files
just lint     # run linter
just test     # run tests
```

## Current Status

Phase 0 preflight through Phase 11 are completed, and Phase 20 (offline reporting v1, HTML MVP) is implemented.
