# Contributing to Aconiq

Thank you for your interest in contributing to Aconiq! This document explains how to get involved.

## Getting started

1. Fork and clone the repository.
2. Install prerequisites: Go 1.24+, [just](https://github.com/casey/just), and the formatters listed in `treefmt.toml`.
3. Run `just ci` to verify everything builds, passes tests, and is formatted.

## Development workflow

```bash
just fmt          # Format all files
just lint         # Run golangci-lint
just test         # Run all Go tests
just build        # Build CLI → bin/noise
just ci           # Run all checks (format, test, lint, tidy)
```

See the `justfile` for the full list of recipes.

## Submitting changes

1. Create a feature branch from `main`.
2. Make your changes in small, focused commits.
3. Ensure `just ci` passes locally before pushing.
4. Open a pull request against `main` with a clear description of what and why.

## Code style

- Go code is formatted with `gofumpt` + `gci` (enforced via `just fmt`).
- All linters in `.golangci.yml` must pass.
- Follow existing patterns in the codebase. When in doubt, look at neighboring files.

## Testing

- Add tests for new functionality. Run `just test` to verify.
- Golden tests live in `testdata/` next to the owning test package. Update intentionally via `just update-golden` and review diffs.
- Determinism matters: same inputs must produce identical outputs regardless of parallelism. See `docs/policies/determinism.md`.

## Standards modules

If you are contributing to a normative standards module (under `backend/internal/standards/`):

- Normative outputs must only come from normative modules.
- Do not embed restricted normative text or tables verbatim. Use external data packs where redistribution rights are unclear.
- Document the compliance boundary for any changes to emission, propagation, or indicator logic.

## Reporting issues

Use [GitHub Issues](https://github.com/aconiq/backend/issues) to report bugs, request features, or ask questions about conformance.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
