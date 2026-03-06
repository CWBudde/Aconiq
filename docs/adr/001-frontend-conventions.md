# ADR-001: Frontend Conventions

Status: Accepted
Date: 2026-03-06

## Context

Phase 23a introduces the frontend workspace. We need to establish conventions early so the codebase stays consistent as it grows.

## Decisions

### Stack

- **Runtime/bundler:** Bun + Vite
- **Framework:** React 19 + TypeScript (strict mode)
- **UI library:** shadcn/ui (Phase 23b)
- **Map:** MapLibre GL JS (Phase 23d)

### Project structure

Single Vite app in `frontend/` (no monorepo workspaces). Source layout:

```
frontend/
  src/
    api/       # Backend API client and types
    ui/        # Shared UI primitives and theme (shadcn/ui wrappers)
    map/       # Map adapters and layer helpers
    App.tsx    # Root component
    main.tsx   # Entry point
```

### TypeScript

- Strict mode with `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`
- Path alias `@/*` maps to `src/*`

### Linting and formatting

- ESLint 9 flat config with `typescript-eslint` strict type-checked rules
- Prettier via project-wide treefmt (same as backend formatting flow)
- `just fe-lint` / `just fmt`

### Environment variables

- Vite `VITE_` prefix convention
- `.env.example` checked in, `.env` gitignored
- `src/env.d.ts` provides type safety for `import.meta.env`

### Testing

- Vitest for unit/component tests
- Playwright for E2E (Phase 23h)

### CI

- Separate `frontend-ci.yml` workflow, triggered only on `frontend/` changes
- Steps: install, typecheck, lint, test, build

### API client

- Hand-written typed client in `src/api/client.ts` (matches backend DTOs)
- Will move to generated client when OpenAPI spec is available (Phase 23 API contract)

## Consequences

- Simple flat structure avoids workspace overhead; can be introduced later if needed
- Strict TypeScript catches issues early but requires explicit typing at boundaries
- treefmt handles formatting for both Go and frontend files consistently
