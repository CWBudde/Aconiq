# Formatting Policy

Status date: 2026-03-06

## Go Source Formatting

- `gofumpt` (strict superset of `gofmt`) is mandatory for all `.go` files.
- `gci` organizes imports.
- No merge is acceptable with unformatted Go code.

## Enforcement

- CI executes `just check-formatted` (treefmt with `--fail-on-change`).
- Linting: `just lint` runs golangci-lint v2 with all linters enabled.

## Local Workflow

- Before committing:
  - `just fmt` — formats Go, shell, markdown, YAML, JSON
  - `just test`
  - `just lint`
- Or run all checks at once: `just ci`
