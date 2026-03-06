# Formatting Policy

Status date: 2026-03-06

## Go Source Formatting

- `gofmt` is mandatory for all `.go` files.
- `gofumpt` is optional and not enforced yet.
- No merge is acceptable with unformatted Go code.

## Enforcement

- CI executes `scripts/check-gofmt.sh`.
- CI fails if any file appears in `gofmt -l` output.

## Local Workflow

- Before committing:
  - `gofmt -w $(find backend -type f -name '*.go')`
  - `scripts/test-go.sh`
