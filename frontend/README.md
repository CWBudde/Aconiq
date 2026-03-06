# Aconiq Frontend

React/TypeScript UI for the Aconiq environmental noise modeling system.

## Stack

- React 19 + TypeScript (strict)
- Vite (dev server + bundler)
- Bun (package manager + test runner)
- ESLint 9 (flat config, strict type-checked)
- Prettier via treefmt

## Getting started

```bash
# Install dependencies
just fe-install

# Start dev server (proxies /api to localhost:8080)
just fe-dev

# Run all checks
just fe-ci
```

## Source layout

```
src/
  api/       Backend API client and types
  ui/        Shared UI primitives (shadcn/ui, Phase 23b)
  map/       Map adapters and layer helpers (MapLibre, Phase 23d)
  App.tsx    Root component
  main.tsx   Entry point
```

## Environment

Copy `.env.example` to `.env` and adjust as needed. All environment variables use the `VITE_` prefix.

## Commands

All commands are available via `just` from the repo root:

| Command             | What it does             |
| ------------------- | ------------------------ |
| `just fe-install`   | Install dependencies     |
| `just fe-dev`       | Start Vite dev server    |
| `just fe-build`     | Production build         |
| `just fe-typecheck` | TypeScript type checking |
| `just fe-lint`      | Run ESLint               |
| `just fe-test`      | Run Vitest tests         |
| `just fe-ci`        | All checks (CI)          |

See `../PLAN.md` Phase 23a-23h for the full frontend roadmap.
